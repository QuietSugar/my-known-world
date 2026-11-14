// myknownworld-csv-import converts a generic machine inventory CSV into the public SourceBatch protocol.
// It emits JSON to stdout and never connects to a database or reads credentials.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/QuietSugar/my-known-world/internal/importer"
	"io"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "csv-import:", err)
		os.Exit(1)
	}
}
func run(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("myknownworld-csv-import", flag.ContinueOnError)
	flags.SetOutput(errOut)
	input := flags.String("input", "", "inventory CSV path")
	source := flags.String("source", "", "explicit source identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *input == "" {
		return fmt.Errorf("-input is required")
	}
	if strings.TrimSpace(*source) == "" {
		return fmt.Errorf("-source is required")
	}
	f, err := os.Open(*input)
	if err != nil {
		return err
	}
	defer f.Close()
	report, err := importer.MachineCSV(f, *source)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
