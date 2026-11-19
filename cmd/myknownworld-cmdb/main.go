// myknownworld-cmdb runs the replaceable upper-layer environment catalog API.
// Secret lifecycle routes remain unavailable until a provider is injected.
package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QuietSugar/my-known-world/internal/cmdb"
)

func main() {
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
	var secrets cmdb.SecretService
	switch os.Getenv("CMDB_SECRET_PROVIDER") {
	case "":
	case "file":
		key, err := base64.StdEncoding.DecodeString(os.Getenv("CMDB_VAULT_KEY_BASE64"))
		if err != nil || len(key) != 32 {
			log.Fatal("CMDB_VAULT_KEY_BASE64 must encode exactly 32 random bytes")
		}
		dir := os.Getenv("CMDB_VAULT_DIR")
		if dir == "" {
			log.Fatal("CMDB_VAULT_DIR is required for the file provider")
		}
		secrets = cmdb.SecretService{Provider: &cmdb.FileVault{Dir: dir, Key: key}, Authorizer: cmdb.RequestSecretAuthorizer{}}
	default:
		log.Fatal("unsupported CMDB_SECRET_PROVIDER; configure a provider implementation")
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
