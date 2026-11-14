package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QuietSugar/my-known-world/internal/api"
	"github.com/QuietSugar/my-known-world/internal/graph"
	pgstore "github.com/QuietSugar/my-known-world/internal/store/postgres"
	"github.com/QuietSugar/my-known-world/internal/webui"
)

func main() {
	g := demoGraph()
	var syncPreviewer api.SyncPreviewer
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		store, err := pgstore.Open(ctx, dsn)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		syncPreviewer = store
		if err = store.Migrate(ctx); err != nil {
			log.Fatal(err)
		}
		g, err = store.LoadSnapshot(ctx)
		if errors.Is(err, pgstore.ErrEmptySnapshot) && os.Getenv("MYKNOWNWORLD_SEED_DEMO") == "1" {
			g = demoGraph()
			err = store.SaveSnapshot(ctx, g)
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
		addr = ":8080"
	}
	mux := http.NewServeMux()
	var repository api.Repository
	if configured, ok := syncPreviewer.(api.Repository); ok {
		repository = configured
	}
	apiHandler := &api.Handler{Graph: g, SyncPreviewer: syncPreviewer, Repository: repository, WriteToken: os.Getenv("MYKNOWNWORLD_WRITE_TOKEN")}
	mux.Handle("/api/", apiHandler)
	mux.Handle("/healthz", apiHandler)
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

func demoGraph() *graph.Graph {
	g := graph.New()
	must(g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", Source: "synthetic demo", EvidenceKind: graph.EvidenceManual, Certainty: graph.CertaintyConfirmed, ModelVersion: 1}))
	must(g.Publish(graph.Node{ID: "demo-machine", Type: graph.Machine, Name: "Example machine", Lifecycle: "active", Source: "synthetic demo", EvidenceKind: graph.EvidenceManual, Certainty: graph.CertaintyConfirmed, ModelVersion: 1}, graph.Relation{ID: "demo-maintains-machine", Predicate: graph.Maintains, To: "demo-machine", Source: "synthetic demo"}))
	must(g.Publish(graph.Node{ID: "demo-project", Type: graph.SoftwareProject, Name: "Example project", Lifecycle: "maintained", Source: "synthetic demo", EvidenceKind: graph.EvidenceManual, Certainty: graph.CertaintyConfirmed, ModelVersion: 1}, graph.Relation{ID: "demo-knows-project", To: "demo-project", Source: "synthetic demo"}))
	return g
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
