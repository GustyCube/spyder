package format

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gustycube/spyder/internal/types"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatJSON    OutputFormat = "json"
	FormatJSONL   OutputFormat = "jsonl"
	FormatCSV     OutputFormat = "csv"
	FormatParquet OutputFormat = "parquet" // Requires additional implementation
)

// Formatter interface for different output formats
type Formatter interface {
	Format(batch *types.Batch) ([]byte, error)
	FormatStream(batch *types.Batch, w io.Writer) error
}

// JSONFormatter formats output as JSON
type JSONFormatter struct {
	Indent bool
}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter(indent bool) *JSONFormatter {
	return &JSONFormatter{Indent: indent}
}

// Format formats a batch as JSON bytes
func (f *JSONFormatter) Format(batch *types.Batch) ([]byte, error) {
	if f.Indent {
		return json.MarshalIndent(batch, "", "  ")
	}
	return json.Marshal(batch)
}

// FormatStream writes JSON to a stream
func (f *JSONFormatter) FormatStream(batch *types.Batch, w io.Writer) error {
	encoder := json.NewEncoder(w)
	if f.Indent {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(batch)
}

// JSONLFormatter formats output as JSON Lines (JSONL)
type JSONLFormatter struct{}

// NewJSONLFormatter creates a new JSONL formatter
func NewJSONLFormatter() *JSONLFormatter {
	return &JSONLFormatter{}
}

// Format formats a batch as JSONL bytes
func (f *JSONLFormatter) Format(batch *types.Batch) ([]byte, error) {
	var result []byte
	
	// Write each edge as a separate JSON line
	for _, edge := range batch.Edges {
		line := map[string]interface{}{
			"type":        "edge",
			"batch_id":    batch.BatchID,
			"timestamp":   batch.Timestamp,
			"probe_id":    batch.ProbeID,
			"run_id":      batch.RunID,
			"edge":        edge,
		}
		data, err := json.Marshal(line)
		if err != nil {
			return nil, err
		}
		result = append(result, data...)
		result = append(result, '\n')
	}
	
	// Write domain nodes
	for _, node := range batch.NodesDomain {
		line := map[string]interface{}{
			"type":        "node_domain",
			"batch_id":    batch.BatchID,
			"timestamp":   batch.Timestamp,
			"probe_id":    batch.ProbeID,
			"run_id":      batch.RunID,
			"node":        node,
		}
		data, err := json.Marshal(line)
		if err != nil {
			return nil, err
		}
		result = append(result, data...)
		result = append(result, '\n')
	}
	
	// Write IP nodes
	for _, node := range batch.NodesIP {
		line := map[string]interface{}{
			"type":        "node_ip",
			"batch_id":    batch.BatchID,
			"timestamp":   batch.Timestamp,
			"probe_id":    batch.ProbeID,
			"run_id":      batch.RunID,
			"node":        node,
		}
		data, err := json.Marshal(line)
		if err != nil {
			return nil, err
		}
		result = append(result, data...)
		result = append(result, '\n')
	}
	
	// Write cert nodes
	for _, node := range batch.NodesCert {
		line := map[string]interface{}{
			"type":        "node_cert",
			"batch_id":    batch.BatchID,
			"timestamp":   batch.Timestamp,
			"probe_id":    batch.ProbeID,
			"run_id":      batch.RunID,
			"node":        node,
		}
		data, err := json.Marshal(line)
		if err != nil {
			return nil, err
		}
		result = append(result, data...)
		result = append(result, '\n')
	}
	
	return result, nil
}

// FormatStream writes JSONL to a stream
func (f *JSONLFormatter) FormatStream(batch *types.Batch, w io.Writer) error {
	data, err := f.Format(batch)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// CSVFormatter formats output as CSV
type CSVFormatter struct {
	writer     *csv.Writer
	hasHeader  bool
	edgesOnly  bool
}

// NewCSVFormatter creates a new CSV formatter
func NewCSVFormatter(edgesOnly bool) *CSVFormatter {
	return &CSVFormatter{
		edgesOnly: edgesOnly,
	}
}

// Format formats a batch as CSV bytes
func (f *CSVFormatter) Format(batch *types.Batch) ([]byte, error) {
	var result strings.Builder
	w := csv.NewWriter(&result)
	
	// Write header if first time
	if !f.hasHeader {
		if f.edgesOnly {
			w.Write([]string{
				"batch_id", "timestamp", "probe_id", "run_id",
				"edge_type", "source", "target", "observed_at",
			})
		} else {
			w.Write([]string{
				"batch_id", "timestamp", "probe_id", "run_id",
				"record_type", "data1", "data2", "data3", "data4", "data5",
			})
		}
		f.hasHeader = true
	}
	
	// Write edges
	for _, edge := range batch.Edges {
		if f.edgesOnly {
			w.Write([]string{
				batch.BatchID,
				batch.Timestamp.Format(time.RFC3339),
				batch.ProbeID,
				batch.RunID,
				edge.Type,
				edge.Source,
				edge.Target,
				edge.ObservedAt.Format(time.RFC3339),
			})
		} else {
			w.Write([]string{
				batch.BatchID,
				batch.Timestamp.Format(time.RFC3339),
				batch.ProbeID,
				batch.RunID,
				"edge",
				edge.Type,
				edge.Source,
				edge.Target,
				edge.ObservedAt.Format(time.RFC3339),
				"",
			})
		}
	}
	
	if !f.edgesOnly {
		// Write domain nodes
		for _, node := range batch.NodesDomain {
			w.Write([]string{
				batch.BatchID,
				batch.Timestamp.Format(time.RFC3339),
				batch.ProbeID,
				batch.RunID,
				"node_domain",
				node.Host,
				node.Apex,
				node.FirstSeen.Format(time.RFC3339),
				node.LastSeen.Format(time.RFC3339),
				"",
			})
		}
		
		// Write IP nodes
		for _, node := range batch.NodesIP {
			w.Write([]string{
				batch.BatchID,
				batch.Timestamp.Format(time.RFC3339),
				batch.ProbeID,
				batch.RunID,
				"node_ip",
				node.IP,
				node.FirstSeen.Format(time.RFC3339),
				node.LastSeen.Format(time.RFC3339),
				"",
				"",
			})
		}
		
		// Write cert nodes
		for _, node := range batch.NodesCert {
			w.Write([]string{
				batch.BatchID,
				batch.Timestamp.Format(time.RFC3339),
				batch.ProbeID,
				batch.RunID,
				"node_cert",
				node.SPKI,
				node.SubjectCN,
				node.IssuerCN,
				node.NotBefore.Format(time.RFC3339),
				node.NotAfter.Format(time.RFC3339),
			})
		}
	}
	
	w.Flush()
	return []byte(result.String()), nil
}

// FormatStream writes CSV to a stream
func (f *CSVFormatter) FormatStream(batch *types.Batch, w io.Writer) error {
	if f.writer == nil {
		f.writer = csv.NewWriter(w)
	}
	
	// Write header if first time
	if !f.hasHeader {
		if f.edgesOnly {
			f.writer.Write([]string{
				"batch_id", "timestamp", "probe_id", "run_id",
				"edge_type", "source", "target", "observed_at",
			})
		} else {
			f.writer.Write([]string{
				"batch_id", "timestamp", "probe_id", "run_id",
				"record_type", "data1", "data2", "data3", "data4", "data5",
			})
		}
		f.hasHeader = true
	}
	
	// Write data using the same logic as Format
	data, err := f.Format(batch)
	if err != nil {
		return err
	}
	
	_, err = w.Write(data)
	return err
}

// GetFormatter returns a formatter for the specified format
func GetFormatter(format OutputFormat, options map[string]interface{}) (Formatter, error) {
	switch format {
	case FormatJSON:
		indent := false
		if v, ok := options["indent"].(bool); ok {
			indent = v
		}
		return NewJSONFormatter(indent), nil
		
	case FormatJSONL:
		return NewJSONLFormatter(), nil
		
	case FormatCSV:
		edgesOnly := false
		if v, ok := options["edges_only"].(bool); ok {
			edgesOnly = v
		}
		return NewCSVFormatter(edgesOnly), nil
		
	case FormatParquet:
		return nil, fmt.Errorf("parquet format not yet implemented")
		
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// ParseFormat parses a format string
func ParseFormat(s string) (OutputFormat, error) {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON, nil
	case "jsonl", "ndjson":
		return FormatJSONL, nil
	case "csv":
		return FormatCSV, nil
	case "parquet":
		return FormatParquet, nil
	default:
		return "", fmt.Errorf("unknown format: %s", s)
	}
}