package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"upwind-cli/internal/auth"
	"upwind-cli/internal/buildinfo"
	"upwind-cli/internal/config"
	"upwind-cli/internal/openapi"
	"upwind-cli/internal/render"
)

func NewRootCmd() (*cobra.Command, error) {
	if err := config.LoadDotEnv(); err != nil {
		return nil, err
	}

	catalog, err := openapi.LoadCatalog()
	if err != nil {
		return nil, err
	}

	options := &config.Options{
		OrganizationID: os.Getenv(config.EnvOrganizationID),
		Region:         os.Getenv(config.EnvRegion),
		BaseURL:        os.Getenv(config.EnvBaseURL),
		AuthURL:        os.Getenv(config.EnvAuthURL),
		Audience:       os.Getenv(config.EnvAudience),
		ClientID:       os.Getenv(config.EnvClientID),
		ClientSecret:   os.Getenv(config.EnvClientSecret),
		AccessToken:    os.Getenv(config.EnvAccessToken),
		Output:         os.Getenv(config.EnvOutput),
		Timeout:        config.EnvDuration(config.EnvTimeout, 30*time.Second),
	}

	rootCmd := &cobra.Command{
		Use:           "upwind",
		Short:         "CLI client for the Upwind Management APIs",
		Long:          "A Cobra-based Upwind CLI generated from the provided OpenAPI specifications. When the same tag and operation exist in both versions, the CLI prefers the v2 definition.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildinfo.Short(),
	}
	rootCmd.SetVersionTemplate(buildinfo.Details())

	flags := rootCmd.PersistentFlags()
	flags.StringVarP(&options.OrganizationID, "organization-id", "o", options.OrganizationID, fmt.Sprintf("Upwind organization ID (env %s)", config.EnvOrganizationID))
	flags.StringVar(&options.Region, "region", defaultIfEmpty(options.Region, "us"), fmt.Sprintf("Region to target: us, eu, or me (env %s)", config.EnvRegion))
	flags.StringVar(&options.BaseURL, "base-url", options.BaseURL, fmt.Sprintf("Override API base URL (env %s)", config.EnvBaseURL))
	flags.StringVar(&options.AuthURL, "auth-url", options.AuthURL, fmt.Sprintf("Override OAuth base URL (env %s)", config.EnvAuthURL))
	flags.StringVar(&options.Audience, "audience", options.Audience, fmt.Sprintf("OAuth audience override (env %s)", config.EnvAudience))
	flags.StringVar(&options.ClientID, "client-id", options.ClientID, fmt.Sprintf("OAuth client ID (env %s)", config.EnvClientID))
	flags.StringVar(&options.ClientSecret, "client-secret", options.ClientSecret, fmt.Sprintf("OAuth client secret (env %s)", config.EnvClientSecret))
	flags.StringVar(&options.AccessToken, "access-token", options.AccessToken, fmt.Sprintf("Bearer token override (env %s)", config.EnvAccessToken))
	flags.StringVar(&options.Output, "output", defaultIfEmpty(options.Output, "table"), fmt.Sprintf("Output format: table or json (env %s)", config.EnvOutput))
	flags.DurationVar(&options.Timeout, "timeout", options.Timeout, fmt.Sprintf("HTTP timeout (env %s)", config.EnvTimeout))

	rootCmd.AddCommand(newVersionCommand())

	for _, tag := range catalog.PreferredTags() {
		rootCmd.AddCommand(newTagCommand(options, tag))
	}

	return rootCmd, nil
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), buildinfo.Details())
			return err
		},
	}
}

func newTagCommand(options *config.Options, tag openapi.TagGroup) *cobra.Command {
	tagCmd := &cobra.Command{
		Use:   tag.Name,
		Short: fmt.Sprintf("%s operations", tag.Name),
	}

	for _, operation := range tag.Operations {
		tagCmd.AddCommand(newOperationCommand(options, operation))
	}

	return tagCmd
}

