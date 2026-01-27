package documentation

import (
	"fmt"
	"sync"
)

// Endpoint represents an API endpoint
type Endpoint struct {
	Path        string
	Method      string
	Summary     string
	Description string
	Parameters  []Parameter
	RequestBody RequestBody
	Response    Response
	Examples    []Example
	Tags        []string
	Deprecated  bool
}

// Parameter represents an API parameter
type Parameter struct {
	Name        string
	In          string // query, path, header, body
	Type        string // string, integer, boolean, etc
	Description string
	Required    bool
	Example     interface{}
}

// RequestBody represents request body specification
type RequestBody struct {
	Description string
	Content     map[string]interface{}
	Required    bool
	Example     interface{}
}

// Response represents response specification
type Response struct {
	Status      int
	Description string
	Schema      map[string]interface{}
	Example     interface{}
}

// Example represents an API example
type Example struct {
	Title       string
	Description string
	Request     string
	Response    string
	StatusCode  int
}

// APIDocumentation represents complete API documentation
type APIDocumentation struct {
	Title           string
	Description     string
	Version         string
	BaseURL         string
	Endpoints       map[string]*Endpoint
	Schemas         map[string]interface{}
	SecuritySchemes map[string]interface{}
}

// APIDocumentationBuilder builds API documentation
type APIDocumentationBuilder struct {
	mu  sync.RWMutex
	doc *APIDocumentation
}

// NewAPIDocumentationBuilder creates a new builder
func NewAPIDocumentationBuilder(title, description, version, baseURL string) *APIDocumentationBuilder {
	return &APIDocumentationBuilder{
		doc: &APIDocumentation{
			Title:           title,
			Description:     description,
			Version:         version,
			BaseURL:         baseURL,
			Endpoints:       make(map[string]*Endpoint),
			Schemas:         make(map[string]interface{}),
			SecuritySchemes: make(map[string]interface{}),
		},
	}
}

// AddEndpoint adds an endpoint to the documentation
func (adb *APIDocumentationBuilder) AddEndpoint(method, path string, endpoint *Endpoint) {
	adb.mu.Lock()
	defer adb.mu.Unlock()

	key := fmt.Sprintf("%s %s", method, path)
	endpoint.Method = method
	endpoint.Path = path
	adb.doc.Endpoints[key] = endpoint
}

// AddSchema adds a schema definition
func (adb *APIDocumentationBuilder) AddSchema(name string, schema interface{}) {
	adb.mu.Lock()
	defer adb.mu.Unlock()

	adb.doc.Schemas[name] = schema
}

// Build returns the built documentation
func (adb *APIDocumentationBuilder) Build() *APIDocumentation {
	adb.mu.RLock()
	defer adb.mu.RUnlock()

	return adb.doc
}

// GenerateOpenAPISpec generates OpenAPI 3.0 specification
func (adb *APIDocumentationBuilder) GenerateOpenAPISpec() map[string]interface{} {
	adb.mu.RLock()
	defer adb.mu.RUnlock()

	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       adb.doc.Title,
			"description": adb.doc.Description,
			"version":     adb.doc.Version,
		},
		"servers": []map[string]interface{}{
			{
				"url":         adb.doc.BaseURL,
				"description": "API Server",
			},
		},
		"paths": adb.generatePaths(),
		"components": map[string]interface{}{
			"schemas": adb.doc.Schemas,
		},
	}

	return spec
}

// generatePaths generates OpenAPI paths object
func (adb *APIDocumentationBuilder) generatePaths() map[string]interface{} {
	paths := make(map[string]interface{})

	for key, endpoint := range adb.doc.Endpoints {
		method := endpoint.Method
		path := endpoint.Path

		if _, exists := paths[path]; !exists {
			paths[path] = make(map[string]interface{})
		}

		pathObj := paths[path].(map[string]interface{})

		operation := map[string]interface{}{
			"summary":     endpoint.Summary,
			"description": endpoint.Description,
			"tags":        endpoint.Tags,
		}

		// Add parameters
		if len(endpoint.Parameters) > 0 {
			params := make([]map[string]interface{}, 0)
			for _, param := range endpoint.Parameters {
				params = append(params, map[string]interface{}{
					"name":        param.Name,
					"in":          param.In,
					"required":    param.Required,
					"schema":      map[string]interface{}{"type": param.Type},
					"description": param.Description,
					"example":     param.Example,
				})
			}
			operation["parameters"] = params
		}

		// Add request body
		if endpoint.RequestBody.Description != "" {
			operation["requestBody"] = map[string]interface{}{
				"description": endpoint.RequestBody.Description,
				"required":    endpoint.RequestBody.Required,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema":   endpoint.RequestBody.Content,
						"examples": endpoint.RequestBody.Example,
					},
				},
			}
		}

		// Add responses
		responses := make(map[string]interface{})
		if endpoint.Response.Status > 0 {
			responses[fmt.Sprintf("%d", endpoint.Response.Status)] = map[string]interface{}{
				"description": endpoint.Response.Description,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema":  endpoint.Response.Schema,
						"example": endpoint.Response.Example,
					},
				},
			}
		}

		operation["responses"] = responses

		pathObj[method] = operation
	}

	return paths
}

