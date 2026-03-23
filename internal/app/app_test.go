package app

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"upwind-cli/internal/buildinfo"
	"upwind-cli/internal/openapi"
)

func TestParseNextLink(t *testing.T) {
	headers := http.Header{}
	headers.Add("Link", `<https://api.upwind.io/v1/organizations/org_123/configuration-findings?page-token=abc&per-page=50>; rel="first", <https://api.upwind.io/v1/organizations/org_123/configuration-findings?page-token=def&per-page=50>; rel="next"`)

	next, ok, err := parseNextLink(headers.Values("Link"))
	if err != nil {
		t.Fatalf("parseNextLink returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected next link to be found")
	}
	if next != "https://api.upwind.io/v1/organizations/org_123/configuration-findings?page-token=def&per-page=50" {
		t.Fatalf("unexpected next link: %s", next)
	}
}

func TestMergePaginatedArray(t *testing.T) {
	left := []any{
		map[string]any{"id": "1"},
	}
	right := []any{
		map[string]any{"id": "2"},
	}

	merged, ok := mergePaginated(left, right).([]any)
	if !ok {
		t.Fatal("expected merged result to be a slice")
	}
	if len(merged) != 2 {
		t.Fatalf("expected 2 items, got %d", len(merged))
	}
}

func TestMergePaginatedItemsEnvelope(t *testing.T) {
	left := map[string]any{
		"items": []any{map[string]any{"id": "1"}},
		"metadata": map[string]any{
			"next_cursor": "abc",
		},
	}
	right := map[string]any{
		"items": []any{map[string]any{"id": "2"}},
		"metadata": map[string]any{
			"next_cursor": nil,
		},
	}

	merged, ok := mergePaginated(left, right).(map[string]any)
	if !ok {
		t.Fatal("expected merged result to be a map")
	}

	items, ok := merged["items"].([]any)
	if !ok {
		t.Fatal("expected merged items to be a slice")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	cursor := extractNextCursor(merged)
	if cursor != "" {
		t.Fatalf("expected final cursor to be empty, got %q", cursor)
	}
}

func TestMergePaginatedResourceFindingsEnvelope(t *testing.T) {
	left := map[string]any{
		"resourceFindings": []any{map[string]any{"id": "1"}},
		"pagination": map[string]any{
			"next_page_token": "abc",
		},
	}
	right := map[string]any{
		"resourceFindings": []any{map[string]any{"id": "2"}},
		"pagination": map[string]any{
			"next_page_token": "",
		},
	}

	merged, ok := mergePaginated(left, right).(map[string]any)
	if !ok {
		t.Fatal("expected merged result to be a map")
	}

	items, ok := merged["resourceFindings"].([]any)
	if !ok {
		t.Fatal("expected merged resourceFindings to be a slice")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	pagination, ok := merged["pagination"].(map[string]any)
	if !ok {
		t.Fatal("expected merged pagination to be a map")
	}
	if got := pagination["next_page_token"]; got != "" {
		t.Fatalf("expected final next_page_token to be empty, got %#v", got)
	}
}

func TestCountItems(t *testing.T) {
	if got := countItems([]any{1, 2, 3}); got != 3 {
		t.Fatalf("expected 3 items, got %d", got)
	}
	if got := countItems(map[string]any{"items": []any{1, 2}}); got != 2 {
		t.Fatalf("expected 2 items, got %d", got)
	}
}

func TestPageSizeFromDefaults(t *testing.T) {
	parameters := []openapi.Parameter{
		{Name: "per-page", FlagName: "per-page", Type: openapi.ParamInteger, Default: 100},
	}

	command := &cobra.Command{}
	command.Flags().Int("per-page", 0, "")

	if size := effectivePageSize(command, parameters); size != 100 {
		t.Fatalf("expected default page size 100, got %d", size)
	}
}

func TestCollectQueryValuesCommaSeparatedArray(t *testing.T) {
	parameters := []openapi.Parameter{
		{
			Name:        "cloud-account-tags",
			FlagName:    "cloud-account-tags",
			Type:        openapi.ParamArray,
			ArrayFormat: openapi.ArrayEncodingCSV,
		},
	}

	command := &cobra.Command{}
	command.Flags().StringSlice("cloud-account-tags", nil, "")
	if err := command.Flags().Set("cloud-account-tags", "environment=prod,project=webapp"); err != nil {
		t.Fatalf("set flag: %v", err)
	}

	values, err := collectQueryValues(command, parameters)
	if err != nil {
		t.Fatalf("collectQueryValues returned error: %v", err)
	}

	if got := values["cloud-account-tags"]; len(got) != 1 || got[0] != "environment=prod,project=webapp" {
		t.Fatalf("unexpected cloud-account-tags encoding: %#v", got)
	}
}

func TestBuildURLPreservesBasePathPrefix(t *testing.T) {
	queryValues := url.Values{}
	queryValues.Set("severity", "HIGH")

	built, err := buildURL(
		"https://example.com/prefix",
		"/v1/organizations/{organization-id}/assets/{asset-id}",
		map[string]string{
			"organization-id": "org_123",
			"asset-id":        "asset/123",
		},
		queryValues,
	)
	if err != nil {
		t.Fatalf("buildURL returned error: %v", err)
	}

	expected := "https://example.com/prefix/v1/organizations/org_123/assets/asset%2F123?severity=HIGH"
	if built != expected {
		t.Fatalf("unexpected URL: %s", built)
	}
}

func TestNewRootCmdBuildsVersionlessTagTree(t *testing.T) {
	rootCmd, err := NewRootCmd()
	if err != nil {
		t.Fatalf("NewRootCmd returned error: %v", err)
	}

	commandNames := make([]string, 0, len(rootCmd.Commands()))
	for _, command := range rootCmd.Commands() {
		commandNames = append(commandNames, command.Name())
	}

	if slices.Contains(commandNames, "v1") {
		t.Fatalf("expected root command tree to omit v1, got %v", commandNames)
	}
	if slices.Contains(commandNames, "v2") {
		t.Fatalf("expected root command tree to omit v2, got %v", commandNames)
	}
	if !slices.Contains(commandNames, "inventory") {
		t.Fatalf("expected merged root command tree to include inventory, got %v", commandNames)
	}
	if !slices.Contains(commandNames, "threats") {
		t.Fatalf("expected merged root command tree to include threats, got %v", commandNames)
	}
}

func TestNewRootCmdVersionCommandUsesBuildInfo(t *testing.T) {
	rootCmd, err := NewRootCmd()
	if err != nil {
		t.Fatalf("NewRootCmd returned error: %v", err)
	}

	if rootCmd.Version != buildinfo.Short() {
		t.Fatalf("expected root version %q, got %q", buildinfo.Short(), rootCmd.Version)
	}

	output := &bytes.Buffer{}
	rootCmd.SetOut(output)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := output.String(); got != buildinfo.Details() {
		t.Fatalf("unexpected version output: %q", got)
	}
}