func newOperationCommand(options *config.Options, operation openapi.Operation) *cobra.Command {
	var body string
	var bodyFile string
	var fetchAll bool

	short := strings.TrimSpace(operation.Summary)
	if short == "" {
		short = fmt.Sprintf("%s %s", operation.Method, operation.Path)
	}

	longDescription := strings.TrimSpace(operation.Description)
	if longDescription == "" {
		longDescription = fmt.Sprintf("%s %s", operation.Method, operation.Path)
	}

	cmd := &cobra.Command{
		Use:   operation.CommandName,
		Short: short,
		Long:  longDescription,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Resolve(*options)
			if err != nil {
				return err
			}
			if cfg.OrganizationID == "" {
				return fmt.Errorf("missing organization ID: set --organization-id or %s", config.EnvOrganizationID)
			}

			httpClient := &http.Client{Timeout: cfg.Timeout}
			authProvider := auth.NewProvider(httpClient, cfg)

			pathValues, err := collectPathValues(cmd, cfg, operation.PathParameters)
			if err != nil {
				return err
			}

			queryValues, err := collectQueryValues(cmd, operation.QueryParameters)
			if err != nil {
				return err
			}

			requestBody, err := loadRequestBody(body, bodyFile, operation.BodyRequired)
			if err != nil {
				return err
			}

			executor := requestExecutor{
				cfg:          cfg,
				httpClient:   httpClient,
				authProvider: authProvider,
			}

			value, err := executor.execute(cmd.Context(), cmd, operation, pathValues, queryValues, requestBody, fetchAll)
			if err != nil {
				return err
			}

			return render.Write(cmd.OutOrStdout(), cfg.Output, value)
		},
	}

	if alias := strings.TrimSpace(operation.OperationID); alias != "" && alias != operation.CommandName {
		cmd.Aliases = []string{alias}
	}

	for _, parameter := range operation.PathParameters {
		if parameter.Name == "organization-id" {
			continue
		}
		addFlag(cmd, parameter)
		if parameter.Required {
			_ = cmd.MarkFlagRequired(parameter.FlagName)
		}
	}

	for _, parameter := range operation.QueryParameters {
		addFlag(cmd, parameter)
	}

	if operation.HasJSONBody {
		bodyHelp := "Inline JSON request body."
		if operation.BodyDescription != "" {
			bodyHelp = operation.BodyDescription
		}
		cmd.Flags().StringVar(&body, "body", "", bodyHelp)
		cmd.Flags().StringVar(&bodyFile, "body-file", "", "Path to a JSON request body file. Use - to read from stdin.")
	}

	if operation.Pagination != openapi.PaginationNone {
		cmd.Flags().BoolVar(&fetchAll, "all", false, "Automatically paginate through all available results.")
	}

	return cmd
}

func addFlag(cmd *cobra.Command, parameter openapi.Parameter) {
	helpText := buildHelpText(parameter)
	flags := cmd.Flags()

	switch parameter.Type {
	case openapi.ParamInteger:
		flags.Int(parameter.FlagName, intValue(parameter.Default), helpText)
	case openapi.ParamBoolean:
		flags.Bool(parameter.FlagName, boolValue(parameter.Default), helpText)
	case openapi.ParamArray:
		flags.StringSlice(parameter.FlagName, nil, helpText)
	default:
		flags.String(parameter.FlagName, stringValue(parameter.Default), helpText)
	}
}

func buildHelpText(parameter openapi.Parameter) string {
	parts := make([]string, 0, 4)
	if parameter.Description != "" {
		parts = append(parts, sanitizeHelpText(parameter.Description))
	}
	if len(parameter.Enum) > 0 {
		parts = append(parts, "Allowed: "+strings.Join(parameter.Enum, ", "))
	}
	if parameter.Deprecated {
		parts = append(parts, "Deprecated.")
	}
	if len(parts) == 0 {
		return parameter.Name
	}
	return strings.Join(parts, " ")
}

func sanitizeHelpText(value string) string {
	return strings.ReplaceAll(value, "`", "")
}

func collectPathValues(cmd *cobra.Command, cfg config.Runtime, parameters []openapi.Parameter) (map[string]string, error) {
	values := map[string]string{
		"organization-id": cfg.OrganizationID,
	}

	for _, parameter := range parameters {
		if parameter.Name == "organization-id" {
			continue
		}

		value, err := cmd.Flags().GetString(parameter.FlagName)
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		if parameter.Required && value == "" {
			return nil, fmt.Errorf("missing required path parameter --%s", parameter.FlagName)
		}
		if value != "" {
			values[parameter.Name] = value
		}
	}

	return values, nil
}

func collectQueryValues(cmd *cobra.Command, parameters []openapi.Parameter) (url.Values, error) {
	values := url.Values{}

	for _, parameter := range parameters {
		switch parameter.Type {
		case openapi.ParamInteger:
			if !cmd.Flags().Changed(parameter.FlagName) {
				continue
			}
			value, err := cmd.Flags().GetInt(parameter.FlagName)
			if err != nil {
				return nil, err
			}
			values.Set(parameter.Name, strconv.Itoa(value))
		case openapi.ParamBoolean:
			if !cmd.Flags().Changed(parameter.FlagName) {
				continue
			}
			value, err := cmd.Flags().GetBool(parameter.FlagName)
			if err != nil {
				return nil, err
			}
			values.Set(parameter.Name, strconv.FormatBool(value))
		case openapi.ParamArray:
			if !cmd.Flags().Changed(parameter.FlagName) {
				continue
			}
			items, err := cmd.Flags().GetStringSlice(parameter.FlagName)
			if err != nil {
				return nil, err
			}
			if parameter.ArrayFormat == openapi.ArrayEncodingCSV {
				values.Set(parameter.Name, strings.Join(items, ","))
				continue
			}
			for _, item := range items {
				values.Add(parameter.Name, item)
			}
		default:
			if !cmd.Flags().Changed(parameter.FlagName) {
				continue
			}
			value, err := cmd.Flags().GetString(parameter.FlagName)
			if err != nil {
				return nil, err
			}
			if value != "" {
				values.Set(parameter.Name, value)
			}
		}
	}

	return values, nil
}

