# Pre-flight reads max_allowed_packet and fails fast; it never SET GLOBALs it

The PRD (§5.4) proposed raising the server's `max_allowed_packet` at startup so
huge extended-INSERT statements (a single statement can be hundreds of MB) are
not rejected. We are **not** issuing `SET GLOBAL max_allowed_packet`. Instead the
**Pre-flight Check** reads `@@global.max_allowed_packet` and, if it is below a
safe threshold, aborts with an actionable message. The client-side limit is set
explicitly on the driver DSN (`maxAllowedPacket`, up to the 1GB server maximum),
which needs no privilege.

Verified against MariaDB docs (context7): a `SET GLOBAL` change *"affects all new
sessions but not currently open ones"* — so it would not even apply to the
running restore connection without a reconnect — and it *"requires administrative
privileges"* (`SUPER`), which a restore user may not hold. The server maximum is
1GB (`0x40000000`).

## Considered Options

- **`SET GLOBAL max_allowed_packet` at startup (PRD §5.4):** rejected — (a)
  requires `SUPER`, failing confusingly when absent; (b) does not affect the
  current connection, forcing a reconnect dance; (c) mutates shared server state,
  a surprising side effect for a restore tool.
- **`SET SESSION max_allowed_packet`:** rejected — on the client→server path this
  variable is effectively read-only for enlarging beyond the global for receiving
  packets; the meaningful limit for what the server accepts is global. Reading and
  gating is honest and privilege-free.

## Consequences

- Requires the operator to have provisioned `max_allowed_packet` on the server
  beforehand; the tool guides them but does not do it for them.
- The driver DSN `maxAllowedPacket` default must be verified at implementation
  time (go-sql-driver/mysql is not indexed in context7); we set it explicitly to
  avoid relying on the default.
- The safe threshold is a tunable constant; default proposed at 256MB, overridable.
