// Package importer contains conservative, explicit source-format adapters.
package importer

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type Exclusion struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}
type MachineCSVReport struct {
	Batch      graph.SourceBatch `json:"batch"`
	Exclusions []Exclusion       `json:"exclusions"`
}

// MachineCSV converts a documented inventory CSV schema into a source batch.
// It never imports credentials, free-form notes, document bodies, or unknown columns.
func MachineCSV(r io.Reader, source string) (MachineCSVReport, error) {
	if strings.TrimSpace(source) == "" {
		return MachineCSVReport{}, errors.New("source is required")
	}
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = false
	header, err := cr.Read()
	if err != nil {
		return MachineCSVReport{}, fmt.Errorf("read CSV header: %w", err)
	}
	index := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if key == "" {
			continue
		}
		if _, exists := index[key]; exists {
			return MachineCSVReport{}, fmt.Errorf("duplicate CSV column %q", key)
		}
		index[key] = i
	}
	for _, required := range []string{"id", "name", "hostname"} {
		if _, ok := index[required]; !ok {
			return MachineCSVReport{}, fmt.Errorf("required CSV column %q is missing", required)
		}
	}
	report := MachineCSVReport{Batch: graph.SourceBatch{Source: source, Nodes: []graph.Node{}, Relations: []graph.Relation{}}, Exclusions: []Exclusion{}}
	seen := map[string]bool{}
	line := 1
	for {
		row, readErr := cr.Read()
		if readErr == io.EOF {
			break
		}
		line++
		if readErr != nil {
			report.Exclusions = append(report.Exclusions, Exclusion{line, "malformed CSV record"})
			continue
		}
		get := func(k string) string {
			i, ok := index[k]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		id, name, host := get("id"), get("name"), get("hostname")
		reason := ""
		switch {
		case id == "":
			reason = "missing id"
		case name == "":
			reason = "missing name"
		case host == "":
			reason = "missing hostname"
		case seen[id]:
			reason = "duplicate id"
		}
		if reason != "" {
			report.Exclusions = append(report.Exclusions, Exclusion{line, reason})
			continue
		}
		seen[id] = true
		details := &graph.MachineDetails{Hostname: host, OS: get("os"), Status: get("status")}
		if rawIP := get("ip"); rawIP != "" {
			ip := net.ParseIP(rawIP)
			if ip == nil {
				report.Exclusions = append(report.Exclusions, Exclusion{line, "invalid IP address"})
				continue
			}
			details.IPs = []string{ip.String()}
		}
		report.Batch.Nodes = append(report.Batch.Nodes, graph.Node{ID: id, Type: graph.Machine, Name: name, ModelVersion: 1, Machine: details})
	}
	return report, nil
}
