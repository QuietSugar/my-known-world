// Package postgres persists complete graph snapshots in PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/QuietSugar/my-known-world/internal/graph"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrations embed.FS

const schemaVersion = 2

var ErrEmptySnapshot = errors.New("database contains no graph snapshot")

type Store struct{ db *sql.DB }

func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Migrate applies the embedded initial schema and records its explicit version.
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	files := []string{"migrations/001_init.sql", "migrations/002_provenance.sql"}
	if len(files) != schemaVersion {
		return fmt.Errorf("migration list has %d entries, schemaVersion is %d", len(files), schemaVersion)
	}
	for index, name := range files {
		migration, readErr := migrations.ReadFile(name)
		if readErr != nil {
			return readErr
		}
		version := index + 1
		if _, err = tx.ExecContext(ctx, string(migration)); err != nil {
			return fmt.Errorf("apply schema migration %d: %w", version, err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES($1) ON CONFLICT (version) DO NOTHING`, version); err != nil {
			return fmt.Errorf("record schema migration %d: %w", version, err)
		}
	}
	return tx.Commit()
}

// SaveSnapshot atomically replaces all graph rows. Callers should use this only
// for a complete authoritative snapshot, never for partial imports.
func (s *Store) SaveSnapshot(ctx context.Context, g *graph.Graph) error {
	if err := validateSnapshot(g); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes, relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock graph snapshot tables: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM relations`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM nodes`); err != nil {
		return err
	}
	for _, n := range g.Nodes() {
		if _, err = tx.ExecContext(ctx, `INSERT INTO nodes(id,type,name,lifecycle,last_confirmed,model_version,source,evidence_kind,certainty) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, n.ID, n.Type, n.Name, n.Lifecycle, n.LastConfirmed, n.ModelVersion, n.Source, n.EvidenceKind, n.Certainty); err != nil {
			return fmt.Errorf("save node %q: %w", n.ID, err)
		}
	}
	for _, r := range g.Relations() {
		if _, err = tx.ExecContext(ctx, `INSERT INTO relations(id,from_id,predicate,to_id,source,recorded_at,evidence_kind,certainty) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, r.ID, r.From, r.Predicate, r.To, r.Source, r.RecordedAt, r.EvidenceKind, r.Certainty); err != nil {
			return fmt.Errorf("save relation %q: %w", r.ID, err)
		}
	}
	return tx.Commit()
}

func validateSnapshot(g *graph.Graph) error {
	if g == nil {
		return errors.New("graph is required")
	}
	if len(g.Nodes()) == 0 {
		return errors.New("snapshot must contain Self and at least one node")
	}
	selfCount := 0
	for _, n := range g.Nodes() {
		if n.Type == graph.Self {
			selfCount++
		}
	}
	if selfCount != 1 {
		return fmt.Errorf("snapshot must contain exactly one Self, found %d", selfCount)
	}
	for _, n := range g.Nodes() {
		if _, ok := g.PathFromSelf(n.ID); !ok {
			return fmt.Errorf("node %q is disconnected from Self", n.ID)
		}
	}
	return nil
}

// LoadSnapshot reconstructs and validates the persisted graph.
func (s *Store) LoadSnapshot(ctx context.Context) (*graph.Graph, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	g := graph.New()
	rows, err := tx.QueryContext(ctx, `SELECT id,type,name,lifecycle,last_confirmed,model_version,source,evidence_kind,certainty FROM nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var n graph.Node
		var typ string
		var evidenceKind, certainty string
		var confirmed sql.NullTime
		if err = rows.Scan(&n.ID, &typ, &n.Name, &n.Lifecycle, &confirmed, &n.ModelVersion, &n.Source, &evidenceKind, &certainty); err != nil {
			rows.Close()
			return nil, err
		}
		n.Type = graph.NodeType(typ)
		n.EvidenceKind = graph.EvidenceKind(evidenceKind)
		n.Certainty = graph.Certainty(certainty)
		if confirmed.Valid {
			n.LastConfirmed = &confirmed.Time
		}
		if err = g.AddNode(n); err != nil {
			rows.Close()
			return nil, fmt.Errorf("load node %q: %w", n.ID, err)
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT id,from_id,predicate,to_id,source,recorded_at,evidence_kind,certainty FROM relations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r graph.Relation
		var predicate, evidenceKind, certainty string
		if err = rows.Scan(&r.ID, &r.From, &predicate, &r.To, &r.Source, &r.RecordedAt, &evidenceKind, &certainty); err != nil {
			rows.Close()
			return nil, err
		}
		r.Predicate = graph.Predicate(predicate)
		r.EvidenceKind = graph.EvidenceKind(evidenceKind)
		r.Certainty = graph.Certainty(certainty)
		if err = g.AddRelation(r); err != nil {
			rows.Close()
			return nil, fmt.Errorf("load relation %q: %w", r.ID, err)
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(g.Nodes()) == 0 {
		return nil, ErrEmptySnapshot
	}
	if err = validateSnapshot(g); err != nil {
		return nil, fmt.Errorf("persisted snapshot is invalid: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return g, nil
}
