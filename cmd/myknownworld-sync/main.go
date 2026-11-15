// Command myknownworld-sync previews or atomically applies a JSON source batch.
// Applying requires an exact source confirmation and the plan hash from a prior preview.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
	pgstore "github.com/QuietSugar/my-known-world/internal/store/postgres"
)

const maxBatchBytes = 8 << 20

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "sync:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("myknownworld-sync", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "", "JSON source batch file")
	apply := flags.Bool("apply", false, "atomically apply the batch; without this flag only preview")
	confirm := flags.String("confirm-source", "", "must exactly match batch source when -apply is used")
	confirmPlan := flags.String("confirm-plan-hash", "", "must exactly match planHash printed by a prior preview")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *input == "" {
		return errors.New("-input is required")
	}
	f, err := os.Open(*input)
	if err != nil {
		return fmt.Errorf("open input batch: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBatchBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxBatchBytes {
		return fmt.Errorf("batch exceeds %d-byte limit", maxBatchBytes)
	}
	var batch graph.SourceBatch
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&batch); err != nil {
		return fmt.Errorf("decode source batch: %w", err)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return errors.New("input must contain exactly one JSON object")
	}
	if strings.TrimSpace(batch.Source) == "" {
		return errors.New("batch source is required")
	}
	if *apply && *confirm != batch.Source {
		return fmt.Errorf("applying requires -confirm-source %q", batch.Source)
	}
	if !*apply && *confirm != "" {
		return errors.New("-confirm-source is only valid together with -apply")
	}
	if !*apply && *confirmPlan != "" {
		return errors.New("-confirm-plan-hash is only valid together with -apply")
	}
	if *apply && *confirmPlan == "" {
		return errors.New("applying requires -confirm-plan-hash from a prior preview")
	}
	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	store, err := pgstore.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		return err
	}
	plan, err := store.PreviewSync(ctx, batch)
	if err != nil {
		return err
	}
	plan.PlanHash, err = syncPlanConfirmation(plan.ExpectedRevision, batch)
	if err != nil {
		return err
	}
	if !*apply {
		return writeJSON(stdout, plan)
	}
	if *confirmPlan != plan.PlanHash {
		return fmt.Errorf("batch or graph changed since preview; expected -confirm-plan-hash %s", plan.PlanHash)
	}
	preview, err := store.ApplySyncPlan(ctx, batch, plan.ExpectedRevision)
	if err != nil {
		if preview != nil {
			_ = writeJSON(stdout, preview)
		}
		return err
	}
	return writeJSON(stdout, map[string]any{"applied": true, "preview": preview})
}

func syncPlanConfirmation(revision string, batch graph.SourceBatch) (string, error) {
	data, err := json.Marshal(struct {
		Revision string            `json:"revision"`
		Batch    graph.SourceBatch `json:"batch"`
	}{revision, batch})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
