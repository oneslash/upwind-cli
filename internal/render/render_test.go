package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	var buffer bytes.Buffer

	err := Write(&buffer, "json", map[string]any{
		"id":   "asset-1",
		"name": "demo",
	})
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, "\"id\": \"asset-1\"") {
		t.Fatalf("expected JSON output to contain id, got %q", output)
	}
	if !strings.HasSuffix(output, "\n") {
		t.Fatalf("expected trailing newline, got %q", output)
	}
}

func TestWriteTableItemsEnvelope(t *testing.T) {
	var buffer bytes.Buffer

	err := Write(&buffer, "table", map[string]any{
		"items": []any{
			map[string]any{"id": "1", "name": "alpha"},
			map[string]any{"id": "2", "name": "beta"},
		},
		"metadata": map[string]any{"next_cursor": nil},
	})
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	output := strings.ToLower(buffer.String())
	for _, expected := range []string{"id", "name", "alpha", "beta"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected table output to contain %q, got %q", expected, output)
		}
	}
}

func TestWriteTableResourceFindingsEnvelope(t *testing.T) {
	var buffer bytes.Buffer

	err := Write(&buffer, "table", map[string]any{
		"resourceFindings": []any{
			map[string]any{"id": "1", "status": "FAIL"},
			map[string]any{"id": "2", "status": "PASS"},
		},
		"pagination": map[string]any{"next_page_token": ""},
	})
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	output := strings.ToLower(buffer.String())
	for _, expected := range []string{"id", "status", "fail", "pass"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected table output to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "resourcefindings") {
		t.Fatalf("expected row rendering instead of flattened envelope, got %q", output)
	}
}

func TestWriteTableObject(t *testing.T) {
	var buffer bytes.Buffer

	err := Write(&buffer, "table", map[string]any{
		"id": "story-1",
		"metadata": map[string]any{
			"status": "OPEN",
		},
	})
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	output := strings.ToLower(buffer.String())
	for _, expected := range []string{"field", "value", "metadata.status", "OPEN"} {
		if !strings.Contains(output, strings.ToLower(expected)) {
			t.Fatalf("expected table output to contain %q, got %q", expected, output)
		}
	}
}
