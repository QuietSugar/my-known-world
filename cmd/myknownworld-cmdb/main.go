// myknownworld-cmdb runs the replaceable upper-layer environment catalog API.
// Secret lifecycle routes stay unavailable unless an operator explicitly selects
// the built-in file provider or supplies their own provider endpoint.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/QuietSugar/my-known-world/internal/cmdb"
)

func main() {
	rekey := flag.Bool("rekey-vault", false, "re-encrypt the local file vault with CMDB_VAULT_NEW_KEY_BASE64 or CMDB_VAULT_NEW_KEY_FILE, then exit")
	flag.Parse()
	if *rekey {
		if err := rekeyVault(); err != nil {
			log.Fatal(err)
		}
		log.Printf("vault rekey completed; restart with the new key")
		return
	}
	dsn := os.Getenv("CMDB_DATABASE_URL")
	if dsn == "" {
		log.Fatal("CMDB_DATABASE_URL is required")
	}
	token := os.Getenv("CMDB_APP_TOKEN")
	if len(token) < 32 {
		log.Fatal("CMDB_APP_TOKEN must contain at least 32 characters")
	}
	if coreToken := os.Getenv("MYKNOWNWORLD_WRITE_TOKEN"); coreToken != "" && token == coreToken {
		log.Fatal("CMDB_APP_TOKEN must be independent from MYKNOWNWORLD_WRITE_TOKEN")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo := cmdb.PostgresEnvironmentRepository{DB: db}
	if err := repo.EnsureSchema(ctx); err != nil {
		log.Fatalf("ensure application schema: %v", err)
	}
	addr := os.Getenv("CMDB_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8082"
	}
	provider, err := cmdb.NewSecretProvider(providerOptionsFromEnvironment())
	if err != nil {
		log.Fatal(err)
	}
	var secrets cmdb.SecretService
	if provider != nil {
		secrets = cmdb.SecretService{Provider: provider, Authorizer: cmdb.RequestSecretAuthorizer{}}
	}
	handler := cmdb.HTTPHandler{Secrets: secrets, Catalog: cmdb.Catalog{Repository: repo}, Resources: cmdb.ResourceCatalog{Repository: repo}, Authorizer: cmdb.BearerAuthorizer{Token: token}}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-signals.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}
}

// providerOptionsFromEnvironment maps the documented provider variables onto
// the provider contract. Secrets are supplied by the operator; no provider runs
// by default and no secret material is read from the application database.
func providerOptionsFromEnvironment() cmdb.ProviderOptions {
	opts := cmdb.ProviderOptions{
		Kind:      cmdb.ProviderKind(strings.TrimSpace(os.Getenv("CMDB_SECRET_PROVIDER"))),
		Directory: strings.TrimSpace(os.Getenv("CMDB_VAULT_DIR")),
	}
	if opts.Kind != cmdb.ProviderFile {
		opts.URL = strings.TrimSpace(os.Getenv("CMDB_SECRET_PROVIDER_URL"))
		opts.Token = os.Getenv("CMDB_SECRET_PROVIDER_TOKEN")
		opts.AllowPlaintextHTTP = os.Getenv("CMDB_SECRET_PROVIDER_ALLOW_HTTP") == "1"
		return opts
	}
	key, keyFile, err := cmdb.VaultKeyFromEnvironment(os.Getenv("CMDB_VAULT_KEY_BASE64"), os.Getenv("CMDB_VAULT_KEY_FILE"))
	if err != nil {
		log.Fatalf("vault key configuration: %v", err)
	}
	opts.Key = key
	opts.KeyFile = keyFile
	return opts
}

// rekeyVault rotates the file-provider master key offline. The store is only
// rewritten when every record decrypts with the current key.
func rekeyVault() error {
	if kind := strings.TrimSpace(os.Getenv("CMDB_SECRET_PROVIDER")); kind != string(cmdb.ProviderFile) {
		return errors.New("CMDB_SECRET_PROVIDER must be file for --rekey-vault")
	}
	dir := strings.TrimSpace(os.Getenv("CMDB_VAULT_DIR"))
	if dir == "" {
		return errors.New("CMDB_VAULT_DIR is required for --rekey-vault")
	}
	oldKey, oldFile, err := cmdb.VaultKeyFromEnvironment(os.Getenv("CMDB_VAULT_KEY_BASE64"), os.Getenv("CMDB_VAULT_KEY_FILE"))
	if err != nil {
		return err
	}
	if len(oldKey) == 0 {
		if oldKey, err = cmdb.LoadVaultKeyFile(oldFile); err != nil {
			return err
		}
	}
	newKey, newFile, err := cmdb.VaultKeyFromEnvironment(os.Getenv("CMDB_VAULT_NEW_KEY_BASE64"), os.Getenv("CMDB_VAULT_NEW_KEY_FILE"))
	if err != nil {
		return err
	}
	if len(newKey) == 0 {
		if newFile == "" {
			return errors.New("set CMDB_VAULT_NEW_KEY_BASE64 or CMDB_VAULT_NEW_KEY_FILE for --rekey-vault")
		}
		if newKey, err = cmdb.LoadVaultKeyFile(newFile); err != nil {
			return err
		}
	}
	vault := &cmdb.FileVault{Dir: dir, Key: oldKey}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return vault.Rekey(ctx, newKey)
}
