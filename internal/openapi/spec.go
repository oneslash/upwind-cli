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

	tagNames := make([]string, 0, len(grouped))
	for tag := range grouped {
		tagNames = append(tagNames, tag)
	}
	sort.Strings(tagNames)

	tags := make([]TagGroup, 0, len(tagNames))
	for _, tagName := range tagNames {
		operationsByCommand := grouped[tagName]
		operations := make([]Operation, 0, len(operationsByCommand))
		for _, operation := range operationsByCommand {
			operations = append(operations, operation)
		}

		sort.Slice(operations, func(i, j int) bool {
			if operations[i].CommandName == operations[j].CommandName {
				if operations[i].Path == operations[j].Path {
					return operations[i].Method < operations[j].Method
				}
				return operations[i].Path < operations[j].Path
			}
			return operations[i].CommandName < operations[j].CommandName
		})

		tags = append(tags, TagGroup{
			Name:       tagName,
			Operations: operations,
		})
	}

	return tags
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
		for method, operation := range map[string]*rawOperation{
			"GET":    item.Get,
			"POST":   item.Post,
			"PATCH":  item.Patch,
			"PUT":    item.Put,
			"DELETE": item.Delete,
		} {
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

	tagNames := make([]string, 0, len(grouped))
	for tag := range grouped {
		tagNames = append(tagNames, tag)
	}
	sort.Strings(tagNames)

	versionGroup := VersionGroup{Name: version}
	for _, tag := range tagNames {
		operations := grouped[tag]
		sort.Slice(operations, func(i, j int) bool {
			if operations[i].CommandName == operations[j].CommandName {
				if operations[i].Path == operations[j].Path {
					return operations[i].Method < operations[j].Method
				}
				return operations[i].Path < operations[j].Path
			}
			return operations[i].CommandName < operations[j].CommandName
		})

		versionGroup.Tags = append(versionGroup.Tags, TagGroup{
			Name:       tag,
			Operations: operations,
		})
	}

	return versionGroup, nil
}

func resolveOperation(version, method, path string, operation *rawOperation, components rawComponents) (Operation, error) {
	parameters := make([]Parameter, 0, len(operation.Parameters))
	var pathParameters []Parameter
	var queryParameters []Parameter

	for _, candidate := range operation.Parameters {
		resolved, err := resolveParameter(candidate, components)
		if err != nil {
			return Operation{}, err
		}
		parameters = append(parameters, resolved)
	}

	for _, parameter := range parameters {
		switch parameter.In {
		case "path":
			pathParameters = append(pathParameters, parameter)
		case "query":
			queryParameters = append(queryParameters, parameter)
		}
	}

	tag := "misc"
	if len(operation.Tags) > 0 && strings.TrimSpace(operation.Tags[0]) != "" {
		tag = strings.TrimSpace(operation.Tags[0])
	}

	pagination := detectPagination(queryParameters)

	bodyDescription := ""
	hasJSONBody := false
	bodyRequired := false
	if operation.RequestBody != nil {
		if _, ok := operation.RequestBody.Content["application/json"]; ok {
			hasJSONBody = true
			bodyRequired = operation.RequestBody.Required
			bodyDescription = strings.TrimSpace(operation.RequestBody.Description)
		}
	}

	commandName := toKebabCase(operation.OperationID)
	if commandName == "" {
		commandName = toKebabCase(strings.TrimSpace(operation.Summary))
	}
	if commandName == "" {
		commandName = strings.ToLower(method)
	}

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
	if parameter.Ref != "" {
		name := strings.TrimPrefix(parameter.Ref, "#/components/parameters/")
		resolved, ok := components.Parameters[name]
		if !ok {
			return Parameter{}, fmt.Errorf("unresolved parameter ref %q", parameter.Ref)
		}
		parameter = resolved
	}

	paramType := ParamString
	switch strings.ToLower(strings.TrimSpace(parameter.Schema.Type)) {
	case "integer":
		paramType = ParamInteger
	case "boolean":
		paramType = ParamBoolean
	case "array":
		paramType = ParamArray
	}

	itemType := ParamString
	if parameter.Schema.Items != nil {
		switch strings.ToLower(strings.TrimSpace(parameter.Schema.Items.Type)) {
		case "integer":
			itemType = ParamInteger
		case "boolean":
			itemType = ParamBoolean
		}
	}

	enumValues := make([]string, 0, len(parameter.Schema.Enum))
	for _, value := range parameter.Schema.Enum {
		enumValues = append(enumValues, fmt.Sprint(value))
	}

	arrayFormat := ArrayEncodingRepeat
	if parameter.Name == "cloud-account-tags" {
		arrayFormat = ArrayEncodingCSV
	}

	return Parameter{
		Name:        parameter.Name,
		FlagName:    strings.TrimSpace(parameter.Name),
		In:          strings.TrimSpace(parameter.In),
		Description: strings.TrimSpace(parameter.Description),
		Required:    parameter.Required,
		Deprecated:  parameter.Deprecated,
		Type:        paramType,
		ItemType:    itemType,
		ArrayFormat: arrayFormat,
		Default:     parameter.Schema.Default,
		Enum:        enumValues,
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
