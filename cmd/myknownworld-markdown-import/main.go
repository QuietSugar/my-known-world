// myknownworld-markdown-import scans a local Markdown wiki's frontmatter and
// emits one complete read-only SourceBatch as JSON on stdout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/QuietSugar/my-known-world/internal/importer"
	yaml "github.com/goccy/go-yaml"
)

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("myknownworld-markdown-import", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "local wiki root directory (required)")
	configPath := flags.String("config", "", "private YAML source configuration file (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *root == "" || *configPath == "" {
		return fmt.Errorf("--root and --config are required")
	}
	configBytes, err := os.ReadFile(*configPath)
	if err != nil {
		return fmt.Errorf("read source config: %w", err)
	}
	var config importer.MarkdownConfig
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return fmt.Errorf("parse source config: %w", err)
	}
	report, err := importer.MarkdownFrontmatter(*root, config)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
