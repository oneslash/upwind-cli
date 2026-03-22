package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalogRequiresAllVersions(t *testing.T) {
	specDir := t.TempDir()
	writeSpecFile(t, specDir, "openapi-v1.yaml", "v1", "list-v1")

	_, err := loadCatalog(specDir)
	if err == nil {
		t.Fatal("expected loadCatalog to fail when v2 is missing")
	}
	if !strings.Contains(err.Error(), "missing required spec versions: v2") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadCatalogRejectsDuplicateVersions(t *testing.T) {
	specDir := t.TempDir()
	writeSpecFile(t, specDir, "a-openapi-v1.yaml", "v1", "first-v1")
	writeSpecFile(t, specDir, "b-openapi-v1.yaml", "v1", "second-v1")
	writeSpecFile(t, specDir, "openapi-v2.yaml", "v2", "list-v2")

	_, err := loadCatalog(specDir)
	if err == nil {
		t.Fatal("expected loadCatalog to fail for duplicate v1 specs")
	}
	if !strings.Contains(err.Error(), "duplicate v1 spec files") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadCatalogAcceptsOneSpecPerVersion(t *testing.T) {
	specDir := t.TempDir()
	writeSpecFile(t, specDir, "openapi-v1.yaml", "v1", "list-v1")
	writeSpecFile(t, specDir, "openapi-v2.yaml", "v2", "list-v2")

	catalog, err := loadCatalog(specDir)
	if err != nil {
		t.Fatalf("loadCatalog returned error: %v", err)
	}
	if len(catalog.Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(catalog.Versions))
	}
	if catalog.Versions[0].Name != "v1" || catalog.Versions[1].Name != "v2" {
		t.Fatalf("unexpected version order: %+v", catalog.Versions)
	}
}

func writeSpecFile(t *testing.T, dir string, name string, version string, operationID string) {
	t.Helper()

	path := filepath.Join(dir, name)
	contents := fmt.Sprintf(`openapi: 3.0.0
paths:
  /%s/organizations/{organization-id}/resources:
    get:
      operationId: %s
      tags:
        - test
      responses:
        "200": {}
components:
  parameters: {}
`, version, operationID)

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
