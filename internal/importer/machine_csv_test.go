package importer

import (
	"github.com/QuietSugar/my-known-world/internal/graph"
	"strings"
	"testing"
)

func TestMachineCSVExplicitAllowlistAndExclusion(t *testing.T) {
	input := "id,name,hostname,os,status,ip,notes,password\nserver-1,Example,host.example,Linux,active,192.0.2.1,private note,not imported\nserver-1,Duplicate,dup.example,Linux,,,x,y\nserver-2,Bad IP,bad.example,Linux,active,not-an-ip,x,y\n,Missing ID,none.example,Linux,active,,x,y\n"
	got, err := MachineCSV(strings.NewReader(input), "private-inventory")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Batch.Nodes) != 1 || got.Batch.Nodes[0].Type != graph.Machine || got.Batch.Nodes[0].Machine.Hostname != "host.example" {
		t.Fatalf("bad imported records: %+v", got.Batch)
	}
	if len(got.Exclusions) != 3 {
		t.Fatalf("expected three excluded rows: %+v", got.Exclusions)
	}
	raw := strings.ToLower(strings.TrimSpace(got.Batch.Nodes[0].Name + got.Batch.Nodes[0].Machine.Hostname))
	if strings.Contains(raw, "private note") || strings.Contains(raw, "not imported") {
		t.Fatal("unexpected private/free-form values")
	}
}
func TestMachineCSVRequiresSchemaAndSource(t *testing.T) {
	if _, err := MachineCSV(strings.NewReader("id,name\na,b\n"), "x"); err == nil {
		t.Fatal("missing hostname schema accepted")
	}
	if _, err := MachineCSV(strings.NewReader("id,name,hostname\na,b,c\n"), ""); err == nil {
		t.Fatal("empty source accepted")
	}
}
