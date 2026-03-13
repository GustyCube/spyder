package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/gustycube/spyder/internal/emit"
)

// Format represents the output format
type Format string

const (
	FormatJSON   Format = "json"
	FormatJSONL  Format = "jsonl"
	FormatCSV    Format = "csv"
	FormatPretty Format = "pretty"
)

// Writer handles formatted output
type Writer struct {
	format    Format
	w         io.Writer
	csvWriter *csv.Writer
	mu        sync.Mutex
	hasHeader bool
}

// NewWriter creates a new output writer
func NewWriter(format string, w io.Writer) (*Writer, error) {
	var f Format
	switch strings.ToLower(format) {
	case "json":
		f = FormatJSON
	case "jsonl", "ndjson":
		f = FormatJSONL
	case "csv":
		f = FormatCSV
	case "pretty":
		f = FormatPretty
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	writer := &Writer{
		format: f,
		w:      w,
	}

	if f == FormatCSV {
		writer.csvWriter = csv.NewWriter(w)
	}

	return writer, nil
}

// NewStdoutWriter creates a writer for stdout
func NewStdoutWriter(format string) (*Writer, error) {
	return NewWriter(format, os.Stdout)
}

// WriteBatch writes a batch in the configured format
func (w *Writer) WriteBatch(batch emit.Batch) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch w.format {
	case FormatJSON:
		encoder := json.NewEncoder(w.w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(batch)

	case FormatJSONL:
		data, err := json.Marshal(batch)
		if err != nil {
			return err
		}
		_, err = w.w.Write(data)
		if err != nil {
			return err
		}
		_, err = w.w.Write([]byte("\n"))
		return err

	case FormatCSV:
		return w.writeCSV(batch)

	case FormatPretty:
		return w.writePretty(batch)

	default:
		return fmt.Errorf("unsupported format: %s", w.format)
	}
}

// writeCSV writes batch data as CSV
func (w *Writer) writeCSV(batch emit.Batch) error {
	if !w.hasHeader {
		w.csvWriter.Write([]string{
			"type", "source", "target", "observed_at", "probe_id", "run_id",
		})
		w.hasHeader = true
	}

	for _, e := range batch.Edges {
		w.csvWriter.Write([]string{
			e.Type,
			e.Source,
			e.Target,
			e.ObservedAt.Format("2006-01-02T15:04:05Z"),
			e.ProbeID,
			e.RunID,
		})
	}

	w.csvWriter.Flush()
	return w.csvWriter.Error()
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorMagenta = "\033[35m"
	colorWhite  = "\033[37m"
)

// edgeSymbol returns a colored symbol for each edge type
func edgeSymbol(edgeType string) (string, string) {
	switch edgeType {
	case "RESOLVES_TO":
		return colorGreen + "A" + colorReset, colorDim + "dns" + colorReset
	case "USES_NS":
		return colorBlue + "NS" + colorReset, colorDim + "dns" + colorReset
	case "ALIAS_OF":
		return colorYellow + "CNAME" + colorReset, colorDim + "dns" + colorReset
	case "USES_MX":
		return colorMagenta + "MX" + colorReset, colorDim + "dns" + colorReset
	case "LINKS_TO":
		return colorCyan + "LINK" + colorReset, colorDim + "http" + colorReset
	case "USES_CERT":
		return colorWhite + "TLS" + colorReset, colorDim + "tls" + colorReset
	default:
		return edgeType, ""
	}
}

// writePretty writes a human-readable summary of the batch
func (w *Writer) writePretty(batch emit.Batch) error {
	// Group edges by source domain
	bySource := make(map[string][]emit.Edge)
	var sources []string
	for _, e := range batch.Edges {
		if _, seen := bySource[e.Source]; !seen {
			sources = append(sources, e.Source)
		}
		bySource[e.Source] = append(bySource[e.Source], e)
	}

	for _, src := range sources {
		edges := bySource[src]
		// Domain header
		fmt.Fprintf(w.w, "\n%s%s%s%s\n", colorBold, colorCyan, src, colorReset)

		for _, e := range edges {
			sym, via := edgeSymbol(e.Type)
			if via != "" {
				fmt.Fprintf(w.w, "  %s%-5s%s %s %s\n", colorDim, via, colorReset, sym, e.Target)
			} else {
				fmt.Fprintf(w.w, "        %s %s\n", sym, e.Target)
			}
		}
	}

	// Cert summary if any
	if len(batch.NodesC) > 0 {
		fmt.Fprintf(w.w, "\n  %s%scerts%s\n", colorDim, colorWhite, colorReset)
		for _, c := range batch.NodesC {
			expires := c.NotAfter.Format("2006-01-02")
			fmt.Fprintf(w.w, "    %s%s%s  issuer=%s  expires=%s\n",
				colorWhite, c.SubjectCN, colorReset, c.IssuerCN, expires)
		}
	}

	return nil
}

// Flush flushes any buffered data
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.csvWriter != nil {
		w.csvWriter.Flush()
		return w.csvWriter.Error()
	}
	return nil
}
