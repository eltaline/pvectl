package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eltaline/pvectl/config"
)

// sampleContentJSON is a Proxmox-style response for GET /nodes/{node}/storage/{storage}/content.
var sampleContentJSON = `{
  "data": [
    {
      "volid": "local:iso/debian-12.iso",
      "content": "iso",
      "size": 3909091328,
      "ctime": 1700000000,
      "format": "iso"
    },
    {
      "volid": "local:backup/vzdump-qemu-100-2024_01_01-00_00_00.vma.zst",
      "content": "backup",
      "size": 1073741824,
      "ctime": 1704067200,
      "vmid": 100,
      "format": "vma.zst",
      "notes": "Daily backup of web server"
    },
    {
      "volid": "local:iso/alpine-3.19.iso",
      "content": "iso",
      "size": 209715200,
      "ctime": 1702000000,
      "format": "iso"
    }
  ]
}`

func setupMockServer(t *testing.T, response string) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, response)
	}))

	// Write temp config pointing to the mock server.
	// Strip https:// prefix to get host:port.
	addr := strings.TrimPrefix(srv.URL, "https://")
	parts := strings.SplitN(addr, ":", 2)
	host := parts[0]
	port := "8006"
	if len(parts) > 1 {
		port = parts[1]
	}

	cfgYAML := fmt.Sprintf(`clusters:
  - name: test
    cluster:
      server: %s
      port: %s
      insecure: true
users:
  - name: test
    user:
      token-id: root@pam!test
      token-secret: test-secret
contexts:
  - name: test
    context:
      cluster: test
      user: test
current-context: test
`, host, port)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0644); err != nil {
		t.Fatal(err)
	}
	return srv, cfgPath
}

func TestSortVolumes(t *testing.T) {
	volumes := []storageVolume{
		{Volid: "c", Size: 300, CTime: 1, Content: "iso", VMID: 3},
		{Volid: "a", Size: 100, CTime: 3, Content: "backup", VMID: 1},
		{Volid: "b", Size: 200, CTime: 2, Content: "images", VMID: 2},
	}

	tests := []struct {
		field    string
		expected []string // expected volid order
	}{
		{"volid", []string{"a", "b", "c"}},
		{"size", []string{"a", "b", "c"}},
		{"date", []string{"c", "b", "a"}},
		{"content", []string{"a", "b", "c"}},
		{"vmid", []string{"a", "b", "c"}},
	}

	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			vols := make([]storageVolume, len(volumes))
			copy(vols, volumes)
			sortVolumes(vols, tc.field)
			for i, expected := range tc.expected {
				if vols[i].Volid != expected {
					t.Errorf("position %d: expected volid %q, got %q", i, expected, vols[i].Volid)
				}
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		ts       int64
		expected string
	}{
		{0, ""},
		{1700000000, "2023-11-14 22:13"},
	}
	for _, tc := range tests {
		got := formatTimestamp(tc.ts)
		if got != tc.expected {
			t.Errorf("formatTimestamp(%d) = %q, want %q", tc.ts, got, tc.expected)
		}
	}
}

func TestContentRow_Printable(t *testing.T) {
	row := contentRow{vol: storageVolume{
		Volid:   "local:iso/test.iso",
		Content: "iso",
		Size:    1073741824,
		CTime:   1700000000,
		VMID:    0,
		Format:  "iso",
	}}

	headers := row.TableHeaders()
	if len(headers) != 5 {
		t.Errorf("expected 5 table headers, got %d", len(headers))
	}

	cols := row.TableRow()
	if len(cols) != 5 {
		t.Errorf("expected 5 table columns, got %d", len(cols))
	}
	if cols[0] != "local:iso/test.iso" {
		t.Errorf("expected volid in first column, got %q", cols[0])
	}
	if cols[2] != "1.0Gi" {
		t.Errorf("expected 1.0Gi for size, got %q", cols[2])
	}
	// VMID=0 should be empty
	if cols[4] != "" {
		t.Errorf("expected empty vmid for 0, got %q", cols[4])
	}

	wideHeaders := row.WideHeaders()
	if len(wideHeaders) != 7 {
		t.Errorf("expected 7 wide headers, got %d", len(wideHeaders))
	}

	wideCols := row.WideRow()
	if len(wideCols) != 7 {
		t.Errorf("expected 7 wide columns, got %d", len(wideCols))
	}
	if wideCols[5] != "iso" {
		t.Errorf("expected format in wide column 5, got %q", wideCols[5])
	}
}

func TestContentRow_WideNotesTruncation(t *testing.T) {
	longNotes := strings.Repeat("a", 100)
	row := contentRow{vol: storageVolume{
		Volid: "test:vol", Notes: longNotes,
	}}
	wideCols := row.WideRow()
	if len(wideCols[6]) > 50 {
		t.Errorf("notes should be truncated to 50 chars, got %d", len(wideCols[6]))
	}
	if !strings.HasSuffix(wideCols[6], "...") {
		t.Error("truncated notes should end with ...")
	}
}

func TestRunStorageContent_tableOutput(t *testing.T) {
	srv, cfgPath := setupMockServer(t, sampleContentJSON)
	defer srv.Close()

	oldCfg := CfgOverrides
	oldFmt := OutputFormat
	oldNode := storageContentNode
	oldSort := storageContentSort
	oldContent := storageContentType
	CfgOverrides = config.Overrides{ConfigPath: cfgPath, Insecure: true}
	OutputFormat = "table"
	storageContentNode = "pve1"
	storageContentSort = "volid"
	storageContentType = ""
	defer func() {
		CfgOverrides = oldCfg
		OutputFormat = oldFmt
		storageContentNode = oldNode
		storageContentSort = oldSort
		storageContentType = oldContent
	}()

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := runStorageContent(storageContentCmd, []string{"local"})
	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runStorageContent error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	// Should contain volume IDs
	if !strings.Contains(out, "local:iso/debian-12.iso") {
		t.Error("expected debian iso in output")
	}
	if !strings.Contains(out, "local:backup/vzdump") {
		t.Error("expected backup in output")
	}
	// Should contain headers
	if !strings.Contains(out, "VOLUME ID") {
		t.Error("expected VOLUME ID header")
	}
}

func TestRunStorageContent_jsonOutput(t *testing.T) {
	srv, cfgPath := setupMockServer(t, sampleContentJSON)
	defer srv.Close()

	oldCfg := CfgOverrides
	oldFmt := OutputFormat
	oldNode := storageContentNode
	oldSort := storageContentSort
	oldContent := storageContentType
	CfgOverrides = config.Overrides{ConfigPath: cfgPath, Insecure: true}
	OutputFormat = "json"
	storageContentNode = "pve1"
	storageContentSort = "volid"
	storageContentType = ""
	defer func() {
		CfgOverrides = oldCfg
		OutputFormat = oldFmt
		storageContentNode = oldNode
		storageContentSort = oldSort
		storageContentType = oldContent
	}()

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := runStorageContent(storageContentCmd, []string{"local"})
	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runStorageContent error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var volumes []storageVolume
	if err := json.Unmarshal([]byte(buf.String()), &volumes); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(volumes) != 3 {
		t.Errorf("expected 3 volumes in JSON, got %d", len(volumes))
	}
}
