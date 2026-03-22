package openapi

import "testing"

func TestToKebabCase(t *testing.T) {
	testCases := map[string]string{
		"getApiCatalog":            "get-api-catalog",
		"searchShiftLeftEvents":    "search-shift-left-events",
		"Get Schema By Label":      "get-schema-by-label",
		" list-threat-detections ": "list-threat-detections",
	}

	for input, expected := range testCases {
		if got := toKebabCase(input); got != expected {
			t.Fatalf("toKebabCase(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestResolveParameterCloudAccountTagsUsesCSVEncoding(t *testing.T) {
	parameter, err := resolveParameter(rawParameter{
		Name: "cloud-account-tags",
		In:   "query",
		Schema: rawSchema{
			Type: "array",
			Items: &rawSchema{
				Type: "string",
			},
		},
	}, rawComponents{})
	if err != nil {
		t.Fatalf("resolveParameter returned error: %v", err)
	}

	if parameter.Type != ParamArray {
		t.Fatalf("expected array parameter, got %s", parameter.Type)
	}
	if parameter.ArrayFormat != ArrayEncodingCSV {
		t.Fatalf("expected CSV array encoding, got %s", parameter.ArrayFormat)
	}
}

func TestLoadCatalogSearchAssetsOperation(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog returned error: %v", err)
	}

	operation, ok := findOperation(catalog, "v2", "inventory", "search-assets")
	if !ok {
		t.Fatal("expected to find v2 inventory search-assets")
	}

	if operation.Method != "POST" {
		t.Fatalf("unexpected method: %s", operation.Method)
	}
	if !operation.HasJSONBody {
		t.Fatal("expected search-assets to accept a JSON body")
	}
	if operation.Pagination != PaginationV2Cursor {
		t.Fatalf("unexpected pagination style: %s", operation.Pagination)
	}
}

func TestCatalogPreferredTagsPrefersV2ForDuplicateCommand(t *testing.T) {
	catalog := Catalog{
		Versions: []VersionGroup{
			{
				Name: "v1",
				Tags: []TagGroup{
					{
						Name: "events",
						Operations: []Operation{
							{Version: "v1", Tag: "events", CommandName: "search-shift-left-events", Path: "/v1/search", Method: "POST"},
							{Version: "v1", Tag: "events", CommandName: "list-events", Path: "/v1/list", Method: "GET"},
						},
					},
				},
			},
			{
				Name: "v2",
				Tags: []TagGroup{
					{
						Name: "events",
						Operations: []Operation{
							{Version: "v2", Tag: "events", CommandName: "search-shift-left-events", Path: "/v2/search", Method: "POST"},
						},
					},
				},
			},
		},
	}

	tags := catalog.PreferredTags()
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if len(tags[0].Operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(tags[0].Operations))
	}

	var preferred Operation
	for _, operation := range tags[0].Operations {
		if operation.CommandName == "search-shift-left-events" {
			preferred = operation
			break
		}
	}

	if preferred.Version != "v2" {
		t.Fatalf("expected v2 operation to be preferred, got %s", preferred.Version)
	}
}

func findOperation(catalog *Catalog, versionName, tagName, commandName string) (Operation, bool) {
	for _, version := range catalog.Versions {
		if version.Name != versionName {
			continue
		}
		for _, tag := range version.Tags {
			if tag.Name != tagName {
				continue
			}
			for _, operation := range tag.Operations {
				if operation.CommandName == commandName {
					return operation, true
				}
			}
		}
	}
	return Operation{}, false
}
