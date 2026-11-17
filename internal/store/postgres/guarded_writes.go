package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type sourceMapping struct {
	Source   string `json:"source"`
	SourceID string `json:"sourceId"`
	NodeID   string `json:"nodeId"`
}

type syncRevisionState struct {
	Graph          string                  `json:"graph"`
	Sources        []graph.SyncSourceState `json:"sources"`
	Mappings       []sourceMapping         `json:"mappings"`
	PolicyRevision string                  `json:"policyRevision"`
}

func revisionOf(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func sourcePolicies(ctx context.Context, q queryer) ([]graph.SourcePolicy, error) {
	rows, err := q.QueryContext(ctx, `SELECT source,priority,allowed_node_types,allowed_predicates,allowed_field_paths FROM sync_source_policies ORDER BY source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	policies := []graph.SourcePolicy{}
	for rows.Next() {
		var p graph.SourcePolicy
		var nodeTypes, predicates, fields []byte
		if err := rows.Scan(&p.Source, &p.Priority, &nodeTypes, &predicates, &fields); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(nodeTypes, &p.AllowedNodeTypes); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(predicates, &p.AllowedPredicates); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(fields, &p.AllowedFieldPaths); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func sourcePolicyRevision(ctx context.Context, q queryer) (string, error) {
	rows, err := q.QueryContext(ctx, `SELECT source,priority,allowed_node_types,allowed_predicates,allowed_field_paths,xmin::text FROM sync_source_policies ORDER BY source`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type item struct {
		Policy  graph.SourcePolicy `json:"policy"`
		Version string             `json:"version"`
	}
	items := []item{}
	for rows.Next() {
		var p graph.SourcePolicy
		var nodeTypes, predicates, fields []byte
		var version string
		if err = rows.Scan(&p.Source, &p.Priority, &nodeTypes, &predicates, &fields, &version); err != nil {
			return "", err
		}
		if err = json.Unmarshal(nodeTypes, &p.AllowedNodeTypes); err != nil {
			return "", err
		}
		if err = json.Unmarshal(predicates, &p.AllowedPredicates); err != nil {
			return "", err
		}
		if err = json.Unmarshal(fields, &p.AllowedFieldPaths); err != nil {
			return "", err
		}
		items = append(items, item{p, version})
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	return revisionOf(items)
}

func syncRevision(ctx context.Context, q queryer, g *graph.Graph) (string, error) {
	graphHash, err := g.Fingerprint()
	if err != nil {
		return "", err
	}
	state := syncRevisionState{
		Graph:    graphHash,
		Sources:  []graph.SyncSourceState{},
		Mappings: []sourceMapping{}, PolicyRevision: "",
	}
	rows, err := q.QueryContext(ctx, `SELECT source,source_hash,parser_version,applied_at FROM sync_sources ORDER BY source`)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var item graph.SyncSourceState
		if err = rows.Scan(&item.Source, &item.SourceHash, &item.ParserVersion, &item.AppliedAt); err != nil {
			rows.Close()
			return "", err
		}
		state.Sources = append(state.Sources, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()

	rows, err = q.QueryContext(ctx, `SELECT source,source_id,node_id FROM sync_identity_mappings ORDER BY source,source_id`)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var item sourceMapping
		if err = rows.Scan(&item.Source, &item.SourceID, &item.NodeID); err != nil {
			rows.Close()
			return "", err
		}
		state.Mappings = append(state.Mappings, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()

	state.PolicyRevision, err = sourcePolicyRevision(ctx, q)
	if err != nil {
		return "", err
	}
	return revisionOf(state)
}

func undoPlanRevision(id int64, graphHash, operationHash string) (string, error) {
	return revisionOf(struct {
		OperationID   int64  `json:"operationId"`
		GraphHash     string `json:"graphHash"`
		OperationHash string `json:"operationHash"`
	}{id, graphHash, operationHash})
}

func validateSyncUndoState(ctx context.Context, tx queryer, metadata syncUndoMetadata) error {
	if metadata.AppliedState == nil {
		return errors.New("sync undo metadata lacks the applied source state; operation is audit-only")
	}
	var state graph.SyncSourceState
	if err := tx.QueryRowContext(ctx, `SELECT source,source_hash,parser_version,applied_at FROM sync_sources WHERE source=$1`, metadata.Source).Scan(&state.Source, &state.SourceHash, &state.ParserVersion, &state.AppliedAt); err != nil {
		return errors.New("sync source state changed since the operation; undo rejected")
	}
	mappings, err := loadIdentityMappings(ctx, tx, metadata.Source)
	if err != nil {
		return err
	}
	expected := metadata.AppliedState
	if state.Source != expected.Source || state.SourceHash != expected.SourceHash || state.ParserVersion != expected.ParserVersion || !state.AppliedAt.Equal(expected.AppliedAt) || !reflect.DeepEqual(mappings, metadata.AppliedMappings) {
		return errors.New("sync source state or identity mappings changed since the operation; undo rejected")
	}
	return nil
}

func (s *Store) PreviewUndoOperation(ctx context.Context, id int64) (graph.MutationPlan, error) {
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
	graphHash, err := current.Fingerprint()
	if err != nil {
		return graph.MutationPlan{}, err
	}
	var afterHash, action string
	var before, metadataJSON []byte
	if err = tx.QueryRowContext(ctx, `SELECT before_state,after_hash,action,metadata FROM graph_mutation_operations WHERE operation_id=$1`, id).Scan(&before, &afterHash, &action, &metadataJSON); errors.Is(err, sql.ErrNoRows) {
		return graph.MutationPlan{}, fmt.Errorf("operation %d not found", id)
	} else if err != nil {
		return graph.MutationPlan{}, err
	}
	if strings.HasPrefix(action, "sync:") {
		var meta syncUndoMetadata
		if err = json.Unmarshal(metadataJSON, &meta); err != nil {
			return graph.MutationPlan{}, err
		}
		if err = validateSyncUndoState(ctx, tx, meta); err != nil {
			return graph.MutationPlan{}, err
		}
	}
	if string(before) == `{}` {
		return graph.MutationPlan{}, errors.New("undo snapshot expired; operation is audit-only")
	}
	var undone bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM graph_operation_undos WHERE operation_id=$1)`, id).Scan(&undone); err != nil {
		return graph.MutationPlan{}, err
	}
	if undone {
		return graph.MutationPlan{}, fmt.Errorf("operation %d was already undone", id)
	}
	if graphHash != afterHash {
		return graph.MutationPlan{}, errors.New("undo rejected: graph changed after this operation")
	}
	revision, err := undoPlanRevision(id, graphHash, afterHash)
	if err != nil {
		return graph.MutationPlan{}, err
	}
	return graph.MutationPlan{ExpectedRevision: revision}, tx.Commit()
}

func (s *Store) PreviewSourcePolicy(ctx context.Context, policy graph.SourcePolicy) (graph.MutationPlan, *graph.SourcePolicy, error) {
	if err := policy.Validate(); err != nil {
		return graph.MutationPlan{}, nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return graph.MutationPlan{}, nil, err
	}
	defer tx.Rollback()
	policies, err := sourcePolicies(ctx, tx)
	if err != nil {
		return graph.MutationPlan{}, nil, err
	}
	revision, err := sourcePolicyRevision(ctx, tx)
	if err != nil {
		return graph.MutationPlan{}, nil, err
	}
	var previous *graph.SourcePolicy
	for i := range policies {
		if policies[i].Source == policy.Source {
			value := policies[i]
			previous = &value
			break
		}
	}
	return graph.MutationPlan{ExpectedRevision: revision}, previous, tx.Commit()
}

func (s *Store) ApplySourcePolicy(ctx context.Context, policy graph.SourcePolicy, expectedRevision string) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes, relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `LOCK TABLE sync_source_policies IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	revision, err := sourcePolicyRevision(ctx, tx)
	if err != nil {
		return err
	}
	if expectedRevision == "" || revision != expectedRevision {
		return errors.New("source policies changed since preview; preview again before applying")
	}
	nodeTypes, err := json.Marshal(policy.AllowedNodeTypes)
	if err != nil {
		return err
	}
	predicates, err := json.Marshal(policy.AllowedPredicates)
	if err != nil {
		return err
	}
	fields, err := json.Marshal(policy.AllowedFieldPaths)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sync_source_policies(source,priority,allowed_node_types,allowed_predicates,allowed_field_paths) VALUES($1,$2,$3,$4,$5) ON CONFLICT(source) DO UPDATE SET priority=EXCLUDED.priority,allowed_node_types=EXCLUDED.allowed_node_types,allowed_predicates=EXCLUDED.allowed_predicates,allowed_field_paths=EXCLUDED.allowed_field_paths,updated_at=now()`, policy.Source, policy.Priority, nodeTypes, predicates, fields); err != nil {
		return fmt.Errorf("save source policy: %w", err)
	}
	return tx.Commit()
}