// GenerateMarkdownDocs generates Markdown documentation
func (adb *APIDocumentationBuilder) GenerateMarkdownDocs() string {
	adb.mu.RLock()
	defer adb.mu.RUnlock()

	markdown := fmt.Sprintf("# %s API Documentation\n\n", adb.doc.Title)
	markdown += fmt.Sprintf("**Version:** %s\n\n", adb.doc.Version)
	markdown += fmt.Sprintf("**Description:** %s\n\n", adb.doc.Description)
	markdown += fmt.Sprintf("**Base URL:** `%s`\n\n", adb.doc.BaseURL)

	markdown += "## Endpoints\n\n"

	for _, endpoint := range adb.doc.Endpoints {
		markdown += fmt.Sprintf("### %s %s\n\n", endpoint.Method, endpoint.Path)
		markdown += fmt.Sprintf("%s\n\n", endpoint.Summary)
		markdown += fmt.Sprintf("**Description:** %s\n\n", endpoint.Description)

		// Parameters
		if len(endpoint.Parameters) > 0 {
			markdown += "**Parameters:**\n\n"
			markdown += "| Name | In | Type | Required | Description |\n"
			markdown += "|------|----|----|----------|-------------|\n"
			for _, param := range endpoint.Parameters {
				required := "No"
				if param.Required {
					required = "Yes"
				}
				markdown += fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
					param.Name, param.In, param.Type, required, param.Description)
			}
			markdown += "\n"
		}

		// Examples
		if len(endpoint.Examples) > 0 {
			markdown += "**Examples:**\n\n"
			for _, example := range endpoint.Examples {
				markdown += fmt.Sprintf("**%s**\n\n", example.Title)
				markdown += "```bash\n"
				markdown += fmt.Sprintf("curl -X %s %s%s\n", endpoint.Method, adb.doc.BaseURL, endpoint.Path)
				markdown += "```\n\n"
				markdown += "Response:\n"
				markdown += "```json\n"
				markdown += example.Response + "\n"
				markdown += "```\n\n"
			}
		}

		markdown += "---\n\n"
	}

	return markdown
}

// APIVersioning manages API versioning
type APIVersioning struct {
	CurrentVersion string
	Versions       map[string]*APIDocumentation
	Deprecated     []string
}

// NewAPIVersioning creates API versioning info
func NewAPIVersioning(currentVersion string) *APIVersioning {
	return &APIVersioning{
		CurrentVersion: currentVersion,
		Versions:       make(map[string]*APIDocumentation),
		Deprecated:     make([]string, 0),
	}
}

// RegisterVersion registers an API version
func (av *APIVersioning) RegisterVersion(version string, doc *APIDocumentation) {
	av.Versions[version] = doc
}

// DeprecateVersion marks a version as deprecated
func (av *APIVersioning) DeprecateVersion(version string) {
	av.Deprecated = append(av.Deprecated, version)
}

// IsVersionSupported checks if a version is supported
func (av *APIVersioning) IsVersionSupported(version string) bool {
	_, exists := av.Versions[version]
	return exists && !av.isVersionDeprecated(version)
}

// isVersionDeprecated checks if a version is deprecated
func (av *APIVersioning) isVersionDeprecated(version string) bool {
	for _, v := range av.Deprecated {
		if v == version {
			return true
		}
	}
	return false
}

// RateLimitingDocs documents rate limiting
type RateLimitingDocs struct {
	RequestsPerMinute int
	RequestsPerHour   int
	BurstSize         int
	RetryAfter        string
}

// SecurityDocs documents security requirements
type SecurityDocs struct {
	AuthType     string // API_KEY, BEARER, BASIC, OAUTH2
	Description  string
	HeaderName   string
	Scopes       []string
	ExampleUsage string
}

// CreateSecurityDocs creates security documentation
func CreateSecurityDocs() *SecurityDocs {
	return &SecurityDocs{
		AuthType:     "BEARER",
		Description:  "JWT Bearer token for API authentication",
		HeaderName:   "Authorization",
		ExampleUsage: "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	}
}
