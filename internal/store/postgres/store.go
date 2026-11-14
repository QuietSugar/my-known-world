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

const schemaVersion = 16

var ErrEmptySnapshot = errors.New("database contains no graph snapshot")

type Store struct{ db *sql.DB }
type syncUndoMetadata struct {
	Source           string                  `json:"source"`
	PreviousState    *graph.SyncSourceState  `json:"previousState,omitempty"`
	PreviousMappings []graph.IdentityMapping `json:"previousMappings"`
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

// Migrate applies the embedded initial schema and records its explicit version.
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	files := []string{"migrations/001_init.sql", "migrations/002_provenance.sql", "migrations/003_sync_state.sql", "migrations/004_identity_mappings.sql", "migrations/005_provisional_review.sql", "migrations/006_running_periods.sql", "migrations/007_sync_history.sql", "migrations/008_field_assertions.sql", "migrations/009_typed_model_details.sql", "migrations/010_graph_change_history.sql", "migrations/011_field_resolutions.sql", "migrations/012_source_policies.sql", "migrations/013_usage_relationships.sql", "migrations/014_network_model.sql", "migrations/015_reversible_operations.sql", "migrations/016_sync_undo_metadata.sql"}
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
	if _, err = tx.ExecContext(ctx, `DELETE FROM sync_sources`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM relations`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM nodes`); err != nil {
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
		if _, err = tx.ExecContext(ctx, `INSERT INTO nodes(id,type,name,lifecycle,last_confirmed,model_version,source,evidence_kind,certainty,candidate_type,review_status,review_questions) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, n.ID, n.Type, n.Name, n.Lifecycle, n.LastConfirmed, n.ModelVersion, n.Source, n.EvidenceKind, n.Certainty, n.CandidateType, n.ReviewStatus, questions); err != nil {
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
	rows, err := tx.QueryContext(ctx, `SELECT n.id,n.type,n.name,n.lifecycle,n.last_confirmed,n.model_version,n.source,n.evidence_kind,n.certainty,n.candidate_type,n.review_status,n.review_questions,rp.started_at,rp.ended_at,rp.deployed_version,rp.verification_at,rp.verification_result,rp.evidence_ref,hd.details,md.details,spd.details,sid.details,acd.details,pd.details,dd.details,rd.details,wid.details,nd.details FROM nodes n LEFT JOIN running_periods rp ON rp.node_id=n.id LEFT JOIN hardware_details hd ON hd.node_id=n.id LEFT JOIN machine_details md ON md.node_id=n.id LEFT JOIN software_project_details spd ON spd.node_id=n.id LEFT JOIN software_instance_details sid ON sid.node_id=n.id LEFT JOIN account_credential_details acd ON acd.node_id=n.id LEFT JOIN product_details pd ON pd.node_id=n.id LEFT JOIN document_details dd ON dd.node_id=n.id LEFT JOIN runbook_details rd ON rd.node_id=n.id LEFT JOIN work_item_details wid ON wid.node_id=n.id LEFT JOIN network_details nd ON nd.node_id=n.id ORDER BY n.id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var n graph.Node
		var typ, evidenceKind, certainty, candidateType string
		var confirmed sql.NullTime
		var questions []byte
		var started, ended, verified sql.NullTime
		var deployed, verificationResult, evidenceRef sql.NullString
		var hardwareDetails, machineDetails, projectDetails, instanceDetails, credentialDetails, productDetails, documentDetails, runbookDetails, workDetails, networkDetails []byte
		if err = rows.Scan(&n.ID, &typ, &n.Name, &n.Lifecycle, &confirmed, &n.ModelVersion, &n.Source, &evidenceKind, &certainty, &candidateType, &n.ReviewStatus, &questions, &started, &ended, &deployed, &verified, &verificationResult, &evidenceRef, &hardwareDetails, &machineDetails, &projectDetails, &instanceDetails, &credentialDetails, &productDetails, &documentDetails, &runbookDetails, &workDetails, &networkDetails); err != nil {
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
	if err := p.Validate(); err != nil {
		return err
	}
	t, _ := json.Marshal(p.AllowedNodeTypes)
	r, _ := json.Marshal(p.AllowedPredicates)
	f, _ := json.Marshal(p.AllowedFieldPaths)
	_, err := s.db.ExecContext(ctx, `INSERT INTO sync_source_policies(source,priority,allowed_node_types,allowed_predicates,allowed_field_paths) VALUES($1,$2,$3,$4,$5) ON CONFLICT(source) DO UPDATE SET priority=EXCLUDED.priority,allowed_node_types=EXCLUDED.allowed_node_types,allowed_predicates=EXCLUDED.allowed_predicates,allowed_field_paths=EXCLUDED.allowed_field_paths,updated_at=now()`, p.Source, p.Priority, t, r, f)
	return err
}

// ApplySync atomically reconciles a source batch with the persisted graph.
func (s *Store) ApplySync(ctx context.Context, batch graph.SourceBatch) (*graph.SyncPreview, error) {
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
	beforeState, _ := json.Marshal(current.Snapshot())
	afterState, _ := json.Marshal(updated.Snapshot())
	afterHash, err := updated.Fingerprint()
	if err != nil {
		return &preview, err
	}
	metadata, err := json.Marshal(syncUndo)
	if err != nil {
		return &preview, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_mutation_operations(action,before_state,after_state,after_hash,metadata,run_id) VALUES($1,$2,$3,$4,$5,$6)`, "sync:"+batch.Source, beforeState, afterState, afterHash, metadata, runID); err != nil {
		return &preview, fmt.Errorf("record reversible sync operation: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE graph_mutation_operations SET before_state='{}'::jsonb,after_state='{}'::jsonb WHERE operation_id NOT IN (SELECT operation_id FROM graph_mutation_operations ORDER BY recorded_at DESC,operation_id DESC LIMIT 50)`); err != nil {
		return &preview, err
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
	batch.IdentityMappings = batch.IdentityMappings[:0]
	for sourceID, nodeID := range byID {
		batch.IdentityMappings = append(batch.IdentityMappings, graph.IdentityMapping{SourceID: sourceID, NodeID: nodeID})
	}
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

func loadIdentityMappings(ctx context.Context, tx *sql.Tx, source string) ([]graph.IdentityMapping, error) {
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
	return s.mutateGraph(ctx, "system:review", func(g *graph.Graph) (*graph.Graph, error) { return g.ReviewProvisional(id, target) })
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
	before, _ := json.Marshal(current.Snapshot())
	after, _ := json.Marshal(updated.Snapshot())
	afterHash, err := updated.Fingerprint()
	if err != nil {
		return graph.MutationPlan{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_mutation_operations(action,before_state,after_state,after_hash) VALUES($1,$2,$3,$4)`, action, before, after, afterHash); err != nil {
		return graph.MutationPlan{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE graph_mutation_operations SET before_state='{}'::jsonb,after_state='{}'::jsonb WHERE operation_id NOT IN (SELECT operation_id FROM graph_mutation_operations ORDER BY recorded_at DESC,operation_id DESC LIMIT 50)`); err != nil {
		return graph.MutationPlan{}, err
	}
	plan := graph.MutationPlan{ExpectedRevision: revision, PlanHash: planHash, Delta: graph.DiffGraphs(current, updated)}
	if err = tx.Commit(); err != nil {
		return graph.MutationPlan{}, err
	}
	return plan, nil
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
	return s.mutateGraph(ctx, "system:delete", func(g *graph.Graph) (*graph.Graph, error) { return g.DeleteNode(id) })
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
	beforeJSON, _ := json.Marshal(current.Snapshot())
	afterJSON, _ := json.Marshal(updated.Snapshot())
	afterHash, hashErr := updated.Fingerprint()
	if hashErr != nil {
		return hashErr
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_mutation_operations(action,before_state,after_state,after_hash) VALUES($1,$2,$3,$4)`, source, beforeJSON, afterJSON, afterHash); err != nil {
		return fmt.Errorf("record reversible operation: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE graph_mutation_operations SET before_state='{}'::jsonb,after_state='{}'::jsonb WHERE operation_id NOT IN (SELECT operation_id FROM graph_mutation_operations ORDER BY recorded_at DESC,operation_id DESC LIMIT 50)`); err != nil {
		return fmt.Errorf("prune old undo snapshots: %w", err)
	}
	return tx.Commit()
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

func (s *Store) MutationOperations(ctx context.Context, limit int) ([]graph.MutationOperation, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	current, err := s.LoadSnapshot(ctx)
	if errors.Is(err, ErrEmptySnapshot) {
		current = graph.New()
	} else if err != nil {
		return nil, err
	}
	hash, err := current.Fingerprint()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT o.operation_id,o.action,o.recorded_at,o.after_hash,(u.undo_id IS NOT NULL),(o.before_state <> '{}'::jsonb) FROM graph_mutation_operations o LEFT JOIN graph_operation_undos u ON u.operation_id=o.operation_id ORDER BY o.recorded_at DESC,o.operation_id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []graph.MutationOperation{}
	for rows.Next() {
		var item graph.MutationOperation
		var afterHash string
		var undone, snapshotAvailable bool
		if err = rows.Scan(&item.ID, &item.Action, &item.RecordedAt, &afterHash, &undone, &snapshotAvailable); err != nil {
			return nil, err
		}
		item.Reversible = !undone && snapshotAvailable && afterHash == hash
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UndoOperation(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes,relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return err
	}
	var beforeJSON, metadataJSON []byte
	var expected, action string
	err = tx.QueryRowContext(ctx, `SELECT before_state,after_hash,action,metadata FROM graph_mutation_operations WHERE operation_id=$1`, id).Scan(&beforeJSON, &expected, &action, &metadataJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("operation %d not found", id)
	}
	if err != nil {
		return err
	}
	if string(beforeJSON) == `{}` {
		return errors.New("undo snapshot expired; the operation is audit-only")
	}
	var already bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM graph_operation_undos WHERE operation_id=$1)`, id).Scan(&already); err != nil {
		return err
	}
	if already {
		return fmt.Errorf("operation %d was already undone", id)
	}
	current, err := loadSnapshotTx(ctx, tx)
	if errors.Is(err, ErrEmptySnapshot) {
		current = graph.New()
	} else if err != nil {
		return err
	}
	currentHash, err := current.Fingerprint()
	if err != nil {
		return err
	}
	if currentHash != expected {
		return errors.New("undo rejected: graph changed after this operation; only the latest unchanged operation can be undone")
	}
	var snapshot graph.Snapshot
	if err = json.Unmarshal(beforeJSON, &snapshot); err != nil {
		return err
	}
	previous, err := graph.RestoreSnapshot(snapshot)
	if err != nil {
		return fmt.Errorf("stored undo snapshot invalid: %w", err)
	}
	if len(previous.Nodes()) > 0 {
		if err = validateSnapshot(previous); err != nil {
			return err
		}
	}
	if err = saveSnapshotTx(ctx, tx, previous); err != nil {
		return err
	}
	if err = appendGraphDiff(ctx, tx, nil, "system:undo", current, previous); err != nil {
		return err
	}
	beforeHash, err := previous.Fingerprint()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO graph_operation_undos(operation_id,before_hash) VALUES($1,$2)`, id, beforeHash); err != nil {
		return err
	}
	if strings.HasPrefix(action, "sync:") {
		var meta syncUndoMetadata
		if err = json.Unmarshal(metadataJSON, &meta); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM sync_sources WHERE source=$1`, meta.Source); err != nil {
			return err
		}
		if meta.PreviousState != nil {
			p := meta.PreviousState
			if _, err = tx.ExecContext(ctx, `INSERT INTO sync_sources(source,source_hash,parser_version,applied_at) VALUES($1,$2,$3,$4)`, p.Source, p.SourceHash, p.ParserVersion, p.AppliedAt); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM sync_identity_mappings WHERE source=$1`, meta.Source); err != nil {
			return err
		}
		for _, mapping := range meta.PreviousMappings {
			if _, err = tx.ExecContext(ctx, `INSERT INTO sync_identity_mappings(source,source_id,node_id) VALUES($1,$2,$3)`, meta.Source, mapping.SourceID, mapping.NodeID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
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
	if err := resolution.Validate(); err != nil {
		return resolution, err
	}
	ids, err := json.Marshal(resolution.SelectedAssertionIDs)
	if err != nil {
		return resolution, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return resolution, err
	}
	defer tx.Rollback()
	for _, id := range resolution.SelectedAssertionIDs {
		var nodeID, field string
		if err = tx.QueryRowContext(ctx, `SELECT node_id,field_path FROM field_assertions WHERE id=$1`, id).Scan(&nodeID, &field); err != nil {
			return resolution, fmt.Errorf("selected assertion %q not found", id)
		}
		if nodeID != resolution.NodeID || field != resolution.FieldPath {
			return resolution, fmt.Errorf("selected assertion %q belongs to another field", id)
		}
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO field_resolutions(node_id,field_path,selected_assertion_ids,rationale_reference) VALUES($1,$2,$3,$4) RETURNING resolution_id,recorded_at`, resolution.NodeID, resolution.FieldPath, ids, resolution.RationaleReference).Scan(&resolution.ID, &resolution.RecordedAt)
	if err != nil {
		return resolution, err
	}
	return resolution, tx.Commit()
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

func saveSnapshotTx(ctx context.Context, tx *sql.Tx, g *graph.Graph) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM relations`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM nodes`); err != nil {
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO nodes(id,type,name,lifecycle,last_confirmed,model_version,source,evidence_kind,certainty,candidate_type,review_status,review_questions) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, n.ID, n.Type, n.Name, n.Lifecycle, n.LastConfirmed, n.ModelVersion, n.Source, n.EvidenceKind, n.Certainty, n.CandidateType, n.ReviewStatus, questions); err != nil {
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
