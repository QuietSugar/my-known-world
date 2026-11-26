// Package postgres persists complete graph snapshots in PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrations embed.FS

const schemaVersion = 2

var ErrEmptySnapshot = errors.New("database contains no graph snapshot")

type Store struct{ db *sql.DB }
type syncUndoMetadata struct {
	Source           string                  `json:"source"`
	PreviousState    *graph.SyncSourceState  `json:"previousState,omitempty"`
	PreviousMappings []graph.IdentityMapping `json:"previousMappings"`
	AppliedState     *graph.SyncSourceState  `json:"appliedState,omitempty"`
	AppliedMappings  []graph.IdentityMapping `json:"appliedMappings,omitempty"`
}

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
func (s *Store) DB() *sql.DB  { return s.db }

// Migrate applies the embedded initial schema and records its explicit version.
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// schema_migrations records which schema version a database already has.
	// It has to exist before the version can be read, so it is created here
	// rather than inside the schema file.
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    integer PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	// A single schema file describes the whole current structure. The previous
	// 24-step incremental chain is gone: it only ever needed to run on the
	// development and svc databases, both of which are already at the final
	// shape, and keeping one-way steps around made every start replay them.
	files := []string{"migrations/001_schema.sql", "migrations/002_drop_undo_snapshots.sql"}
	if len(files) != schemaVersion {
		return fmt.Errorf("migration list has %d entries, schemaVersion is %d", len(files), schemaVersion)
	}

	applied := map[int]bool{}
	rows, queryErr := tx.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if queryErr != nil {
		return fmt.Errorf("read applied migrations: %w", queryErr)
	}
	for rows.Next() {
		var version int
		if scanErr := rows.Scan(&version); scanErr != nil {
			rows.Close()
			return fmt.Errorf("read applied migrations: %w", scanErr)
		}
		applied[version] = true
	}
	rows.Close()

	for index, name := range files {
		version := index + 1
		if applied[version] {
			continue
		}
		migration, readErr := migrations.ReadFile(name)
		if readErr != nil {
			return readErr
		}
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
	// Truncate timestamps to microsecond precision, which is what PostgreSQL
	// stores. Without this the fingerprint of the written graph (nanoseconds)
	// never matches the one read back (microseconds), and undo history — which
	// compares the two — becomes permanently unusable.
	g = g.CanonicalizeTimes()
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
	if _, err = tx.ExecContext(ctx, `DELETE FROM sync_sources`); err != nil {
		return err
	}
	if err = clearGraphRows(ctx, tx); err != nil {
		return err
	}
	for _, n := range g.Nodes() {
		questions, marshalErr := json.Marshal(n.ReviewQuestions)
		if marshalErr != nil {
			return marshalErr
		}
		modelDetails, detailErr := n.MarshalModelDetails()
		if detailErr != nil {
			return detailErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO nodes(id,type,name,lifecycle,last_confirmed,model_version,source,evidence_kind,certainty,candidate_type,review_status,review_questions,evidence_reference) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, n.ID, n.Type, n.Name, n.Lifecycle, n.LastConfirmed, n.ModelVersion, n.Source, n.EvidenceKind, n.Certainty, n.CandidateType, n.ReviewStatus, questions, n.EvidenceReference); err != nil {
			return fmt.Errorf("save node %q: %w", n.ID, err)
		}
		if err := insertTypedDetails(ctx, tx, n, modelDetails); err != nil {
			return err
		}
	}
	if err := insertRunningPeriods(ctx, tx, g); err != nil {
		return err
	}
	if err := insertAssertions(ctx, tx, g); err != nil {
		return err
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
	g, err := loadSnapshotTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return g, nil
}

func loadSnapshotTx(ctx context.Context, tx *sql.Tx) (*graph.Graph, error) {
	g := graph.New()
	rows, err := tx.QueryContext(ctx, `SELECT n.id,''::text,n.type,n.name,n.lifecycle,n.last_confirmed,n.model_version,n.source,n.evidence_kind,n.certainty,n.candidate_type,n.review_status,n.review_questions,n.evidence_reference,rp.started_at,rp.ended_at,rp.deployed_version,rp.verification_at,rp.verification_result,rp.evidence_ref,hd.details,md.details,spd.details,sid.details,acd.details,pd.details,dd.details,rd.details,wid.details,nd.details FROM nodes n LEFT JOIN running_periods rp ON rp.node_id=n.id LEFT JOIN hardware_details hd ON hd.node_id=n.id LEFT JOIN machine_details md ON md.node_id=n.id LEFT JOIN software_project_details spd ON spd.node_id=n.id LEFT JOIN software_instance_details sid ON sid.node_id=n.id LEFT JOIN account_credential_details acd ON acd.node_id=n.id LEFT JOIN product_details pd ON pd.node_id=n.id LEFT JOIN document_details dd ON dd.node_id=n.id LEFT JOIN runbook_details rd ON rd.node_id=n.id LEFT JOIN work_item_details wid ON wid.node_id=n.id LEFT JOIN network_details nd ON nd.node_id=n.id ORDER BY n.id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var n graph.Node
		var typ, evidenceKind, certainty, candidateType string
		var confirmed sql.NullTime
		var questions []byte
		var started, ended, verified sql.NullTime
		var deployed, verificationResult, evidenceRef, nodeEvidenceReference sql.NullString
		var hardwareDetails, machineDetails, projectDetails, instanceDetails, credentialDetails, productDetails, documentDetails, runbookDetails, workDetails, networkDetails []byte
		if err = rows.Scan(&n.ID, &n.UUID, &typ, &n.Name, &n.Lifecycle, &confirmed, &n.ModelVersion, &n.Source, &evidenceKind, &certainty, &candidateType, &n.ReviewStatus, &questions, &nodeEvidenceReference, &started, &ended, &deployed, &verified, &verificationResult, &evidenceRef, &hardwareDetails, &machineDetails, &projectDetails, &instanceDetails, &credentialDetails, &productDetails, &documentDetails, &runbookDetails, &workDetails, &networkDetails); err != nil {
			rows.Close()
			return nil, err
		}
		n.Type = graph.NodeType(typ)
		var details []byte
		switch n.Type {
		case graph.Hardware:
			details = hardwareDetails
		case graph.Machine:
			details = machineDetails
		case graph.SoftwareProject:
			details = projectDetails
		case graph.SoftwareInstance:
			details = instanceDetails
		case graph.AccountCredential:
			details = credentialDetails
		case graph.Product:
			details = productDetails
		case graph.Document:
			details = documentDetails
		case graph.Runbook:
			details = runbookDetails
		case graph.WorkItem:
			details = workDetails
		case graph.Network:
			details = networkDetails
		}
		if err = n.UnmarshalModelDetails(details); err != nil {
			rows.Close()
			return nil, fmt.Errorf("load type details for %q: %w", n.ID, err)
		}
		n.CandidateType = graph.NodeType(candidateType)
		if nodeEvidenceReference.Valid {
			n.EvidenceReference = nodeEvidenceReference.String
		}
		if started.Valid {
			n.StartedAt = &started.Time
		}
		if ended.Valid {
			n.EndedAt = &ended.Time
		}
		if deployed.Valid {
			n.DeployedVersion = deployed.String
		}
		if verified.Valid {
			n.VerificationAt = &verified.Time
		}
		if verificationResult.Valid {
			n.VerificationResult = verificationResult.String
		}
		if evidenceRef.Valid {
			n.EvidenceRef = evidenceRef.String
		}
		if err = json.Unmarshal(questions, &n.ReviewQuestions); err != nil {
			rows.Close()
			return nil, fmt.Errorf("load review questions for %q: %w", n.ID, err)
		}
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
	rows, err = tx.QueryContext(ctx, `SELECT id,node_id,field_path,value_hash,reference,source,evidence_kind,certainty,asserted_at FROM field_assertions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a graph.FieldAssertion
		var evidence, certainty string
		if err = rows.Scan(&a.ID, &a.NodeID, &a.FieldPath, &a.ValueHash, &a.Reference, &a.Source, &evidence, &certainty, &a.AssertedAt); err != nil {
			rows.Close()
			return nil, err
		}
		a.EvidenceKind = graph.EvidenceKind(evidence)
		a.Certainty = graph.Certainty(certainty)
		if err = g.AddAssertion(a); err != nil {
			rows.Close()
			return nil, fmt.Errorf("load assertion %q: %w", a.ID, err)
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
	return g, nil
}

func readSourcePolicy(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, source string) (graph.SourcePolicy, bool, error) {
	var p graph.SourcePolicy
	var types, predicates, fields []byte
	err := q.QueryRowContext(ctx, `SELECT source,priority,allowed_node_types,allowed_predicates,allowed_field_paths FROM sync_source_policies WHERE source=$1`, source).Scan(&p.Source, &p.Priority, &types, &predicates, &fields)
	if errors.Is(err, sql.ErrNoRows) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	if err = json.Unmarshal(types, &p.AllowedNodeTypes); err != nil {
		return p, false, err
	}
	if err = json.Unmarshal(predicates, &p.AllowedPredicates); err != nil {
		return p, false, err
	}
	if err = json.Unmarshal(fields, &p.AllowedFieldPaths); err != nil {
		return p, false, err
	}
	return p, true, nil
}
func (s *Store) SourcePolicies(ctx context.Context) ([]graph.SourcePolicy, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source,priority,allowed_node_types,allowed_predicates,allowed_field_paths FROM sync_source_policies ORDER BY source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []graph.SourcePolicy{}
	for rows.Next() {
		var p graph.SourcePolicy
		var t, r, f []byte
		if err = rows.Scan(&p.Source, &p.Priority, &t, &r, &f); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(t, &p.AllowedNodeTypes); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(r, &p.AllowedPredicates); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(f, &p.AllowedFieldPaths); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) SetSourcePolicy(ctx context.Context, p graph.SourcePolicy) error {
	plan, _, err := s.PreviewSourcePolicy(ctx, p)
	if err != nil {
		return err
	}
	return s.ApplySourcePolicy(ctx, p, plan.ExpectedRevision)
}

// ApplySync atomically reconciles a source batch with the persisted graph.
func (s *Store) ApplySync(ctx context.Context, batch graph.SourceBatch) (*graph.SyncPreview, error) {
	return s.applySync(ctx, batch, "")
}

func (s *Store) ApplySyncPlan(ctx context.Context, batch graph.SourceBatch, expectedRevision string) (*graph.SyncPreview, error) {
	if expectedRevision == "" {
		return nil, errors.New("expected revision is required")
	}
	return s.applySync(ctx, batch, expectedRevision)
}

func (s *Store) applySync(ctx context.Context, batch graph.SourceBatch, expectedRevision string) (*graph.SyncPreview, error) {
	batch.Source = strings.TrimSpace(batch.Source)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes, relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return nil, fmt.Errorf("lock graph for sync: %w", err)
	}
	syncUndo := syncUndoMetadata{Source: batch.Source, PreviousMappings: []graph.IdentityMapping{}}
	var prior graph.SyncSourceState
	stateErr := tx.QueryRowContext(ctx, `SELECT source,source_hash,parser_version,applied_at FROM sync_sources WHERE source=$1`, batch.Source).Scan(&prior.Source, &prior.SourceHash, &prior.ParserVersion, &prior.AppliedAt)
	if stateErr == nil {
		syncUndo.PreviousState = &prior
	} else if !errors.Is(stateErr, sql.ErrNoRows) {
		return nil, stateErr
	}
	syncUndo.PreviousMappings, err = loadIdentityMappings(ctx, tx, batch.Source)
	if err != nil {
		return nil, err
	}
	batch, err = mergeIdentityMappings(ctx, tx, batch)
	if err != nil {
		return nil, err
	}
	current, err := loadSnapshotTx(ctx, tx)
	if errors.Is(err, ErrEmptySnapshot) {
		current = graph.New()
	} else if err != nil {
		return nil, err
	}
	if expectedRevision != "" {
		if _, err = tx.ExecContext(ctx, `LOCK TABLE sync_sources, sync_identity_mappings, sync_source_policies IN SHARE ROW EXCLUSIVE MODE`); err != nil {
			return nil, err
		}
		revision, revErr := syncRevision(ctx, tx, current)
		if revErr != nil {
			return nil, revErr
		}
		if revision != expectedRevision {
			return nil, errors.New("graph, source mappings, or policies changed since preview; preview again before applying")
		}
	}
	policy, configured, policyErr := readSourcePolicy(ctx, tx, batch.Source)
	if policyErr != nil {
		return nil, policyErr
	}
	if configured {
		violations := policy.ValidateBatch(batch)
		if len(violations) > 0 {
			preview := current.PreviewSync(batch)
			preview.Conflicts = append(preview.Conflicts, violations...)
			sort.Strings(preview.Conflicts)
			return &preview, fmt.Errorf("source policy rejected batch: %s", strings.Join(violations, "; "))
		}
	}
	updated, preview, err := current.ApplySync(batch)
	if err != nil {
		return &preview, err
	}
	updated = updated.CanonicalizeTimes()
	if err = validateSnapshot(updated); err != nil {
		return &preview, err
	}
	if err = saveSnapshotTx(ctx, tx, updated); err != nil {
		return &preview, err
	}
	for _, mapping := range batch.IdentityMappings {
		if _, err = tx.ExecContext(ctx, `INSERT INTO sync_identity_mappings(source,source_id,node_id) VALUES($1,$2,$3) ON CONFLICT(source,source_id) DO UPDATE SET node_id=EXCLUDED.node_id`, batch.Source, mapping.SourceID, mapping.NodeID); err != nil {
			return &preview, fmt.Errorf("save identity mapping %q: %w", mapping.SourceID, err)
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sync_sources(source,source_hash,parser_version,applied_at) VALUES($1,$2,$3,now()) ON CONFLICT(source) DO UPDATE SET source_hash=EXCLUDED.source_hash,parser_version=EXCLUDED.parser_version,applied_at=EXCLUDED.applied_at`, preview.Source, preview.SourceHash, preview.ParserVersion); err != nil {
		return &preview, fmt.Errorf("record sync source state: %w", err)
	}
	var appliedState graph.SyncSourceState
	if err = tx.QueryRowContext(ctx, `SELECT source,source_hash,parser_version,applied_at FROM sync_sources WHERE source=$1`, batch.Source).Scan(&appliedState.Source, &appliedState.SourceHash, &appliedState.ParserVersion, &appliedState.AppliedAt); err != nil {
		return &preview, err
	}
	appliedMappings, err := loadIdentityMappings(ctx, tx, batch.Source)
	if err != nil {
		return &preview, err
	}
	syncUndo.AppliedState = &appliedState
	syncUndo.AppliedMappings = appliedMappings
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		return &preview, err
	}
	var runID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO sync_source_runs(source,source_hash,parser_version,preview) VALUES($1,$2,$3,$4) RETURNING run_id`, preview.Source, preview.SourceHash, preview.ParserVersion, previewJSON).Scan(&runID); err != nil {
		return &preview, fmt.Errorf("record sync history: %w", err)
	}
	if err = appendGraphDiff(ctx, tx, &runID, preview.Source, current, updated); err != nil {
		return &preview, err
	}
	metadata, err := json.Marshal(syncUndo)
	if err != nil {
		return &preview, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_mutation_operations(action,metadata,run_id) VALUES($1,$2,$3)`, "sync:"+batch.Source, metadata, runID); err != nil {
		return &preview, fmt.Errorf("record sync operation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return &preview, err
	}
	return &preview, nil
}

func mergeIdentityMappings(ctx context.Context, tx *sql.Tx, batch graph.SourceBatch) (graph.SourceBatch, error) {
	stored, err := loadIdentityMappings(ctx, tx, batch.Source)
	if err != nil {
		return batch, err
	}
	byID := map[string]string{}
	for _, m := range stored {
		byID[m.SourceID] = m.NodeID
	}
	for _, m := range batch.IdentityMappings {
		if old, ok := byID[m.SourceID]; ok && old != m.NodeID {
			return batch, fmt.Errorf("identity mapping conflict for source id %q", m.SourceID)
		}
		byID[m.SourceID] = m.NodeID
	}
	merged := make([]graph.IdentityMapping, 0, len(byID))
	for sourceID, nodeID := range byID {
		merged = append(merged, graph.IdentityMapping{SourceID: sourceID, NodeID: nodeID})
	}
	batch.IdentityMappings = merged
	return batch, nil
}

// PreviewSync resolves persisted source identity mappings in the same snapshot as the graph.
func (s *Store) PreviewSync(ctx context.Context, batch graph.SourceBatch) (graph.SyncPreview, error) {
	batch.Source = strings.TrimSpace(batch.Source)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return graph.SyncPreview{}, err
	}
	defer tx.Rollback()
	batch, err = mergeIdentityMappings(ctx, tx, batch)
	if err != nil {
		return graph.SyncPreview{}, err
	}
	current, err := loadSnapshotTx(ctx, tx)
	if errors.Is(err, ErrEmptySnapshot) {
		current = graph.New()
	} else if err != nil {
		return graph.SyncPreview{}, err
	}
	result := current.PreviewSync(batch)
	result.ExpectedRevision, err = syncRevision(ctx, tx, current)
	if err != nil {
		return graph.SyncPreview{}, err
	}
	policy, configured, policyErr := readSourcePolicy(ctx, tx, batch.Source)
	if policyErr != nil {
		return graph.SyncPreview{}, policyErr
	}
	if configured {
		result.Conflicts = append(result.Conflicts, policy.ValidateBatch(batch)...)
		sort.Strings(result.Conflicts)
	}
	if err = tx.Commit(); err != nil {
		return graph.SyncPreview{}, err
	}
	return result, nil
}

func (s *Store) SyncSources(ctx context.Context) ([]graph.SyncSourceState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source,source_hash,parser_version,applied_at FROM sync_sources ORDER BY source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []graph.SyncSourceState{}
	for rows.Next() {
		var item graph.SyncSourceState
		if err = rows.Scan(&item.Source, &item.SourceHash, &item.ParserVersion, &item.AppliedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) SyncHistory(ctx context.Context, source string, limit int) ([]graph.SyncRun, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.run_id,r.source,r.source_hash,r.parser_version,r.applied_at,r.preview,(u.undo_id IS NOT NULL) FROM sync_source_runs r LEFT JOIN graph_mutation_operations o ON o.run_id=r.run_id LEFT JOIN graph_operation_undos u ON u.operation_id=o.operation_id WHERE r.source=$1 ORDER BY r.applied_at DESC,r.run_id DESC LIMIT $2`, source, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []graph.SyncRun{}
	for rows.Next() {
		var run graph.SyncRun
		var preview []byte
		if err = rows.Scan(&run.ID, &run.Source, &run.SourceHash, &run.ParserVersion, &run.AppliedAt, &preview, &run.Undone); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(preview, &run.Preview); err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func loadIdentityMappings(ctx context.Context, tx queryer, source string) ([]graph.IdentityMapping, error) {
	rows, err := tx.QueryContext(ctx, `SELECT source_id,node_id FROM sync_identity_mappings WHERE source=$1 ORDER BY source_id`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []graph.IdentityMapping{}
	for rows.Next() {
		var m graph.IdentityMapping
		if err = rows.Scan(&m.SourceID, &m.NodeID); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ReviewProvisional atomically classifies a pending graph candidate.
func (s *Store) ReviewProvisional(ctx context.Context, id string, target graph.NodeType) error {
	return s.ReviewProvisionalWithEvidence(ctx, id, target, graph.ProvisionalReviewEvidence{})
}
func (s *Store) ReviewProvisionalWithEvidence(ctx context.Context, id string, target graph.NodeType, evidence graph.ProvisionalReviewEvidence) error {
	return s.mutateGraph(ctx, "system:review", func(g *graph.Graph) (*graph.Graph, error) {
		return g.ReviewProvisionalWithEvidence(id, target, evidence)
	})
}

// DeleteNode explicitly removes one node without allowing orphaned graph fragments.
func (s *Store) CreateNode(ctx context.Context, node graph.Node, relation graph.Relation) error {
	return s.mutateGraph(ctx, "manual:create", func(g *graph.Graph) (*graph.Graph, error) {
		if err := g.Publish(node, relation); err != nil {
			return nil, err
		}
		return g, nil
	})
}
func (s *Store) PreviewManualMutation(ctx context.Context, mutation graph.ManualMutation) (graph.MutationPlan, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return graph.MutationPlan{}, err
	}
	defer tx.Rollback()
	current, err := loadSnapshotTx(ctx, tx)
	if errors.Is(err, ErrEmptySnapshot) {
		current = graph.New()
	} else if err != nil {
		return graph.MutationPlan{}, err
	}
	revision, err := current.Fingerprint()
	if err != nil {
		return graph.MutationPlan{}, err
	}
	planHash, err := mutation.PlanHashValue()
	if err != nil {
		return graph.MutationPlan{}, err
	}
	updated, err := current.ApplyManualMutation(mutation)
	if err != nil {
		return graph.MutationPlan{}, err
	}
	updated = updated.CanonicalizeTimes()
	if len(updated.Nodes()) > 0 {
		if err = validateSnapshot(updated); err != nil {
			return graph.MutationPlan{}, err
		}
	}
	plan := graph.MutationPlan{ExpectedRevision: revision, PlanHash: planHash, Delta: graph.DiffGraphs(current, updated)}
	if err = tx.Commit(); err != nil {
		return graph.MutationPlan{}, err
	}
	return plan, nil
}

func (s *Store) ApplyManualMutation(ctx context.Context, mutation graph.ManualMutation, expectedRevision, expectedPlanHash string) (graph.MutationPlan, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return graph.MutationPlan{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes,relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return graph.MutationPlan{}, err
	}
	current, err := loadSnapshotTx(ctx, tx)
	if errors.Is(err, ErrEmptySnapshot) {
		current = graph.New()
	} else if err != nil {
		return graph.MutationPlan{}, err
	}
	revision, err := current.Fingerprint()
	if err != nil {
		return graph.MutationPlan{}, err
	}
	if expectedRevision == "" || revision != expectedRevision {
		return graph.MutationPlan{}, errors.New("graph changed since preview; preview again before applying")
	}
	planHash, err := mutation.PlanHashValue()
	if err != nil {
		return graph.MutationPlan{}, err
	}
	if expectedPlanHash == "" || planHash != expectedPlanHash {
		return graph.MutationPlan{}, errors.New("mutation differs from reviewed preview")
	}
	updated, err := current.ApplyManualMutation(mutation)
	if err != nil {
		return graph.MutationPlan{}, err
	}
	updated = updated.CanonicalizeTimes()
	if len(updated.Nodes()) > 0 {
		if err = validateSnapshot(updated); err != nil {
			return graph.MutationPlan{}, err
		}
	}
	if err = saveSnapshotTx(ctx, tx, updated); err != nil {
		return graph.MutationPlan{}, err
	}
	action := "manual:" + mutation.Kind
	if err = appendGraphDiff(ctx, tx, nil, action, current, updated); err != nil {
		return graph.MutationPlan{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_mutation_operations(action) VALUES($1)`, action); err != nil {
		return graph.MutationPlan{}, err
	}
	plan := graph.MutationPlan{ExpectedRevision: revision, PlanHash: planHash, Delta: graph.DiffGraphs(current, updated)}
	if err = tx.Commit(); err != nil {
		return graph.MutationPlan{}, err
	}
	return plan, nil
}

// ApplyCreateBatch atomically creates nodes with their Self anchor relations and
// any additional relations. It records one reversible operation and per-entity
// history rows, exactly like the guarded manual-mutation path. An empty
// expectedRevision opts out of the optimistic concurrency check; when it is
// supplied the whole batch is rejected if the graph moved.
func (s *Store) ApplyCreateBatch(ctx context.Context, batch graph.CreateBatch, expectedRevision string) (graph.CreateResult, string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return graph.CreateResult{}, "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes,relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return graph.CreateResult{}, "", err
	}
	current, err := loadSnapshotTx(ctx, tx)
	if errors.Is(err, ErrEmptySnapshot) {
		current = graph.New()
	} else if err != nil {
		return graph.CreateResult{}, "", err
	}
	revision, err := current.Fingerprint()
	if err != nil {
		return graph.CreateResult{}, "", err
	}
	if expectedRevision != "" && revision != expectedRevision {
		return graph.CreateResult{}, "", errors.New("graph changed since this batch was prepared; fetch the current revision and retry")
	}
	updated, result, err := current.ApplyCreateBatch(batch)
	if err != nil {
		return graph.CreateResult{}, "", err
	}
	updated = updated.CanonicalizeTimes()
	if err = validateSnapshot(updated); err != nil {
		return graph.CreateResult{}, "", err
	}
	if err = saveSnapshotTx(ctx, tx, updated); err != nil {
		return graph.CreateResult{}, "", err
	}
	if err = appendGraphDiff(ctx, tx, nil, "manual:create", current, updated); err != nil {
		return graph.CreateResult{}, "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_mutation_operations(action) VALUES($1)`, "manual:createBatch"); err != nil {
		return graph.CreateResult{}, "", fmt.Errorf("record operation: %w", err)
	}
	newRevision, err := updated.Fingerprint()
	if err != nil {
		return graph.CreateResult{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return graph.CreateResult{}, "", err
	}
	return result, newRevision, nil
}

func (s *Store) UpdateNode(ctx context.Context, id string, node graph.Node) error {
	return s.mutateGraph(ctx, "manual:edit", func(g *graph.Graph) (*graph.Graph, error) { return g.UpdateNode(id, node) })
}
func (s *Store) AddRelation(ctx context.Context, relation graph.Relation) error {
	return s.mutateGraph(ctx, "manual:relation", func(g *graph.Graph) (*graph.Graph, error) { return g.AddRelationChecked(relation) })
}
func (s *Store) DeleteRelation(ctx context.Context, id string) error {
	return s.mutateGraph(ctx, "manual:relation", func(g *graph.Graph) (*graph.Graph, error) { return g.DeleteRelation(id) })
}

func (s *Store) DeleteNode(ctx context.Context, id string) error {
	return s.mutateGraph(ctx, "system:retire", func(g *graph.Graph) (*graph.Graph, error) { return g.DeleteNode(id) })
}

// RejectProvisional atomically closes a candidate review without deleting it.
func (s *Store) RejectProvisional(ctx context.Context, id string) error {
	return s.mutateGraph(ctx, "system:review", func(g *graph.Graph) (*graph.Graph, error) { return g.RejectProvisional(id) })
}

func (s *Store) mutateGraph(ctx context.Context, source string, mutate func(*graph.Graph) (*graph.Graph, error)) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes, relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return err
	}
	current, err := loadSnapshotTx(ctx, tx)
	if err != nil {
		return err
	}
	updated, err := mutate(current.Clone())
	if err != nil {
		return err
	}
	updated = updated.CanonicalizeTimes()
	if err = validateSnapshot(updated); err != nil {
		return err
	}
	if err = saveSnapshotTx(ctx, tx, updated); err != nil {
		return err
	}
	if err = appendGraphDiff(ctx, tx, nil, source, current, updated); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_mutation_operations(action) VALUES($1)`, source); err != nil {
		return fmt.Errorf("record operation: %w", err)
	}
	return tx.Commit()
}

func (s *Store) mutateWithOptimisticRevision(
	ctx context.Context,
	source string,
	expectedRevision string,
	mutate func(current *graph.Graph) (*graph.Graph, any, error),
) (any, string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes, relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return nil, "", err
	}
	current, err := loadSnapshotTx(ctx, tx)
	if errors.Is(err, ErrEmptySnapshot) {
		current = graph.New()
	} else if err != nil {
		return nil, "", err
	}
	revision, err := current.Fingerprint()
	if err != nil {
		return nil, "", err
	}
	if expectedRevision != "" && revision != expectedRevision {
		return nil, "", errors.New("graph changed since this request was prepared; fetch the current revision and retry")
	}
	updated, result, err := mutate(current)
	if err != nil {
		return nil, "", err
	}
	updated = updated.CanonicalizeTimes()
	if err = validateSnapshot(updated); err != nil {
		return nil, "", err
	}
	if err = saveSnapshotTx(ctx, tx, updated); err != nil {
		return nil, "", err
	}
	if err = appendGraphDiff(ctx, tx, nil, source, current, updated); err != nil {
		return nil, "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_mutation_operations(action) VALUES($1)`, source); err != nil {
		return nil, "", fmt.Errorf("record operation: %w", err)
	}
	newRevision, err := updated.Fingerprint()
	if err != nil {
		return nil, "", err
	}
	if err = tx.Commit(); err != nil {
		return nil, "", err
	}
	return result, newRevision, nil
}

func (s *Store) PatchNode(ctx context.Context, id string, patch graph.Node, expectedRevision string) (graph.Node, string, error) {
	if expectedRevision == "" {
		return graph.Node{}, "", errors.New("expectedRevision is required")
	}
	var patchedNode graph.Node
	_, rev, err := s.mutateWithOptimisticRevision(ctx, "manual:patchNode", expectedRevision, func(current *graph.Graph) (*graph.Graph, any, error) {
		old, ok := current.Node(id)
		if !ok {
			return nil, nil, fmt.Errorf("node %q not found", id)
		}
		if old.Type == graph.Self {
			return nil, nil, errors.New("Self cannot be patched")
		}
		if patch.ID != "" && patch.ID != id {
			return nil, nil, errors.New("node ID is immutable")
		}
		if patch.Type != "" && patch.Type != old.Type {
			return nil, nil, errors.New("node type is immutable; use provisional review")
		}
		merged := old
		if patch.Name != "" {
			merged.Name = patch.Name
		}
		if patch.Lifecycle != "" {
			merged.Lifecycle = patch.Lifecycle
		}
		if patch.Certainty != "" {
			merged.Certainty = patch.Certainty
		}
		if patch.EvidenceKind != "" {
			merged.EvidenceKind = patch.EvidenceKind
		}
		if patch.ModelVersion > 0 {
			merged.ModelVersion = patch.ModelVersion
		}
		if patch.Machine != nil {
			merged.Machine = patch.Machine
		}
		if patch.SoftwareProject != nil {
			merged.SoftwareProject = patch.SoftwareProject
		}
		if patch.SoftwareInstance != nil {
			merged.SoftwareInstance = patch.SoftwareInstance
		}
		if patch.Product != nil {
			merged.Product = patch.Product
		}
		if patch.Runbook != nil {
			merged.Runbook = patch.Runbook
		}
		if patch.WorkItem != nil {
			merged.WorkItem = patch.WorkItem
		}
		if patch.Document != nil {
			merged.Document = patch.Document
		}
		if patch.Network != nil {
			merged.Network = patch.Network
		}
		if patch.Hardware != nil {
			merged.Hardware = patch.Hardware
		}
		next, err := current.UpdateNode(id, merged)
		if err != nil {
			return nil, nil, err
		}
		patchedNode = merged
		return next, merged, nil
	})
	if err != nil {
		return graph.Node{}, "", err
	}
	return patchedNode, rev, nil
}

func (s *Store) RetireNode(ctx context.Context, id string, expectedRevision string) (graph.Node, string, error) {
	var retiredNode graph.Node
	_, rev, err := s.mutateWithOptimisticRevision(ctx, "manual:retireNode", expectedRevision, func(current *graph.Graph) (*graph.Graph, any, error) {
		old, ok := current.Node(id)
		if !ok {
			return nil, nil, fmt.Errorf("node %q not found", id)
		}
		if old.Type == graph.Self {
			return nil, nil, errors.New("Self cannot be retired")
		}
		next, err := current.RetireNode(id)
		if err != nil {
			return nil, nil, err
		}
		retired, _ := next.Node(id)
		retiredNode = retired
		return next, retired, nil
	})
	if err != nil {
		return graph.Node{}, "", err
	}
	return retiredNode, rev, nil
}

func (s *Store) DeleteNodeWithRevision(ctx context.Context, id string, expectedRevision string) (string, error) {
	if expectedRevision == "" {
		return "", errors.New("expectedRevision is required")
	}
	// Removal is retirement: the node and its relations stay, only the lifecycle
	// changes. Nothing else has to be dismantled first, and the call is
	// reversible by patching the lifecycle back.
	_, rev, err := s.mutateWithOptimisticRevision(ctx, "manual:retireNode", expectedRevision, func(current *graph.Graph) (*graph.Graph, any, error) {
		next, err := current.DeleteNode(id)
		if err != nil {
			return nil, nil, err
		}
		return next, id, nil
	})
	return rev, err
}

func (s *Store) PatchRelation(ctx context.Context, id string, patch graph.Relation, expectedRevision string) (graph.Relation, string, error) {
	if expectedRevision == "" {
		return graph.Relation{}, "", errors.New("expectedRevision is required")
	}
	var patchedRel graph.Relation
	_, rev, err := s.mutateWithOptimisticRevision(ctx, "manual:patchRelation", expectedRevision, func(current *graph.Graph) (*graph.Graph, any, error) {
		old, ok := current.Relation(id)
		if !ok {
			return nil, nil, fmt.Errorf("relation %q not found", id)
		}
		if patch.ID != "" && patch.ID != id {
			return nil, nil, errors.New("relation ID is immutable")
		}
		if (patch.From != "" && patch.From != old.From) ||
			(patch.To != "" && patch.To != old.To) ||
			(patch.Predicate != "" && patch.Predicate != old.Predicate) {
			return nil, nil, errors.New("relation endpoints and predicate are immutable; replace through explicit delete/add")
		}
		merged := old
		if patch.Certainty != "" {
			merged.Certainty = patch.Certainty
		}
		if patch.EvidenceKind != "" {
			merged.EvidenceKind = patch.EvidenceKind
		}
		next, err := current.UpdateRelation(id, merged)
		if err != nil {
			return nil, nil, err
		}
		patchedRel = merged
		return next, merged, nil
	})
	if err != nil {
		return graph.Relation{}, "", err
	}
	return patchedRel, rev, nil
}

func (s *Store) DeleteRelationWithRevision(ctx context.Context, id string, expectedRevision string) (string, error) {
	if expectedRevision == "" {
		return "", errors.New("expectedRevision is required")
	}
	_, rev, err := s.mutateWithOptimisticRevision(ctx, "manual:deleteRelation", expectedRevision, func(current *graph.Graph) (*graph.Graph, any, error) {
		if _, ok := current.Relation(id); !ok {
			return nil, nil, fmt.Errorf("relation %q not found", id)
		}
		next, err := current.DeleteRelation(id)
		if err != nil {
			return nil, nil, err
		}
		return next, id, nil
	})
	return rev, err
}

func appendGraphDiff(ctx context.Context, tx *sql.Tx, runID *int64, source string, before, after *graph.Graph) error {
	if err := recordEntityDiff(ctx, tx, runID, "node", source, before.Nodes(), after.Nodes(), func(v graph.Node) string { return v.ID }, func(v graph.Node) string { return v.Source }, func(v graph.Node) bool { return v.Certainty == graph.CertaintyStale }); err != nil {
		return err
	}
	if err := recordEntityDiff(ctx, tx, runID, "relation", source, before.Relations(), after.Relations(), func(v graph.Relation) string { return v.ID }, func(v graph.Relation) string { return v.Source }, func(v graph.Relation) bool { return v.Certainty == graph.CertaintyStale }); err != nil {
		return err
	}
	return recordEntityDiff(ctx, tx, runID, "assertion", source, before.Assertions(), after.Assertions(), func(v graph.FieldAssertion) string { return v.ID }, func(v graph.FieldAssertion) string { return v.Source }, func(v graph.FieldAssertion) bool { return v.Certainty == graph.CertaintyStale })
}

func recordEntityDiff[T any](ctx context.Context, tx *sql.Tx, runID *int64, kind, defaultSource string, before, after []T, idOf func(T) string, sourceOf func(T) string, stale func(T) bool) error {
	oldMap := map[string]T{}
	newMap := map[string]T{}
	for _, item := range before {
		oldMap[idOf(item)] = item
	}
	for _, item := range after {
		newMap[idOf(item)] = item
	}
	set := map[string]bool{}
	for id := range oldMap {
		set[id] = true
	}
	for id := range newMap {
		set[id] = true
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var run any
	if runID != nil {
		run = *runID
	}
	for _, id := range ids {
		old, oldOK := oldMap[id]
		current, newOK := newMap[id]
		if oldOK && newOK && reflect.DeepEqual(old, current) {
			continue
		}
		action := "updated"
		source := defaultSource
		var snapshot any
		switch {
		case !oldOK && newOK:
			action = "created"
			source = sourceOf(current)
			snapshot = current
		case oldOK && !newOK:
			action = "deleted"
			source = sourceOf(old)
		case oldOK && newOK:
			source = sourceOf(current)
			snapshot = current
			if !stale(old) && stale(current) {
				action = "stale"
			}
		}
		var raw any
		if snapshot != nil {
			b, err := json.Marshal(snapshot)
			if err != nil {
				return err
			}
			raw = b
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO graph_change_history(run_id,source,entity_type,entity_id,action,snapshot) VALUES($1,$2,$3,$4,$5,$6)`, run, source, kind, id, action, raw); err != nil {
			return fmt.Errorf("record %s history for %q: %w", kind, id, err)
		}
	}
	return nil
}
func (s *Store) EntityHistory(ctx context.Context, kind, id string, limit int) ([]graph.ChangeEvent, error) {
	switch kind {
	case "node", "relation", "assertion":
	default:
		return nil, fmt.Errorf("unknown history entity type %q", kind)
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT history_id,run_id,source,entity_type,entity_id,action,recorded_at,snapshot FROM graph_change_history WHERE entity_type=$1 AND entity_id=$2 ORDER BY recorded_at DESC,history_id DESC LIMIT $3`, kind, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []graph.ChangeEvent{}
	for rows.Next() {
		var event graph.ChangeEvent
		var run sql.NullInt64
		var snapshot []byte
		if err = rows.Scan(&event.ID, &run, &event.Source, &event.EntityType, &event.EntityID, &event.Action, &event.RecordedAt, &snapshot); err != nil {
			return nil, err
		}
		if run.Valid {
			event.RunID = &run.Int64
		}
		event.Snapshot = snapshot
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Store) ResolveField(ctx context.Context, resolution graph.FieldResolution) (graph.FieldResolution, error) {
	plan, err := s.PreviewFieldResolution(ctx, resolution)
	if err != nil {
		return resolution, err
	}
	return s.ApplyFieldResolution(ctx, resolution, plan.ExpectedRevision)
}

func (s *Store) AssertionViews(ctx context.Context, nodeID string) ([]graph.AssertionView, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	g, err := loadSnapshotTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	if _, ok := g.Node(nodeID); !ok {
		return nil, fmt.Errorf("node %q not found", nodeID)
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT ON(field_path) resolution_id,field_path,selected_assertion_ids FROM field_resolutions WHERE node_id=$1 ORDER BY field_path,recorded_at DESC,resolution_id DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	decisions := []graph.FieldResolution{}
	for rows.Next() {
		var item graph.FieldResolution
		var ids []byte
		if err = rows.Scan(&item.ID, &item.FieldPath, &ids); err != nil {
			rows.Close()
			return nil, err
		}
		if err = json.Unmarshal(ids, &item.SelectedAssertionIDs); err != nil {
			rows.Close()
			return nil, err
		}
		decisions = append(decisions, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	views := graph.BuildAssertionViews(g.AssertionsForNode(nodeID), decisions)
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return views, nil
}

func (s *Store) FieldResolutions(ctx context.Context, nodeID string) ([]graph.FieldResolution, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT resolution_id,node_id,field_path,selected_assertion_ids,rationale_reference,recorded_at FROM field_resolutions WHERE node_id=$1 ORDER BY recorded_at DESC,resolution_id DESC LIMIT 100`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []graph.FieldResolution{}
	for rows.Next() {
		var item graph.FieldResolution
		var ids []byte
		if err = rows.Scan(&item.ID, &item.NodeID, &item.FieldPath, &ids, &item.RationaleReference, &item.RecordedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(ids, &item.SelectedAssertionIDs); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func insertTypedDetails(ctx context.Context, tx *sql.Tx, n graph.Node, raw []byte) error {
	if len(raw) == 0 || string(raw) == `{}` {
		return nil
	}
	table := map[graph.NodeType]string{graph.Hardware: "hardware_details", graph.Machine: "machine_details", graph.SoftwareProject: "software_project_details", graph.SoftwareInstance: "software_instance_details", graph.AccountCredential: "account_credential_details", graph.Product: "product_details", graph.Document: "document_details", graph.Runbook: "runbook_details", graph.WorkItem: "work_item_details", graph.Network: "network_details"}[n.Type]
	if table == "" {
		return fmt.Errorf("no detail table for node type %s", n.Type)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO `+table+`(node_id,details) VALUES($1,$2)`, n.ID, raw)
	if err != nil {
		return fmt.Errorf("save %s details for %q: %w", n.Type, n.ID, err)
	}
	return nil
}

func insertRunningPeriods(ctx context.Context, tx *sql.Tx, g *graph.Graph) error {
	for _, n := range g.Nodes() {
		if n.Type != graph.RunningPeriod {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO running_periods(node_id,started_at,ended_at,deployed_version,verification_at,verification_result,evidence_ref) VALUES($1,$2,$3,$4,$5,$6,$7)`, n.ID, n.StartedAt, n.EndedAt, n.DeployedVersion, n.VerificationAt, n.VerificationResult, n.EvidenceRef); err != nil {
			return fmt.Errorf("save running period %q: %w", n.ID, err)
		}
	}
	return nil
}

func insertAssertions(ctx context.Context, tx *sql.Tx, g *graph.Graph) error {
	for _, a := range g.Assertions() {
		if _, err := tx.ExecContext(ctx, `INSERT INTO field_assertions(id,node_id,field_path,value_hash,reference,source,evidence_kind,certainty,asserted_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, a.ID, a.NodeID, a.FieldPath, a.ValueHash, a.Reference, a.Source, a.EvidenceKind, a.Certainty, a.AssertedAt); err != nil {
			return fmt.Errorf("save field assertion %q: %w", a.ID, err)
		}
	}
	return nil
}

// detailTables lists every table that hangs off a node. They used to be cleared
// by an ON DELETE CASCADE from nodes.uuid; that foreign key went away when the
// redundant uuid column was removed, so snapshot replacement has to clear them
// explicitly or a re-save collides with the previous row.
var detailTables = []string{
	"hardware_details", "machine_details", "software_project_details",
	"software_instance_details", "account_credential_details", "product_details",
	"document_details", "runbook_details", "work_item_details", "network_details",
	"running_periods",
}

// clearGraphRows removes the previous snapshot, children first.
func clearGraphRows(ctx context.Context, tx *sql.Tx) error {
	for _, table := range detailTables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM field_assertions`); err != nil {
		return fmt.Errorf("clear field_assertions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM relations`); err != nil {
		return fmt.Errorf("clear relations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM nodes`); err != nil {
		return fmt.Errorf("clear nodes: %w", err)
	}
	return nil
}

func saveSnapshotTx(ctx context.Context, tx *sql.Tx, g *graph.Graph) error {
	g = g.CanonicalizeTimes()
	if err := clearGraphRows(ctx, tx); err != nil {
		return err
	}
	for _, n := range g.Nodes() {
		questions, marshalErr := json.Marshal(n.ReviewQuestions)
		if marshalErr != nil {
			return marshalErr
		}
		modelDetails, detailErr := n.MarshalModelDetails()
		if detailErr != nil {
			return detailErr
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO nodes(id,type,name,lifecycle,last_confirmed,model_version,source,evidence_kind,certainty,candidate_type,review_status,review_questions,evidence_reference) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, n.ID, n.Type, n.Name, n.Lifecycle, n.LastConfirmed, n.ModelVersion, n.Source, n.EvidenceKind, n.Certainty, n.CandidateType, n.ReviewStatus, questions, n.EvidenceReference); err != nil {
			return fmt.Errorf("save node %q: %w", n.ID, err)
		}
		if err := insertTypedDetails(ctx, tx, n, modelDetails); err != nil {
			return err
		}
	}
	if err := insertRunningPeriods(ctx, tx, g); err != nil {
		return err
	}
	if err := insertAssertions(ctx, tx, g); err != nil {
		return err
	}
	for _, r := range g.Relations() {
		if _, err := tx.ExecContext(ctx, `INSERT INTO relations(id,from_id,predicate,to_id,source,recorded_at,evidence_kind,certainty) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, r.ID, r.From, r.Predicate, r.To, r.Source, r.RecordedAt, r.EvidenceKind, r.Certainty); err != nil {
			return fmt.Errorf("save relation %q: %w", r.ID, err)
		}
	}
	return nil
}
