package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eltaline/pvectl/api"
	"github.com/eltaline/pvectl/config"
	"github.com/eltaline/pvectl/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage Proxmox VE storage",
}

// --- storage content ---

var (
	storageContentNode    string
	storageContentType    string
	storageContentSort    string
)

var storageContentCmd = &cobra.Command{
	Use:   "content <storage>",
	Short: "List contents of a storage (images, backups, ISOs, templates, snippets)",
	Long: `List the contents of a Proxmox VE storage with size, date, and owner information.

Requires --node to specify which node to query. Use --content to filter by
content type (images, backup, iso, vztmpl, snippets, rootdir) and --sort to
control sort order.`,
	Args: cobra.ExactArgs(1),
	RunE: runStorageContent,
}

func init() {
	storageContentCmd.Flags().StringVar(&storageContentNode, "node", "", "node to query (required)")
	storageContentCmd.MarkFlagRequired("node")
	storageContentCmd.Flags().StringVar(&storageContentType, "content", "", "filter by content type (images, backup, iso, vztmpl, snippets, rootdir)")
	storageContentCmd.Flags().StringVar(&storageContentSort, "sort", "volid", "sort by field: volid, size, date, content, vmid")
	storageCmd.AddCommand(storageContentCmd)
	rootCmd.AddCommand(storageCmd)
}

// storageVolume represents a single volume from the Proxmox storage content API.
type storageVolume struct {
	Volid        string `json:"volid" yaml:"volid"`
	Content      string `json:"content" yaml:"content"`
	Size         uint64 `json:"size" yaml:"size"`
	CTime        int64  `json:"ctime" yaml:"ctime"`
	Format       string `json:"format,omitempty" yaml:"format,omitempty"`
	VMID         int    `json:"vmid,omitempty" yaml:"vmid,omitempty"`
	Notes        string `json:"notes,omitempty" yaml:"notes,omitempty"`
	Verification string `json:"verification,omitempty" yaml:"verification,omitempty"`
}

// contentRow implements output.Printable for table/wide display.
type contentRow struct {
	vol storageVolume
}

func (r contentRow) TableHeaders() []string {
	return []string{"Volume ID", "Content", "Size", "Date", "VMID"}
}

func (r contentRow) TableRow() []string {
	vmid := ""
	if r.vol.VMID > 0 {
		vmid = strconv.Itoa(r.vol.VMID)
	}
	return []string{
		r.vol.Volid,
		r.vol.Content,
		output.FormatBytes(r.vol.Size),
		formatTimestamp(r.vol.CTime),
		vmid,
	}
}

func (r contentRow) WideHeaders() []string {
	return []string{"Volume ID", "Content", "Size", "Date", "VMID", "Format", "Notes"}
}

func (r contentRow) WideRow() []string {
	vmid := ""
	if r.vol.VMID > 0 {
		vmid = strconv.Itoa(r.vol.VMID)
	}
	notes := r.vol.Notes
	if len(notes) > 50 {
		notes = notes[:47] + "..."
	}
	return []string{
		r.vol.Volid,
		r.vol.Content,
		output.FormatBytes(r.vol.Size),
		formatTimestamp(r.vol.CTime),
		vmid,
		r.vol.Format,
		notes,
	}
}

func formatTimestamp(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04")
}

func createAPIClient(rc *config.ResolvedConfig) (*api.Client, error) {
	if rc.Cluster.Server == "" {
		return nil, fmt.Errorf("no cluster server configured; set a cluster in config or use --cluster")
	}

	port := rc.Cluster.Port
	if port == 0 {
		port = 8006
	}
	baseURL := fmt.Sprintf("https://%s:%d", rc.Cluster.Server, port)

	opts := []api.ClientOption{
		api.WithInsecure(rc.Cluster.Insecure || CfgOverrides.Insecure),
		api.WithVerbosity(api.Verbosity(CfgOverrides.Verbosity)),
	}
	caFile := rc.Cluster.CAFile
	if CfgOverrides.CAFile != "" {
		caFile = CfgOverrides.CAFile
	}
	if caFile != "" {
		opts = append(opts, api.WithCAFile(caFile))
	}

	// Set up authentication.
	if rc.User.TokenID != "" && rc.User.TokenSecret != "" {
		opts = append(opts, api.WithTokenAuth(rc.User.TokenID, rc.User.TokenSecret))
	} else if rc.User.Ticket != "" {
		opts = append(opts, api.WithTicketAuth(rc.User.Ticket, rc.User.CSRFToken))
	}

	return api.NewClient(baseURL, opts...)
}

func runStorageContent(cmd *cobra.Command, args []string) error {
	storageName := args[0]

	rc, err := config.Resolve(CfgOverrides)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	client, err := createAPIClient(rc)
	if err != nil {
		return fmt.Errorf("creating API client: %w", err)
	}

	path := fmt.Sprintf("/nodes/%s/storage/%s/content", storageContentNode, storageName)
	if storageContentType != "" {
		path += "?content=" + storageContentType
	}

	data, err := client.Get(path)
	if err != nil {
		return fmt.Errorf("fetching storage content: %w", err)
	}

	var volumes []storageVolume
	if err := json.Unmarshal(data, &volumes); err != nil {
		return fmt.Errorf("decoding storage content: %w", err)
	}

	sortVolumes(volumes, storageContentSort)

	opts, err := ResolveOutputOptions(os.Stdout)
	if err != nil {
		return err
	}

	// For JSON/YAML, output the raw volume structs for full data.
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return printRawData(volumes, opts)
	}

	items := make([]output.Printable, len(volumes))
	for i := range volumes {
		items[i] = contentRow{vol: volumes[i]}
	}
	return output.Print(items, opts)
}

func printRawData(v interface{}, opts output.Options) error {
	switch opts.Format {
	case output.FormatJSON:
		enc := json.NewEncoder(opts.Writer)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case output.FormatYAML:
		data, err := yaml.Marshal(v)
		if err != nil {
			return err
		}
		_, err = opts.Writer.Write(data)
		return err
	}
	return nil
}

func sortVolumes(volumes []storageVolume, field string) {
	switch strings.ToLower(field) {
	case "size":
		sort.Slice(volumes, func(i, j int) bool { return volumes[i].Size < volumes[j].Size })
	case "date", "ctime":
		sort.Slice(volumes, func(i, j int) bool { return volumes[i].CTime < volumes[j].CTime })
	case "content":
		sort.Slice(volumes, func(i, j int) bool { return volumes[i].Content < volumes[j].Content })
	case "vmid":
		sort.Slice(volumes, func(i, j int) bool { return volumes[i].VMID < volumes[j].VMID })
	default: // "volid" or anything else
		sort.Slice(volumes, func(i, j int) bool { return volumes[i].Volid < volumes[j].Volid })
	}
}
