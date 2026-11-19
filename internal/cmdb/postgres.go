package cmdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	_ "github.com/lib/pq"
)

// PostgresEnvironmentRepository persists application-owned grouping metadata in
// its own table. It does not read or mutate the core graph tables.
type PostgresEnvironmentRepository struct{ DB *sql.DB }

func (r PostgresEnvironmentRepository) EnsureSchema(ctx context.Context) error {
	if r.DB == nil {
		return errors.New("environment database is not configured")
	}
	_, err := r.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cmdb_app_environments (
		id text PRIMARY KEY,
		name text NOT NULL,
		description text NOT NULL DEFAULT '',
		tags jsonb NOT NULL DEFAULT '[]'::jsonb,
		service_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
		account_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
		updated_at timestamptz NOT NULL DEFAULT now()
	)`)
	return err
}
func (r PostgresEnvironmentRepository) List(ctx context.Context) ([]Environment, error) {
	if r.DB == nil {
		return nil, errors.New("environment database is not configured")
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id,name,description,tags,service_ids,account_ids FROM cmdb_app_environments ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Environment{}
	for rows.Next() {
		var e Environment
		var tags, services, accounts []byte
		if err := rows.Scan(&e.ID, &e.Name, &e.Description, &tags, &services, &accounts); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(tags, &e.Tags); err != nil {
			return nil, errors.New("stored environment tags are invalid")
		}
		if err := json.Unmarshal(services, &e.ServiceIDs); err != nil {
			return nil, errors.New("stored environment services are invalid")
		}
		if err := json.Unmarshal(accounts, &e.AccountIDs); err != nil {
			return nil, errors.New("stored environment accounts are invalid")
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
func (r PostgresEnvironmentRepository) Put(ctx context.Context, e Environment) error {
	if r.DB == nil {
		return errors.New("environment database is not configured")
	}
	tags, err := json.Marshal(e.Tags)
	if err != nil {
		return err
	}
	services, err := json.Marshal(e.ServiceIDs)
	if err != nil {
		return err
	}
	accounts, err := json.Marshal(e.AccountIDs)
	if err != nil {
		return err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO cmdb_app_environments(id,name,description,tags,service_ids,account_ids,updated_at) VALUES($1,$2,$3,$4::jsonb,$5::jsonb,$6::jsonb,now()) ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,tags=EXCLUDED.tags,service_ids=EXCLUDED.service_ids,account_ids=EXCLUDED.account_ids,updated_at=now()`, e.ID, e.Name, e.Description, string(tags), string(services), string(accounts))
	return err
}
func (r PostgresEnvironmentRepository) Delete(ctx context.Context, id string) error {
	if r.DB == nil {
		return errors.New("environment database is not configured")
	}
	_, err := r.DB.ExecContext(ctx, `DELETE FROM cmdb_app_environments WHERE id=$1`, id)
	return err
}
