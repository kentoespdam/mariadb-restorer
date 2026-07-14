package credentialvault

import (
	"fmt"
	"os"
)

// CredentialSource indicates where the password was resolved from.
type CredentialSource int

const (
	SourceNone         CredentialSource = iota // no source resolved
	SourcePasswordFile                         // --password-file flag
	SourcePasswordFlag                         // --password / -p flag
	SourceVault                                // sealed_password in profile
	SourceEnvVar                               // MYSQL_PWD
	SourcePrompt                               // TTY prompt
)

func (s CredentialSource) String() string {
	switch s {
	case SourceNone:
		return "none"
	case SourcePasswordFile:
		return "--password-file"
	case SourcePasswordFlag:
		return "--password"
	case SourceVault:
		return "vault (sealed)"
	case SourceEnvVar:
		return "MYSQL_PWD"
	case SourcePrompt:
		return "TTY prompt"
	default:
		return "unknown"
	}
}

// ResolvedCredential holds the resolved password and its source.
type ResolvedCredential struct {
	Password string
	Source   CredentialSource
}

// ResolveCredential resolves the password from the available sources
// following the precedence chain:
//
//	--password-file > --password > vault > MYSQL_PWD > TTY prompt
//
// Parameters with empty string are treated as unavailable.
func ResolveCredential(passwordFile, passwordFlag string, vaultPassword string) (ResolvedCredential, error) {
	// 1. --password-file
	if passwordFile != "" {
		data, err := os.ReadFile(passwordFile)
		if err != nil {
			return ResolvedCredential{}, fmt.Errorf("read password file: %w", err)
		}
		// Trim trailing whitespace (usually newline).
		pwd := string(data)
		for len(pwd) > 0 && (pwd[len(pwd)-1] == '\n' || pwd[len(pwd)-1] == '\r' || pwd[len(pwd)-1] == ' ') {
			pwd = pwd[:len(pwd)-1]
		}
		return ResolvedCredential{Password: pwd, Source: SourcePasswordFile}, nil
	}

	// 2. --password flag
	if passwordFlag != "" {
		return ResolvedCredential{Password: passwordFlag, Source: SourcePasswordFlag}, nil
	}

	// 3. Vault (sealed password). This is already decrypted before calling.
	if vaultPassword != "" {
		return ResolvedCredential{Password: vaultPassword, Source: SourceVault}, nil
	}

	// 4. MYSQL_PWD env var
	if envPwd := os.Getenv("MYSQL_PWD"); envPwd != "" {
		return ResolvedCredential{Password: envPwd, Source: SourceEnvVar}, nil
	}

	// 5. No source found — caller may prompt interactively.
	return ResolvedCredential{Source: SourceNone}, nil
}
