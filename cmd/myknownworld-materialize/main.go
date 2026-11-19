// myknownworld-materialize exports a read-only snapshot from the public HTTP API.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/QuietSugar/my-known-world/internal/materialize"
)

type items[T any] struct {
	Items []T `json:"items"`
}

func main() {
	api := flag.String("api", "http://127.0.0.1:8080", "MyKnownWorld read API base URL")
	out := flag.String("out", "", "explicit output directory")
	token := flag.String("token", "", "optional read API bearer token")
	source := flag.String("source", "", "optional exact source filter")
	types := flag.String("types", "", "comma-separated exact node types; default exports all")
	overwrite := flag.Bool("overwrite", false, "explicitly replace generated files in the destination")
	flag.Parse()
	if strings.TrimSpace(*out) == "" {
		fmt.Fprintln(os.Stderr, "--out is required")
		os.Exit(2)
	}
	base, err := filepath.Abs(*out)
	if err != nil {
		fail(err)
	}
	if filepath.Clean(base) == string(filepath.Separator) {
		fail(fmt.Errorf("refusing filesystem root as output directory"))
	}
	client := &http.Client{Timeout: 30 * time.Second}
	g := graph.New()
	var nodeResponse items[graph.Node]
	if err := getJSON(client, *api, *token, "/api/nodes", &nodeResponse); err != nil {
		fail(err)
	}
	for _, n := range nodeResponse.Items {
		if err := g.AddNode(n); err != nil {
			fail(fmt.Errorf("invalid node in read API response: %w", err))
		}
	}
	var relationResponse items[graph.Relation]
	if err := getJSON(client, *api, *token, "/api/relations", &relationResponse); err != nil {
		fail(err)
	}
	for _, r := range relationResponse.Items {
		if err := g.AddRelation(r); err != nil {
			fail(fmt.Errorf("invalid relation in read API response: %w", err))
		}
	}
	selected := map[graph.NodeType]bool{}
	for _, raw := range strings.Split(*types, ",") {
		if raw = strings.TrimSpace(raw); raw != "" {
			selected[graph.NodeType(raw)] = true
		}
	}
	manifest, report, err := materialize.Export(base, g, materialize.Options{Overwrite: *overwrite, Source: *source, Types: selected})
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "materialized %d nodes, %d relations; %d unsupported-data notes\n", len(manifest.Nodes), len(manifest.Relations), len(report.Unsupported))
}
func getJSON(client *http.Client, base, token, route string, target any) error {
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+route, nil)
	if err != nil {
		return err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("read API %s returned HTTP %d: %s", route, response.StatusCode, strings.TrimSpace(string(data)))
	}
	return json.NewDecoder(io.LimitReader(response.Body, 64<<20)).Decode(target)
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