func fieldResolutionRevision(ctx context.Context, q queryer, resolution graph.FieldResolution) (string, error) {
	var nodeVersion string
	if err := q.QueryRowContext(ctx, `SELECT xmin::text FROM nodes WHERE id=$1`, resolution.NodeID).Scan(&nodeVersion); err != nil {
		return "", fmt.Errorf("node %q not found", resolution.NodeID)
	}
	assertionRows, err := q.QueryContext(ctx, `SELECT id,xmin::text FROM field_assertions WHERE node_id=$1 AND field_path=$2 ORDER BY id`, resolution.NodeID, resolution.FieldPath)
	if err != nil {
		return "", err
	}
	ids := []struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	}{}
	for assertionRows.Next() {
		var item struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		}
		if err = assertionRows.Scan(&item.ID, &item.Version); err != nil {
			assertionRows.Close()
			return "", err
		}
		ids = append(ids, item)
	}
	if err = assertionRows.Err(); err != nil {
		assertionRows.Close()
		return "", err
	}
	assertionRows.Close()
	var existing []byte
	err = q.QueryRowContext(ctx, `SELECT COALESCE(json_agg(json_build_array(resolution_id,xmin::text,selected_assertion_ids) ORDER BY resolution_id),'[]'::json) FROM field_resolutions WHERE node_id=$1 AND field_path=$2`, resolution.NodeID, resolution.FieldPath).Scan(&existing)
	if err != nil {
		return "", err
	}
	return revisionOf(struct {
		NodeVersion string `json:"nodeVersion"`
		Assertions  []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		} `json:"assertions"`
		Resolutions json.RawMessage `json:"resolutions"`
	}{nodeVersion, ids, existing})
}