func loadRequestBody(body string, bodyFile string, required bool) ([]byte, error) {
	if strings.TrimSpace(body) != "" && strings.TrimSpace(bodyFile) != "" {
		return nil, fmt.Errorf("use either --body or --body-file, not both")
	}

	var payload []byte
	switch {
	case strings.TrimSpace(body) != "":
		payload = []byte(strings.TrimSpace(body))
	case strings.TrimSpace(bodyFile) != "":
		var err error
		if bodyFile == "-" {
			payload, err = io.ReadAll(os.Stdin)
		} else {
			payload, err = os.ReadFile(bodyFile)
		}
		if err != nil {
			return nil, err
		}
		payload = bytes.TrimSpace(payload)
	}

	if len(payload) == 0 {
		if required {
			return nil, fmt.Errorf("this operation requires a JSON request body")
		}
		return nil, nil
	}

	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, fmt.Errorf("invalid JSON request body: %w", err)
	}

	return payload, nil
}

type requestExecutor struct {
	cfg          config.Runtime
	httpClient   *http.Client
	authProvider *auth.Provider
}

func (r requestExecutor) execute(
	ctx context.Context,
	cmd *cobra.Command,
	operation openapi.Operation,
	pathValues map[string]string,
	queryValues url.Values,
	requestBody []byte,
	fetchAll bool,
) (any, error) {
	initialURL, err := buildURL(r.cfg.BaseURL, operation.Path, pathValues, queryValues)
	if err != nil {
		return nil, err
	}

	currentURL, err := url.Parse(initialURL)
	if err != nil {
		return nil, err
	}

	var result any
	pageSize := effectivePageSize(cmd, operation.QueryParameters)

	for {
		responseValue, headers, err := r.doRequest(ctx, operation.Method, currentURL.String(), requestBody)
		if err != nil {
			return nil, err
		}

		if !fetchAll || operation.Pagination == openapi.PaginationNone {
			return responseValue, nil
		}

		result = mergePaginated(result, responseValue)

		nextURL, ok, err := nextPageURL(operation.Pagination, currentURL, headers, responseValue, pageSize)
		if err != nil {
			return nil, err
		}
		if !ok {
			return result, nil
		}
		currentURL = nextURL
	}
}

func (r requestExecutor) doRequest(ctx context.Context, method string, rawURL string, requestBody []byte) (any, http.Header, error) {
	var bodyReader io.Reader
	if len(requestBody) > 0 {
		bodyReader = bytes.NewReader(requestBody)
	}

	request, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", buildinfo.UserAgent())
	if len(requestBody) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}

	authHeader, err := r.authProvider.AuthorizationHeader(ctx)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Authorization", authHeader)

	response, err := r.httpClient.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}

	if response.StatusCode >= http.StatusBadRequest {
		return nil, response.Header, formatHTTPError(response.Status, payload)
	}

	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil, response.Header, nil
	}

	var value any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, response.Header, fmt.Errorf("decode response: %w", err)
	}

	return value, response.Header, nil
}

