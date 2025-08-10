package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
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
func (w *Writer) WriteBatch(batch interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch w.format {
	case FormatJSON:
		encoder := json.NewEncoder(w.w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(batch)

	case FormatJSONL:
		// For JSONL, we need to extract and write individual records
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
func (w *Writer) writeCSV(batch interface{}) error {
	// Write CSV header if first time
	if !w.hasHeader {
		w.csvWriter.Write([]string{
			"timestamp", "type", "source", "target", "observed_at", "probe_id", "run_id",
		})
		w.hasHeader = true
	}

	// Use reflection to extract edges from batch
	// This is a simplified version - in production you'd want proper type checking
	if b, ok := batch.(map[string]interface{}); ok {
		if edges, ok := b["edges"].([]interface{}); ok {
			for _, edge := range edges {
				if e, ok := edge.(map[string]interface{}); ok {
					w.csvWriter.Write([]string{
						time.Now().Format(time.RFC3339),
						getString(e, "type"),
						getString(e, "source"),
						getString(e, "target"),
						getString(e, "observed_at"),
						getString(b, "probe_id"),
						getString(b, "run_id"),
					})
				}
			}
		}
	}

	return w.csvWriter.Error()
}

// getString safely gets a string from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
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