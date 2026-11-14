// myknownworld-mcp starts a read-only MCP server over stdio for local agents.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QuietSugar/my-known-world/internal/mcpapi"
	pgstore "github.com/QuietSugar/my-known-world/internal/store/postgres"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func run(ctx context.Context) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required; MCP stdio refuses to seed or mutate a graph")
	}
	startup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	store, err := pgstore.Open(startup, dsn)
	if err != nil {
		return err
	}
	defer store.Close()
	if _, err = store.LoadSnapshot(startup); err != nil {
		return err
	}
	server, err := mcpapi.NewServer(store.LoadSnapshot, store, store)
	if err != nil {
		return err
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
