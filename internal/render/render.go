package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

func Write(writer io.Writer, format string, value any) error {
	switch format {
	case "json":
		return writeJSON(writer, value)
	case "table":
		return writeTable(writer, value)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func writeJSON(writer io.Writer, value any) error {
	if value == nil {
		return nil
	}

	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(writer, string(encoded))
	return err
}

func writeTable(writer io.Writer, value any) error {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case []any:
		return writeRows(writer, typed)
	case map[string]any:
		if rows, ok := topLevelRows(typed); ok {
			return writeRows(writer, rows)
		}
		return writeObject(writer, typed)
	default:
		_, err := fmt.Fprintln(writer, value)
		return err
	}
}

func topLevelRows(value map[string]any) ([]any, bool) {
	for _, key := range []string{"items", "resourceFindings"} {
		rows, ok := value[key].([]any)
		if ok {
			return rows, true
		}
	}

	return nil, false
}

func writeRows(writer io.Writer, rows []any) error {
	renderer := table.NewWriter()
	renderer.SetOutputMirror(writer)
	renderer.Style().Options.SeparateRows = false
	renderer.Style().Options.DrawBorder = false

	flattenedRows := make([]map[string]string, 0, len(rows))
	columns := orderedColumns{}
	for _, row := range rows {
		flattened := map[string]string{}
		flatten("", row, flattened)
		if len(flattened) == 0 {
			flattened["value"] = stringify(row)
		}
		flattenedRows = append(flattenedRows, flattened)
		columns.AddKeys(flattened)
	}

	if len(columns.Names) == 0 {
		_, err := fmt.Fprintln(writer)
		return err
	}

	header := make(table.Row, 0, len(columns.Names))
	for _, column := range columns.Names {
		header = append(header, column)
	}
	renderer.AppendHeader(header)

	for _, row := range flattenedRows {
		record := make(table.Row, 0, len(columns.Names))
		for _, column := range columns.Names {
			record = append(record, row[column])
		}
		renderer.AppendRow(record)
	}

	renderer.Render()
	return nil
}

func writeObject(writer io.Writer, object map[string]any) error {
	renderer := table.NewWriter()
	renderer.SetOutputMirror(writer)
	renderer.Style().Options.DrawBorder = false
	renderer.AppendHeader(table.Row{"field", "value"})

	flattened := map[string]string{}
	flatten("", object, flattened)

	keys := make([]string, 0, len(flattened))
	for key := range flattened {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		renderer.AppendRow(table.Row{key, flattened[key]})
	}

	renderer.Render()
	return nil
}

func flatten(prefix string, value any, target map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			nextPrefix := key
			if prefix != "" {
				nextPrefix = prefix + "." + key
			}
			flatten(nextPrefix, typed[key], target)
		}
	case []any:
		target[prefix] = stringify(typed)
	case nil:
		target[prefix] = ""
	default:
		target[prefix] = stringify(typed)
	}
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		encoded, err := json.Marshal(typed)
		if err == nil && string(encoded) != "null" {
			return strings.Trim(string(encoded), "\"")
		}
		return fmt.Sprint(typed)
	}
}

type orderedColumns struct {
	Names []string
	seen  map[string]struct{}
}

func (o *orderedColumns) AddKeys(values map[string]string) {
	if o.seen == nil {
		o.seen = map[string]struct{}{}
	}

	priority := []string{
		"id",
		"name",
		"title",
		"display_name",
		"status",
		"severity",
		"type",
		"category",
		"version",
		"create_time",
		"update_time",
		"created_at",
		"updated_at",
		"first_seen",
		"last_seen",
	}

	for _, key := range priority {
		if _, ok := values[key]; !ok {
			continue
		}
		if _, ok := o.seen[key]; ok {
			continue
		}
		o.seen[key] = struct{}{}
		o.Names = append(o.Names, key)
	}

	remaining := make([]string, 0, len(values))
	for key := range values {
		if _, ok := o.seen[key]; ok {
			continue
		}
		remaining = append(remaining, key)
	}
	sort.Strings(remaining)
	for _, key := range remaining {
		o.seen[key] = struct{}{}
		o.Names = append(o.Names, key)
	}
}
