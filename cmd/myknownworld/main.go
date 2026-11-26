package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/QuietSugar/my-known-world/devseed"
	"github.com/QuietSugar/my-known-world/internal/api"
	"github.com/QuietSugar/my-known-world/internal/bitwarden"
	"github.com/QuietSugar/my-known-world/internal/cmdb"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/QuietSugar/my-known-world/internal/mcpapi"
	pgstore "github.com/QuietSugar/my-known-world/internal/store/postgres"
	"github.com/QuietSugar/my-known-world/internal/version"
	"github.com/QuietSugar/my-known-world/internal/webui"
)

func main() {
	g, seedErr := devseed.Graph()
	if seedErr != nil {
		log.Fatalf("load development seed: %v", seedErr)
	}
	var syncPreviewer api.SyncPreviewer
	var graphStore *pgstore.Store
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		defaultDSN := "postgresql://myknownworld:vWipKJWDKVao9mESAy4ERMpgJCHkAi97eIoEKeXbH9o2SOZWsPTsSFKlWe9eMINC@172.19.0.3:5432/myknownworld?sslmode=disable"
		ctxCheck, cancelCheck := context.WithTimeout(context.Background(), 1*time.Second)
		if testStore, err := pgstore.Open(ctxCheck, defaultDSN); err == nil {
			_ = testStore.Close()
			dsn = defaultDSN
		}
		cancelCheck()
	}
	if dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		store, err := pgstore.Open(ctx, dsn)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		graphStore = store
		syncPreviewer = store
		if err = store.Migrate(ctx); err != nil {
			log.Fatal(err)
		}
		g, err = store.LoadSnapshot(ctx)
		if errors.Is(err, pgstore.ErrEmptySnapshot) && os.Getenv("MYKNOWNWORLD_SEED_DEMO") == "1" {
			g, err = devseed.Graph()
			if err == nil {
				err = store.SaveSnapshot(ctx, g)
			}
		}
		if err != nil {
			log.Fatalf("load graph snapshot: %v (set MYKNOWNWORLD_SEED_DEMO=1 only for an intentionally synthetic database)", err)
		}
		log.Print("MyKnownWorld serving the PostgreSQL graph snapshot")
	} else {
		log.Print("MyKnownWorld serving synthetic in-memory demo data")
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	mux := http.NewServeMux()
	var repository api.Repository
	if configured, ok := syncPreviewer.(api.Repository); ok {
		repository = configured
	}
	bwCfg := bitwarden.AutoDetectConfig()
	bwUpstream := cmdb.NewBitwardenUpstream(bwCfg)
	fileUpstream, _ := cmdb.NewFileUpstreamVault("", nil)

	var pgDB *sql.DB
	if graphStore != nil {
		pgDB = graphStore.DB()
	}
	secretProvider := cmdb.NewCompositeSecretProvider(pgDB, bwUpstream, fileUpstream)
	if pgDB != nil {
		_ = secretProvider.Local.EnsureSchema(context.Background())
		log.Print("PostgreSQL local encrypted secret engine ready")
	}

	var templateStore api.TemplateStore
	if graphStore != nil {
		templateStore = graphStore
	}

	apiHandler := &api.Handler{
		Graph:          g,
		SyncPreviewer:  syncPreviewer,
		Repository:     repository,
		WriteToken:     os.Getenv("MYKNOWNWORLD_WRITE_TOKEN"),
		SecretProvider: secretProvider,
		Cache:          api.NewSnapshotCache(5 * time.Second),
		TemplateStore:  templateStore,
	}
	mux.Handle("/api/", apiHandler)
	mux.Handle("/healthz", apiHandler)
	mux.Handle("/metrics", apiHandler)
	var mcpHistory mcpapi.HistoryReader
	var mcpAssertions mcpapi.AssertionReader
	var mcpWriter mcpapi.GraphWriter
	// Usage logging is opt-in: without MCP_USAGE_LOG_FILE the server records nothing.
	var usageLog *mcpapi.UsageLog
	if path := strings.TrimSpace(os.Getenv("MCP_USAGE_LOG_FILE")); path != "" {
		opened, openErr := mcpapi.OpenUsageLog(path)
		if openErr != nil {
			log.Printf("usage log disabled: %v", openErr)
		} else {
			usageLog = opened
			defer usageLog.Close()
			log.Printf("MCP usage log: %s", path)
		}
	}
	writeToken := os.Getenv("MYKNOWNWORLD_WRITE_TOKEN")
	if graphStore != nil {
		mcpHistory, mcpAssertions = graphStore, graphStore
		if len(writeToken) >= 32 {
			mcpWriter = graphStore
		}
	}
	mcpHandler, err := mcpapi.NewHandlerWithWriterAndUsage(version.Current(), func(ctx context.Context) (*graph.Graph, error) {
		if graphStore != nil {
			return graphStore.LoadSnapshot(ctx)
		}
		return g.Clone(), nil
	}, mcpHistory, mcpWriter, usageLog, mcpAssertions)
	if err != nil {
		log.Fatalf("configure MCP: %v", err)
	}
	mux.Handle("/mcp", mcpHandler)
	if target := os.Getenv("CMDB_PROXY_TARGET"); target != "" {
		upstream, err := url.Parse(target)
		if err != nil || upstream.Scheme == "" || upstream.Host == "" {
			log.Fatal("CMDB_PROXY_TARGET must be an absolute HTTP URL")
		}
		proxy := httputil.NewSingleHostReverseProxy(upstream)
		original := proxy.Director
		proxy.Director = func(req *http.Request) {
			original(req)
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/cmdb")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
		mux.Handle("/cmdb/", proxy)
		mux.Handle("/cmdb", proxy)
	}
	if root := os.Getenv("WEB_ROOT"); root != "" {
		mux.Handle("/", webui.Handler{Root: root})
	} else {
		mux.Handle("/", apiHandler)
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("MyKnownWorld HTTP service listening on %s", addr)
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}
}
