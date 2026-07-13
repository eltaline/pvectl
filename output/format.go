package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format represents the output format type.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatWide  Format = "wide"
)

// ParseFormat validates and returns the output format.
func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(s)) {
	case FormatTable, "":
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatYAML:
		return FormatYAML, nil
	case FormatWide:
		return FormatWide, nil
	default:
		return "", fmt.Errorf("unknown output format %q (valid: table, json, yaml, wide)", s)
	}
}

// Printable is implemented by any value that can be rendered in multiple
// output formats. Commands build slices of Printable and pass them to Print.
type Printable interface {
	// TableHeaders returns column names for the default table view.
	TableHeaders() []string
	// TableRow returns column values for the default table view.
	TableRow() []string
	// WideHeaders returns column names for the wide table view.
	// If it returns nil, TableHeaders is used as a fallback.
	WideHeaders() []string
	// WideRow returns column values for the wide table view.
	// If it returns nil, TableRow is used as a fallback.
	WideRow() []string
}

// Options controls output behavior.
type Options struct {
	Format    Format
	NoHeaders bool
	Writer    io.Writer
}

// Print renders a slice of Printable items according to the given options.
func Print(items []Printable, opts Options) error {
	if opts.Writer == nil {
		return fmt.Errorf("output writer is nil")
	}

	switch opts.Format {
	case FormatJSON:
		return printJSON(items, opts.Writer)
	case FormatYAML:
		return printYAML(items, opts.Writer)
	case FormatTable:
		return printTable(items, opts, false)
	case FormatWide:
		return printTable(items, opts, true)
	default:
		return printTable(items, opts, false)
	}
}

func printJSON(items []Printable, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

func printYAML(items []Printable, w io.Writer) error {
	data, err := yaml.Marshal(items)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func printTable(items []Printable, opts Options, wide bool) error {
	if len(items) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(opts.Writer, 0, 4, 2, ' ', 0)

	headers := items[0].TableHeaders()
	if wide {
		if wh := items[0].WideHeaders(); wh != nil {
			headers = wh
		}
	}

	if !opts.NoHeaders {
		for i, h := range headers {
			headers[i] = strings.ToUpper(h)
		}
		fmt.Fprintln(tw, strings.Join(headers, "\t"))
	}

	for _, item := range items {
		row := item.TableRow()
		if wide {
			if wr := item.WideRow(); wr != nil {
				row = wr
			}
		}
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	return tw.Flush()
}
