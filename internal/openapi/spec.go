package openapi

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

//go:generate go run ../../tools/openapi-gen -spec-dir ../../../spec-upwind -out catalog_generated.go

type PaginationStyle string

const (
	PaginationNone     PaginationStyle = "none"
	PaginationV1Page   PaginationStyle = "v1-page"
	PaginationV1Token  PaginationStyle = "v1-token"
	PaginationV2Cursor PaginationStyle = "v2-cursor"
)

type ParamType string

const (
	ParamString  ParamType = "string"
	ParamInteger ParamType = "integer"
	ParamBoolean ParamType = "boolean"
	ParamArray   ParamType = "array"
)

type ArrayEncoding string

const (
	ArrayEncodingRepeat ArrayEncoding = "repeat"
	ArrayEncodingCSV    ArrayEncoding = "csv"
)

type Parameter struct {
	Name        string
	FlagName    string
	In          string
	Description string
	Required    bool
	Deprecated  bool
	Type        ParamType
	ItemType    ParamType
	ArrayFormat ArrayEncoding
	Default     any
	Enum        []string
}

type Operation struct {
	Version         string
	Tag             string
	CommandName     string
	OperationID     string
	Summary         string
	Description     string
	Method          string
	Path            string
	PathParameters  []Parameter
	QueryParameters []Parameter
	HasJSONBody     bool
	BodyRequired    bool
	BodyDescription string
	Pagination      PaginationStyle
}

type TagGroup struct {
	Name       string
	Operations []Operation
}

type VersionGroup struct {
	Name string
	Tags []TagGroup
}

type Catalog struct {
	Versions []VersionGroup
}

func (c Catalog) PreferredTags() []TagGroup {
	grouped := map[string]map[string]Operation{}

	for _, version := range c.Versions {
		for _, tag := range version.Tags {
			operationsByCommand, ok := grouped[tag.Name]
			if !ok {
				operationsByCommand = map[string]Operation{}
				grouped[tag.Name] = operationsByCommand
			}

			for _, operation := range tag.Operations {
				existing, ok := operationsByCommand[operation.CommandName]
				if !ok || prefersOperation(operation, existing) {
					operationsByCommand[operation.CommandName] = operation
				}
			}
		}
	}

	return preferredTagGroups(grouped)
}

type rawDocument struct {
	Paths      map[string]rawPathItem `yaml:"paths"`
	Components rawComponents          `yaml:"components"`
}

type rawComponents struct {
	Parameters map[string]rawParameter `yaml:"parameters"`
}

type rawPathItem struct {
	Get    *rawOperation `yaml:"get"`
	Post   *rawOperation `yaml:"post"`
	Patch  *rawOperation `yaml:"patch"`
	Put    *rawOperation `yaml:"put"`
	Delete *rawOperation `yaml:"delete"`
}

type rawOperation struct {
	OperationID string                 `yaml:"operationId"`
	Summary     string                 `yaml:"summary"`
	Description string                 `yaml:"description"`
	Tags        []string               `yaml:"tags"`
	Parameters  []rawParameter         `yaml:"parameters"`
	RequestBody *rawRequestBody        `yaml:"requestBody"`
	Responses   map[string]rawResponse `yaml:"responses"`
}

type rawResponse struct{}

type rawRequestBody struct {
	Ref         string                  `yaml:"$ref"`
	Description string                  `yaml:"description"`
	Required    bool                    `yaml:"required"`
	Content     map[string]rawMediaType `yaml:"content"`
}

type rawMediaType struct {
	Schema rawSchema `yaml:"schema"`
}

type rawParameter struct {
	Ref         string    `yaml:"$ref"`
	Name        string    `yaml:"name"`
	In          string    `yaml:"in"`
	Description string    `yaml:"description"`
	Required    bool      `yaml:"required"`
	Deprecated  bool      `yaml:"deprecated"`
	Schema      rawSchema `yaml:"schema"`
}

type rawSchema struct {
	Type    string     `yaml:"type"`
	Format  string     `yaml:"format"`
	Default any        `yaml:"default"`
	Enum    []any      `yaml:"enum"`
	Items   *rawSchema `yaml:"items"`
	Ref     string     `yaml:"$ref"`
}

func LoadCatalog() (*Catalog, error) {
	catalog := generatedCatalog()
	return &catalog, nil
}

