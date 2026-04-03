package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
)

const signingKeyFilePerms = 0o600

// WithSigningKey fetches the signing key from Vault, stores it into
// a temp file and takes care of removal as soon as the function ended.
// The error returned is either from key fetching or from the function
// execution.
func WithSigningKey(keyPath string, fn func(keyFilePath string) error) (err error) {
	client, err := vaultapi.NewClient(vaultapi.DefaultConfig())
	if err != nil {
		return fmt.Errorf("creating Vault client: %w", err)
	}

	if token := strings.TrimSpace(os.Getenv("VAULT_TOKEN")); token != "" {
		client.SetToken(token)
	} else if homeDir, err := os.UserHomeDir(); err == nil {
		tokenFile := filepath.Join(homeDir, ".vault-token")
		if token, err := os.ReadFile(tokenFile); err == nil { //#nosec:G304 // Intended to read vault token from disk
			client.SetToken(strings.TrimSpace(string(token)))
		}
	}

	secret, err := client.Logical().Read(keyPath)
	if err != nil {
		return fmt.Errorf("reading secret %q from Vault: %w", keyPath, err)
	}
	if secret == nil {
		return fmt.Errorf("reading secret %q from Vault: empty response", keyPath)
	}

	keyData, err := extractSigningKey(secret.Data)
	if err != nil {
		return fmt.Errorf("extracting signing key from %q: %w", keyPath, err)
	}

	f, err := os.CreateTemp("", "archrepo-signing-key-")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	keyFilePath := f.Name()
	defer func() {
		if rmErr := os.Remove(keyFilePath); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("removing temp file %q: %w", keyFilePath, rmErr))
		}
	}()

	if err = f.Chmod(signingKeyFilePerms); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing temp file %q after chmod failure: %w", keyFilePath, closeErr))
		}
		return fmt.Errorf("setting temp file permissions: %w", err)
	}

	if _, err = f.WriteString(keyData); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing temp file %q after write failure: %w", keyFilePath, closeErr))
		}
		return fmt.Errorf("writing signing key: %w", err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	return fn(keyFilePath)
}

func extractSigningKey(data map[string]any) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("secret data is empty")
	}

	if nested, ok := data["data"]; ok {
		nestedMap, ok := nested.(map[string]any)
		if !ok {
			return "", fmt.Errorf("nested secret data has unexpected type %T", nested)
		}
		return extractSigningKey(nestedMap)
	}

	value, ok := data["key"]
	if !ok {
		return "", fmt.Errorf("secret field %q not found", "key")
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("field %q has unexpected type %T", "key", value)
	}

	return str, nil
}