func validateResolutionAssertions(ctx context.Context, q queryer, resolution graph.FieldResolution) error {
	for _, id := range resolution.SelectedAssertionIDs {
		var nodeID, field string
		if err := q.QueryRowContext(ctx, `SELECT node_id,field_path FROM field_assertions WHERE id=$1`, id).Scan(&nodeID, &field); err != nil {
			return fmt.Errorf("selected assertion %q not found", id)
		}
		if nodeID != resolution.NodeID || field != resolution.FieldPath {
			return fmt.Errorf("selected assertion %q belongs to another field", id)
		}
	}
	return nil
}

func (s *Store) PreviewFieldResolution(ctx context.Context, resolution graph.FieldResolution) (graph.MutationPlan, error) {
	if err := resolution.Validate(); err != nil {
		return graph.MutationPlan{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return graph.MutationPlan{}, err
	}
	defer tx.Rollback()
	if err = validateResolutionAssertions(ctx, tx, resolution); err != nil {
		return graph.MutationPlan{}, err
	}
	revision, err := fieldResolutionRevision(ctx, tx, resolution)
	if err != nil {
		return graph.MutationPlan{}, err
	}
	return graph.MutationPlan{ExpectedRevision: revision}, tx.Commit()
}

func (s *Store) ApplyFieldResolution(ctx context.Context, resolution graph.FieldResolution, expectedRevision string) (graph.FieldResolution, error) {
	if err := resolution.Validate(); err != nil {
		return resolution, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return resolution, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE nodes, relations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return resolution, err
	}
	if _, err = tx.ExecContext(ctx, `LOCK TABLE field_assertions, field_resolutions IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return resolution, err
	}
	if err = validateResolutionAssertions(ctx, tx, resolution); err != nil {
		return resolution, err
	}
	revision, err := fieldResolutionRevision(ctx, tx, resolution)
	if err != nil {
		return resolution, err
	}
	if expectedRevision == "" || expectedRevision != revision {
		return resolution, errors.New("field assertions or decisions changed since preview; preview again before applying")
	}
	ids, err := json.Marshal(resolution.SelectedAssertionIDs)
	if err != nil {
		return resolution, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO field_resolutions(node_id,field_path,selected_assertion_ids,rationale_reference) VALUES($1,$2,$3,$4) RETURNING resolution_id,recorded_at`, resolution.NodeID, resolution.FieldPath, ids, resolution.RationaleReference).Scan(&resolution.ID, &resolution.RecordedAt)
	if err != nil {
		return resolution, err
	}
	return resolution, tx.Commit()
}
