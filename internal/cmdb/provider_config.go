package cmdb

import (
	"errors"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/bitwarden"
)

// ProviderKind selects which secret provider an operator has configured.
// An empty kind means credential operations stay disabled.
type ProviderKind string

const (
	ProviderDisabled  ProviderKind = ""
	ProviderFile      ProviderKind = "file"
	ProviderHTTP      ProviderKind = "http"
	ProviderBitwarden ProviderKind = "bitwarden"
)

// ProviderOptions is the deployment-supplied provider configuration. Key
// material is read from Key or KeyFile, never from the store directory.
type ProviderOptions struct {
	Kind ProviderKind
	// File provider settings.
	Directory string
	Key       []byte
	KeyFile   string
	// Operator-supplied secret service settings.
	URL               string
	Token             string
	AllowInsecureHTTP bool
	// Bitwarden / Vaultwarden settings.
	BitwardenServerURL      string
	BitwardenClientID       string
	BitwardenClientSecret   string
	BitwardenMasterPassword string
}

// NewSecretProvider builds the configured provider. It returns (nil, nil) when
// no provider is selected, so credential routes report unavailable instead of
// silently running without authorization.
func NewSecretProvider(opts ProviderOptions) (SecretProvider, error) {
	switch opts.Kind {
	case ProviderDisabled:
		return nil, nil
	case ProviderFile:
		if strings.TrimSpace(opts.Directory) == "" {
			return nil, errors.New("file provider requires CMDB_VAULT_DIR")
		}
		key := opts.Key
		if len(key) == 0 {
			if strings.TrimSpace(opts.KeyFile) == "" {
				return nil, errors.New("file provider requires CMDB_VAULT_KEY_BASE64 or CMDB_VAULT_KEY_FILE")
			}
			loaded, err := LoadVaultKeyFile(opts.KeyFile)
			if err != nil {
				return nil, err
			}
			key = loaded
		}
		if len(key) != 32 {
			return nil, errors.New("file provider key must contain exactly 32 bytes")
		}
		return &FileVault{Dir: opts.Directory, Key: append([]byte(nil), key...)}, nil
	case ProviderHTTP:
		provider := &HTTPSecretProvider{
			BaseURL:           strings.TrimSpace(opts.URL),
			Token:             opts.Token,
			AllowInsecureHTTP: opts.AllowInsecureHTTP,
		}
		// Validate configuration eagerly so a bad endpoint fails at startup
		// rather than on the first credential request.
		if _, err := provider.endpoint("/secrets"); err != nil {
			return nil, err
		}
		return provider, nil
	case ProviderBitwarden:
		cfg := bitwarden.Config{
			ServerURL:      opts.BitwardenServerURL,
			ClientID:       opts.BitwardenClientID,
			ClientSecret:   opts.BitwardenClientSecret,
			MasterPassword: opts.BitwardenMasterPassword,
		}
		if !cfg.Enabled() {
			cfg = bitwarden.AutoDetectConfig()
		}
		provider := NewBitwardenProvider(cfg)
		return provider, nil
	default:
		return nil, errors.New("unsupported CMDB_SECRET_PROVIDER value; supported values are file, http, and bitwarden")
	}
}

// VaultKeyFromEnvironment reads the file-provider key from either the Base64
// variable or a dedicated key file. Both are rejected when set together so an
// operator cannot silently run with an unexpected key.
func VaultKeyFromEnvironment(base64Value, keyFile string) ([]byte, string, error) {
	base64Value = strings.TrimSpace(base64Value)
	keyFile = strings.TrimSpace(keyFile)
	if base64Value != "" && keyFile != "" {
		return nil, "", errors.New("set either CMDB_VAULT_KEY_BASE64 or CMDB_VAULT_KEY_FILE, not both")
	}
	if base64Value != "" {
		key, err := ParseVaultKey([]byte(base64Value))
		if err != nil {
			return nil, "", err
		}
		return key, "", nil
	}
	if keyFile != "" {
		return nil, keyFile, nil
	}
	return nil, "", nil
}
