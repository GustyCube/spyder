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
	FormatJSON  Format = "json"
	FormatJSONL Format = "jsonl"
	FormatCSV   Format = "csv"
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
