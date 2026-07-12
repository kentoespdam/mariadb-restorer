# The lexer scans raw bytes, not decoded runes — restorable dumps are ASCII-transparent by construction

ADR-0023 pinned *that* statement splitting is a state-aware streaming lexer. It left
open *what the lexer scans*: raw `[]byte` off the dump stream, or Unicode runes decoded
from some detected file encoding. This is not a style choice — it decides whether the
tool inherits an entire class of multibyte-boundary bugs, and whether a chunk read can
ever cut a character in half and corrupt a boundary. This ADR pins that the lexer scans
**raw bytes**, and records the MariaDB fact that makes that not merely convenient but
*correct for every dump that can be restored at all*.

## The two candidates

- **Rune-scan:** detect the file's encoding, decode each chunk to runes (Go `rune` /
  UTF-8 code points), and run the state machine over runes. This is what HeidiSQL does
  (`Encoding.Convert` in `ReadTextfileChunk`), and it forces HeidiSQL's whole
  `EEncodingError`-retry dance: a fixed-size chunk read can land in the middle of a
  multibyte character, decoding throws, and HeidiSQL rewinds and grows the chunk by one
  byte, up to ten times, until the cut falls on a character boundary.
- **Byte-scan:** the lexer never decodes. It scans the chunk as `[]byte`, and every
  structural token it must recognise — `'` `"` `` ` `` `\` `;` `/` `*` `-` `#`,
  newlines, and any custom `DELIMITER` bytes — is ASCII (< 0x80). The dump's own bytes
  are forwarded verbatim to the server, which decodes them per the dump's `SET NAMES`.

## Why byte-scan is safe: an ASCII byte is never a UTF-8 continuation byte

In UTF-8 every byte of a multibyte character is ≥ 0x80: lead bytes are 0xC0–0xFF,
continuation bytes are 0x80–0xBF. **No byte of a multibyte character is ever < 0x80.**
So an ASCII byte like `0x3B` (`;`) or `0x27` (`'`) appearing in the stream is *always* a
real, standalone ASCII character — it can never be the tail fragment of some multibyte
character that happened to encode a byte with that value. Scanning for ASCII delimiters
byte-by-byte is therefore immune to the multibyte-cut-at-a-chunk-boundary problem *by
construction*: the delimiter bytes cannot hide inside multibyte characters, and a chunk
boundary that splits a multibyte character splits it only between bytes that are all
≥ 0x80 — none of which the lexer is looking for. HeidiSQL's entire `EEncodingError`
rewind-and-retry loop exists only because it decodes; a byte-scanner has nothing to
rewind.

This eliminates a whole bug class rather than handling it: there is no "character cut in
half" state for the lexer to be wrong about, because the lexer never assembles
characters.

## The honest hole — and why MariaDB closes it for us

The reasoning above holds for **ASCII-transparent** encodings (ASCII, UTF-8/utf8mb4,
Latin-1, and every other encoding where the byte values < 0x80 mean exactly their ASCII
characters and never appear as a fragment of a wider character). It **breaks** for
ASCII-incompatible encodings — `utf16`, `utf16le`, `ucs2`, `utf32` — where `0x3B` can
legitimately be one byte of a two- or four-byte character, so a byte-scanner would see a
phantom `;` inside string data and place a boundary inside a literal. That is exactly the
poisoned-resume-offset failure ADR-0023 exists to prevent.

The question is therefore not "is byte-scan usually safe" but "can a *restorable* dump
ever be in an ASCII-incompatible encoding". MariaDB answers no, at the server:

> "Character sets like `ucs2`, `utf16`, `utf16le`, and `utf32` are **not valid for
> `SET NAMES`** as they cannot be used as client character sets."
> — context7 `/mariadb-corporation/mariadb-docs`, *SET NAMES* and *character_set_client*

Chain the facts:

1. `mariadb-dump` emits `SET NAMES <charset>` near the top of every dump, because
   `--set-charset` is **on by default** (context7 `/mariadb-corporation/mariadb-docs`;
   suppressed only with `--skip-set-charset`). The default charset is **utf8mb4**
   (11.8+) / utf8mb3 (pre-11.8) — both ASCII-transparent.
2. A restore *is* a client sending those statements over a connection. The very first
   thing the dump tells the server is `SET NAMES <charset>`.
