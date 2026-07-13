package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// testItem implements Printable for testing.
type testItem struct {
	Name   string `json:"name" yaml:"name"`
	Status string `json:"status" yaml:"status"`
	Extra  string `json:"extra" yaml:"extra"`
}

func (t testItem) TableHeaders() []string { return []string{"Name", "Status"} }
func (t testItem) TableRow() []string     { return []string{t.Name, t.Status} }
func (t testItem) WideHeaders() []string  { return []string{"Name", "Status", "Extra"} }
func (t testItem) WideRow() []string      { return []string{t.Name, t.Status, t.Extra} }

func items() []Printable {
	return []Printable{
		testItem{Name: "vm-100", Status: "running", Extra: "node1"},
		testItem{Name: "vm-101", Status: "stopped", Extra: "node2"},
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  Format
		err   bool
	}{
		{"table", FormatTable, false},
		{"json", FormatJSON, false},
		{"yaml", FormatYAML, false},
		{"wide", FormatWide, false},
		{"JSON", FormatJSON, false},
		{"", FormatTable, false},
		{"csv", "", true},
	}
	for _, tt := range tests {
		got, err := ParseFormat(tt.input)
		if tt.err && err == nil {
			t.Errorf("ParseFormat(%q) expected error", tt.input)
		}
		if !tt.err && err != nil {
			t.Errorf("ParseFormat(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPrintTable(t *testing.T) {
	var buf bytes.Buffer
	err := Print(items(), Options{Format: FormatTable, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "NAME") {
		t.Error("expected uppercase header NAME")
	}
	if !strings.Contains(out, "vm-100") {
		t.Error("expected vm-100 in output")
	}
	// Should NOT contain "Extra" column in table mode
	if strings.Contains(out, "EXTRA") {
		t.Error("table mode should not have EXTRA header")
	}
}

func TestPrintTableNoHeaders(t *testing.T) {
	var buf bytes.Buffer
	err := Print(items(), Options{Format: FormatTable, NoHeaders: true, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "NAME") {
		t.Error("--no-headers should omit headers")
	}
	if !strings.Contains(out, "vm-100") {
		t.Error("expected vm-100 in output")
	}
}

func TestPrintWide(t *testing.T) {
	var buf bytes.Buffer
	err := Print(items(), Options{Format: FormatWide, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "EXTRA") {
		t.Error("wide mode should have EXTRA header")
	}
	if !strings.Contains(out, "node1") {
		t.Error("expected node1 in wide output")
	}
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	err := Print(items(), Options{Format: FormatJSON, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	var parsed []testItem
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed) != 2 {
		t.Errorf("expected 2 items, got %d", len(parsed))
	}
}

func TestPrintYAML(t *testing.T) {
	var buf bytes.Buffer
	err := Print(items(), Options{Format: FormatYAML, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "name: vm-100") {
		t.Error("expected YAML output with name field")
	}
}

func TestPrintEmptySlice(t *testing.T) {
	var buf bytes.Buffer
	err := Print([]Printable{}, Options{Format: FormatTable, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Error("empty slice should produce no output for table")
	}
}