func ParseVersion(version string, contents []byte) (VersionGroup, error) {
	var document rawDocument
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return VersionGroup{}, fmt.Errorf("parse %s spec: %w", version, err)
	}

	grouped := map[string][]Operation{}
	for path, item := range document.Paths {
		for method, operation := range item.operationsByMethod() {
			if operation == nil {
				continue
			}

			resolved, err := resolveOperation(version, method, path, operation, document.Components)
			if err != nil {
				return VersionGroup{}, err
			}

			grouped[resolved.Tag] = append(grouped[resolved.Tag], resolved)
		}
	}

	versionGroup := VersionGroup{Name: version}
	versionGroup.Tags = append(versionGroup.Tags, buildTagGroups(grouped)...)

	return versionGroup, nil
}

func resolveOperation(version, method, path string, operation *rawOperation, components rawComponents) (Operation, error) {
	pathParameters, queryParameters, err := resolveOperationParameters(operation.Parameters, components)
	if err != nil {
		return Operation{}, err
	}

	tag := primaryTag(operation.Tags)
	pagination := detectPagination(queryParameters)
	hasJSONBody, bodyRequired, bodyDescription := requestBodyInfo(operation.RequestBody)
	commandName := operationCommandName(operation, method)

	return Operation{
		Version:         version,
		Tag:             tag,
		CommandName:     commandName,
		OperationID:     operation.OperationID,
		Summary:         operation.Summary,
		Description:     strings.TrimSpace(operation.Description),
		Method:          method,
		Path:            path,
		PathParameters:  pathParameters,
		QueryParameters: queryParameters,
		HasJSONBody:     hasJSONBody,
		BodyRequired:    bodyRequired,
		BodyDescription: bodyDescription,
		Pagination:      pagination,
	}, nil
}

func resolveParameter(parameter rawParameter, components rawComponents) (Parameter, error) {
	resolved, err := resolveParameterRef(parameter, components)
	if err != nil {
		return Parameter{}, err
	}

	return Parameter{
		Name:        resolved.Name,
		FlagName:    strings.TrimSpace(resolved.Name),
		In:          strings.TrimSpace(resolved.In),
		Description: strings.TrimSpace(resolved.Description),
		Required:    resolved.Required,
		Deprecated:  resolved.Deprecated,
		Type:        schemaParamType(resolved.Schema),
		ItemType:    schemaItemType(resolved.Schema.Items),
		ArrayFormat: arrayEncodingForParameter(resolved.Name),
		Default:     resolved.Schema.Default,
		Enum:        enumStrings(resolved.Schema.Enum),
	}, nil
}

func detectPagination(parameters []Parameter) PaginationStyle {
	hasPage := false
	hasPageToken := false
	hasCursor := false

	for _, parameter := range parameters {
		switch parameter.Name {
		case "page":
			hasPage = true
		case "page-token":
			hasPageToken = true
		case "cursor":
			hasCursor = true
		}
	}

	switch {
	case hasCursor:
		return PaginationV2Cursor
	case hasPageToken:
		return PaginationV1Token
	case hasPage:
		return PaginationV1Page
	default:
		return PaginationNone
	}
}

func toKebabCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastHyphen := false
	for index, runeValue := range value {
		switch {
		case unicode.IsUpper(runeValue):
			if index > 0 && !lastHyphen {
				builder.WriteByte('-')
			}
			builder.WriteRune(unicode.ToLower(runeValue))
			lastHyphen = false
		case unicode.IsLetter(runeValue) || unicode.IsDigit(runeValue):
			builder.WriteRune(unicode.ToLower(runeValue))
			lastHyphen = false
		default:
			if !lastHyphen {
				builder.WriteByte('-')
				lastHyphen = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}

func prefersOperation(candidate Operation, existing Operation) bool {
	candidatePriority := versionPriority(candidate.Version)
	existingPriority := versionPriority(existing.Version)
	if candidatePriority != existingPriority {
		return candidatePriority > existingPriority
	}

	if candidate.Path != existing.Path {
		return candidate.Path < existing.Path
	}

	return candidate.Method < existing.Method
}

func versionPriority(version string) int {
	switch strings.ToLower(strings.TrimSpace(version)) {
	case "v2":
		return 2
	case "v1":
		return 1
	default:
		return 0
	}
}

func (item rawPathItem) operationsByMethod() map[string]*rawOperation {
	return map[string]*rawOperation{
		"GET":    item.Get,
		"POST":   item.Post,
		"PATCH":  item.Patch,
		"PUT":    item.Put,
		"DELETE": item.Delete,
	}
}

func preferredTagGroups(grouped map[string]map[string]Operation) []TagGroup {
	tagNames := sortedMapKeys(grouped)
	tags := make([]TagGroup, 0, len(tagNames))
	for _, tagName := range tagNames {
		tags = append(tags, TagGroup{
			Name:       tagName,
			Operations: sortedPreferredOperations(grouped[tagName]),
		})
	}

	return tags
}

func buildTagGroups(grouped map[string][]Operation) []TagGroup {
	tagNames := sortedMapKeys(grouped)
	tags := make([]TagGroup, 0, len(tagNames))
	for _, tagName := range tagNames {
		tags = append(tags, TagGroup{
			Name:       tagName,
			Operations: sortOperations(grouped[tagName]),
		})
	}

	return tags
}

func sortedPreferredOperations(operationsByCommand map[string]Operation) []Operation {
	operations := make([]Operation, 0, len(operationsByCommand))
	for _, operation := range operationsByCommand {
		operations = append(operations, operation)
	}

	return sortOperations(operations)
}

func sortOperations(operations []Operation) []Operation {
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].CommandName == operations[j].CommandName {
			if operations[i].Path == operations[j].Path {
				return operations[i].Method < operations[j].Method
			}
			return operations[i].Path < operations[j].Path
		}
		return operations[i].CommandName < operations[j].CommandName
	})

	return operations
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func resolveOperationParameters(parameters []rawParameter, components rawComponents) ([]Parameter, []Parameter, error) {
	resolvedParameters := make([]Parameter, 0, len(parameters))
	for _, candidate := range parameters {
		resolved, err := resolveParameter(candidate, components)
		if err != nil {
			return nil, nil, err
		}
		resolvedParameters = append(resolvedParameters, resolved)
	}

	pathParameters := make([]Parameter, 0, len(resolvedParameters))
	queryParameters := make([]Parameter, 0, len(resolvedParameters))
	for _, parameter := range resolvedParameters {
		switch parameter.In {
		case "path":
			pathParameters = append(pathParameters, parameter)
		case "query":
			queryParameters = append(queryParameters, parameter)
		}
	}

	return pathParameters, queryParameters, nil
}

func primaryTag(tags []string) string {
	if len(tags) == 0 {
		return "misc"
	}

	tag := strings.TrimSpace(tags[0])
	if tag == "" {
		return "misc"
	}

	return tag
}

func requestBodyInfo(body *rawRequestBody) (bool, bool, string) {
	if body == nil {
		return false, false, ""
	}

	if _, ok := body.Content["application/json"]; !ok {
		return false, false, ""
	}

	return true, body.Required, strings.TrimSpace(body.Description)
}

func operationCommandName(operation *rawOperation, method string) string {
	commandName := toKebabCase(operation.OperationID)
	if commandName == "" {
		commandName = toKebabCase(strings.TrimSpace(operation.Summary))
	}
	if commandName == "" {
		commandName = strings.ToLower(method)
	}

	return commandName
}

func resolveParameterRef(parameter rawParameter, components rawComponents) (rawParameter, error) {
	if parameter.Ref == "" {
		return parameter, nil
	}

	name := strings.TrimPrefix(parameter.Ref, "#/components/parameters/")
	resolved, ok := components.Parameters[name]
	if !ok {
		return rawParameter{}, fmt.Errorf("unresolved parameter ref %q", parameter.Ref)
	}

	return resolved, nil
}

func schemaParamType(schema rawSchema) ParamType {
	switch normalizedSchemaType(schema.Type) {
	case "integer":
		return ParamInteger
	case "boolean":
		return ParamBoolean
	case "array":
		return ParamArray
	default:
		return ParamString
	}
}

func schemaItemType(items *rawSchema) ParamType {
	if items == nil {
		return ParamString
	}

	switch normalizedSchemaType(items.Type) {
	case "integer":
		return ParamInteger
	case "boolean":
		return ParamBoolean
	default:
		return ParamString
	}
}

func normalizedSchemaType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func enumStrings(values []any) []string {
	enumValues := make([]string, 0, len(values))
	for _, value := range values {
		enumValues = append(enumValues, fmt.Sprint(value))
	}

	return enumValues
}

func arrayEncodingForParameter(name string) ArrayEncoding {
	if name == "cloud-account-tags" {
		return ArrayEncodingCSV
	}

	return ArrayEncodingRepeat
}