3. The server **rejects** `SET NAMES utf16` / `ucs2` / `utf16le` / `utf32` — they are
   not permissible client character sets. So a dump whose SQL text is in one of those
   encodings **cannot be restored by any client**, ours included: its own `SET NAMES`
   fails before a single `INSERT` runs.

So an ASCII-incompatible dump is not a case we choose to drop — it is **outside the set
of inputs that are restorable at all**. Every dump that *can* be restored declares a
client-legal charset, and every client-legal charset is ASCII-transparent. Byte-scan is
therefore correct across the entire domain of valid input, not merely across the common
case. The "hole" is closed by the server's own constraint, not by an assumption we make.

## Decision

- The **Statement Splitter** (ADR-0023) scans the dump as **raw `[]byte`**. It never
  decodes bytes to runes and never detects or converts a file encoding.
- All structural tokens the lexer recognises are ASCII (< 0x80); because no byte of a
  UTF-8 multibyte character is < 0x80, byte-scanning for these tokens is immune to
  multibyte truncation at chunk boundaries. There is no `EEncodingError`-equivalent
  retry path, because there is no decode step.
- Statement bytes are forwarded to the server **verbatim**; the server decodes them per
  the dump's own `SET NAMES`. The tool is charset-agnostic about *content* — it only
  ever reasons about ASCII structure.
- The safety of byte-scanning rests on ASCII-transparency, which is **guaranteed for
  every restorable dump** because MariaDB forbids `utf16`/`utf16le`/`ucs2`/`utf32` as
  client character sets (context7): a dump in such an encoding cannot be restored by any
  client, so it is out of the valid-input set, not an unhandled case.
- The tool does **not** implement encoding detection, a BOM sniffer, or an
  ASCII-incompatible fallback lexer. Adding one would be handling input that is by
  definition non-restorable.

## Considered Options

- **Rune-scan with encoding detection (HeidiSQL's model):** rejected — it buys nothing
  a byte-scanner needs (all delimiters are ASCII) and imports HeidiSQL's whole
  chunk-boundary `EEncodingError` rewind-retry machinery to solve a problem byte-scanning
  does not have. It also spends CPU decoding 9GB of bytes only to forward them verbatim
  anyway.
- **Byte-scan but add a guard that refuses a dump whose `SET NAMES` names an
  ASCII-incompatible charset:** rejected as unnecessary — the *server* already refuses
  such a `SET NAMES`, so the dump fails on its own first statement with a clear server
  error. A pre-emptive guard would duplicate a check the server makes authoritatively and
  would risk diverging from the server's exact charset list over versions. (If real-world
  reports ever surface a dump that reaches the splitter in an ASCII-incompatible encoding
  *without* a rejecting `SET NAMES`, revisit — but no such dump is producible by
  `mariadb-dump`.)
- **Decode only to validate UTF-8, then scan bytes:** rejected — validation is a
  whole-file rune pass in disguise, costing exactly the CPU byte-scan avoids, to reject
  inputs the server already rejects.

## Consequences

- The lexer's alphabet is bytes, and its state set (ADR-0023: string/identifier/comment/
  executable-comment + current delimiter) is defined over byte values, not runes. A
  reviewer who introduces `utf8.DecodeRune` into the scan loop is reintroducing the
  decode step this ADR removes and must justify it against the ASCII-transparency
  guarantee.
- Chunk boundaries can fall anywhere, including mid-character, with **no special
  handling** — the lexer only ever acts on ASCII bytes, and a split multibyte character
  is just two runs of ≥ 0x80 bytes that pass through untouched into the forwarded
  statement text. This is why the **Byte Offset** can be any byte position on a
  **Statement Boundary** without an encoding-alignment caveat.
- The **Byte Offset** / resume story inherits this cleanly: a boundary is the byte after
  a `;` in the `default` state (CONTEXT.md), and because that `;` is provably a real
  ASCII `;` (not a multibyte fragment), the persisted offset is always a legal resume
  point. This is the byte-level underpinning ADR-0023's boundary-correctness claim
  assumed but did not state.
- The tool carries **no encoding configuration** — no `--encoding` flag, no charset
  autodetect. This is a deliberate non-feature: the dump's `SET NAMES` is the single
  source of charset truth, interpreted by the server, and the tool stays out of it.