func buildURL(baseURL string, path string, pathValues map[string]string, queryValues url.Values) (string, error) {
	expandedPath := path
	for key, value := range pathValues {
		expandedPath = strings.ReplaceAll(expandedPath, "{"+key+"}", url.PathEscape(value))
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsedBase.RawQuery = ""
	parsedBase.Fragment = ""

	base := strings.TrimRight(parsedBase.String(), "/")
	fullURL := base + "/" + strings.TrimLeft(expandedPath, "/")
	if encodedQuery := queryValues.Encode(); encodedQuery != "" {
		fullURL += "?" + encodedQuery
	}
	return fullURL, nil
}

func effectivePageSize(cmd *cobra.Command, parameters []openapi.Parameter) int {
	for _, parameter := range parameters {
		if parameter.Name != "per-page" && parameter.Name != "limit" {
			continue
		}
		if cmd.Flags().Changed(parameter.FlagName) {
			value, err := cmd.Flags().GetInt(parameter.FlagName)
			if err == nil {
				return value
			}
		}
		return intValue(parameter.Default)
	}
	return 0
}

func nextPageURL(
	style openapi.PaginationStyle,
	currentURL *url.URL,
	headers http.Header,
	responseValue any,
	pageSize int,
) (*url.URL, bool, error) {
	switch style {
	case openapi.PaginationV1Page:
		itemCount := countItems(responseValue)
		if itemCount == 0 {
			return nil, false, nil
		}
		if pageSize > 0 && itemCount < pageSize {
			return nil, false, nil
		}

		nextURL := *currentURL
		query := nextURL.Query()
		currentPage := 1
		if raw := query.Get("page"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				return nil, false, fmt.Errorf("parse page query parameter: %w", err)
			}
			currentPage = value
		}
		query.Set("page", strconv.Itoa(currentPage+1))
		nextURL.RawQuery = query.Encode()
		return &nextURL, true, nil
	case openapi.PaginationV1Token:
		next, ok, err := parseNextLink(headers.Values("Link"))
		if err != nil || !ok {
			return nil, ok, err
		}
		parsed, err := url.Parse(next)
		if err != nil {
			return nil, false, err
		}
		return parsed, true, nil
	case openapi.PaginationV2Cursor:
		nextCursor := extractNextCursor(responseValue)
		if nextCursor == "" {
			return nil, false, nil
		}
		nextURL := *currentURL
		query := nextURL.Query()
		query.Set("cursor", nextCursor)
		nextURL.RawQuery = query.Encode()
		return &nextURL, true, nil
	default:
		return nil, false, nil
	}
}

func parseNextLink(values []string) (string, bool, error) {
	for _, value := range values {
		for _, part := range splitLinkHeader(value) {
			segments := strings.Split(part, ";")
			if len(segments) == 0 {
				continue
			}

			link := strings.TrimSpace(segments[0])
			link = strings.TrimPrefix(link, "<")
			link = strings.TrimSuffix(link, ">")

			for _, segment := range segments[1:] {
				segment = strings.TrimSpace(segment)
				if segment == `rel="next"` {
					return link, true, nil
				}
			}
		}
	}
	return "", false, nil
}

func splitLinkHeader(header string) []string {
	parts := []string{}
	var builder strings.Builder
	inAngles := false
	for _, runeValue := range header {
		switch runeValue {
		case '<':
			inAngles = true
			builder.WriteRune(runeValue)
		case '>':
			inAngles = false
			builder.WriteRune(runeValue)
		case ',':
			if inAngles {
				builder.WriteRune(runeValue)
				continue
			}
			parts = append(parts, strings.TrimSpace(builder.String()))
			builder.Reset()
		default:
			builder.WriteRune(runeValue)
		}
	}
	if builder.Len() > 0 {
		parts = append(parts, strings.TrimSpace(builder.String()))
	}
	return parts
}

func extractNextCursor(value any) string {
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	metadata, ok := object["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	switch typed := metadata["next_cursor"].(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func countItems(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case map[string]any:
		items, ok := typed["items"].([]any)
		if !ok {
			return 0
		}
		return len(items)
	default:
		return 0
	}
}

func mergePaginated(existing any, next any) any {
	if existing == nil {
		return next
	}

	switch current := existing.(type) {
	case []any:
		nextItems, ok := next.([]any)
		if !ok {
			return next
		}
		combined := append([]any{}, current...)
		combined = append(combined, nextItems...)
		return combined
	case map[string]any:
		nextMap, ok := next.(map[string]any)
		if !ok {
			return next
		}

		collectionKey, currentItems, nextItems, ok := paginatedCollection(current, nextMap)
		if !ok {
			return next
		}

		combined := map[string]any{}
		for key, value := range current {
			combined[key] = value
		}
		for key, value := range nextMap {
			combined[key] = value
		}

		items := append([]any{}, currentItems...)
		items = append(items, nextItems...)
		combined[collectionKey] = items
		return combined
	default:
		return next
	}
}

func paginatedCollection(current map[string]any, next map[string]any) (string, []any, []any, bool) {
	for _, key := range []string{"items", "resourceFindings"} {
		currentItems, currentOK := current[key].([]any)
		nextItems, nextOK := next[key].([]any)
		if currentOK && nextOK {
			return key, currentItems, nextItems, true
		}
	}

	return "", nil, nil, false
}

func formatHTTPError(status string, payload []byte) error {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return fmt.Errorf("request failed: %s", status)
	}

	var value any
	if json.Unmarshal(payload, &value) == nil {
		pretty, err := json.MarshalIndent(value, "", "  ")
		if err == nil {
			return fmt.Errorf("request failed: %s\n%s", status, string(pretty))
		}
	}

	return fmt.Errorf("request failed: %s\n%s", status, string(payload))
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func defaultIfEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
