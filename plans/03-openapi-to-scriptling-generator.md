# Plan: Phase 3 - OpenAPI Spec to Scriptling Library Generator

**Status:** Planning
**Priority:** High
**Dependencies:** Phase 1 (Typed Kwargs), Phase 2 (Validator)
**Estimated Complexity:** High

## Overview

Build a standalone tool that converts OpenAPI 3.x specifications into Scriptling library code. The tool will generate type-safe, documented, and production-ready API client libraries that follow Scriptling best practices.

## Goals

1. Parse OpenAPI 3.x specifications (YAML/JSON)
2. Generate Scriptling library code with proper structure
3. Support authentication (API key, OAuth2, Bearer tokens)
4. Validate generated code using Phase 2 validator
5. Provide CLI and library interfaces
6. Support selective endpoint generation
7. Generate comprehensive documentation

## Architecture

### Components

```
tools/openapi-gen/
├── main.go                    # CLI entry point
├── generator/
│   ├── generator.go           # Main generator orchestration
│   ├── parser.go              # OpenAPI spec parser
│   ├── builder.go             # Scriptling code builder
│   ├── templates.go           # Code templates
│   └── validator.go           # Generated code validator
├── config/
│   ├── config.go              # Configuration handling
│   └── toml.go                # TOML config support
├── openapi/
│   ├── openapi.go             # OpenAPI spec models
│   └── parser.go              # Parse YAML/JSON specs
├── output/
│   ├── library.go             # Library file generation
│   ├── models.go              # Model generation (optional)
│   └── formatter.go           # Code formatting
└── templates/
    ├── library.py.tmpl        # Library template
    ├── client.py.tmpl         # API client template
    └── function.py.tmpl       # Function template

extlibs/auth/                  # Authentication support library
├── auth.go                    # Main auth library
├── oauth.go                   # OAuth2 implementation
├── apikey.go                  # API key implementation
└── bearer.go                  # Bearer token implementation
```

## Files to Create

### 1. `tools/openapi-gen/main.go` (NEW)

**Purpose:** CLI entry point

```go
package main

import (
    "fmt"
    "os"
    "github.com/paularlott/scriptling/tools/openapi-generator/generator"
    "github.com/paularlott/scriptling/tools/openapi-generator/config"
)

func main() {
    cmd := parseCommand()

    if cmd.Validate {
        // Validate mode
        if err := validateConfig(cmd.Config); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Println("✓ Configuration is valid")
        return
    }

    // Generation mode
    gen, err := generator.New(cmd.Config)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    if err := gen.Generate(); err != nil {
        fmt.Fprintf(os.Stderr, "Generation failed: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("✓ Library generated successfully")
}
```

### 2. `tools/openapi-generator/config/config.go` (NEW)

**Purpose:** Configuration structure

```go
package config

import (
    "os"
    "toml"
)

// Config defines the generator configuration
type Config struct {
    Library     LibraryConfig     `toml:"library"`
    Source      SourceConfig      `toml:"source"`
    Endpoints   []EndpointConfig  `toml:"endpoints"`
    Output      OutputConfig      `toml:"output"`
    Auth        AuthConfig        `toml:"auth"`
    Validation  ValidationConfig  `toml:"validation"`
}

type LibraryConfig struct {
    Name        string   `toml:"name"`
    Version     string   `toml:"version"`
    Description string   `toml:"description"`
    Author      string   `toml:"author"`
}

type SourceConfig struct {
    URL         string   `toml:"url"`          // Remote URL
    File        string   `toml:"file"`         // Local file path
    Spec        string   `toml:"spec"`         // Inline spec (YAML/JSON)
}

type EndpointConfig struct {
    Path        string   `toml:"path"`         // "/users"
    Method      string   `toml:"method"`       // "GET", "POST", etc.
    Function    string   `toml:"function"`     // "get_users"
    Include     bool     `toml:"include"`      // true to include
}

type OutputConfig struct {
    Directory   string   `toml:"directory"`    // Output directory
    Package     string   `toml:"package"`      // Package name
    Format      bool     `toml:"format"`       // Format output code
}

type AuthConfig struct {
    Type        string   `toml:"type"`         // "none", "apikey", "oauth2", "bearer"
    HeaderName  string   `toml:"header_name"`  // For API key
    TokenURL    string   `toml:"token_url"`    // For OAuth2
}

type ValidationConfig struct {
    Enabled     bool     `toml:"enabled"`
    Strict      bool     `toml:"strict"`       // Fail on warnings
    Suggest     bool     `toml:"suggest"`      // Suggest fixes
}

// LoadConfig loads configuration from TOML file
func LoadConfig(path string) (*Config, error)

// LoadConfigFromEnv loads configuration from environment
func LoadConfigFromEnv() (*Config, error)

// Validate validates the configuration
func (c *Config) Validate() error
```

### 3. `tools/openapi-generator/openapi/openapi.go` (NEW)

**Purpose:** OpenAPI specification models

```go
package openapi

// OpenAPISpec represents an OpenAPI 3.x specification
type OpenAPISpec struct {
    OpenAPI    string                 `yaml:"openapi"`
    Info       Info                   `yaml:"info"`
    Servers    []Server               `yaml:"servers"`
    Paths      map[string]PathItem    `yaml:"paths"`
    Components Components             `yaml:"components"`
    Security   []SecurityRequirement  `yaml:"security"`
}

type Info struct {
    Title       string `yaml:"title"`
    Version     string `yaml:"version"`
    Description string `yaml:"description"`
}

type Server struct {
    URL         string `yaml:"url"`
    Description string `yaml:"description"`
}

type PathItem struct {
    Ref         string     `yaml:"$ref"`
    Summary     string     `yaml:"summary"`
    Description string     `yaml:"description"`
    Get         *Operation `yaml:"get"`
    Put         *Operation `yaml:"put"`
    Post        *Operation `yaml:"post"`
    Delete      *Operation `yaml:"delete"`
    Options     *Operation `yaml:"options"`
    Head        *Operation `yaml:"head"`
    Patch       *Operation `yaml:"patch"`
    Trace       *Operation `yaml:"trace"`
}

type Operation struct {
    Tags        []string            `yaml:"tags"`
    Summary     string              `yaml:"summary"`
    Description string              `yaml:"description"`
    OperationID string              `yaml:"operationId"`
    Parameters  []Parameter         `yaml:"parameters"`
    RequestBody *RequestBody        `yaml:"requestBody"`
    Responses   map[string]Response `yaml:"responses"`
    Security    []map[string][]string `yaml:"security"`
}

type Parameter struct {
    Name            string      `yaml:"name"`
    In              string      `yaml:"in"` // "query", "header", "path", "cookie"
    Description     string      `yaml:"description"`
    Required        bool        `yaml:"required"`
    Deprecated      bool        `yaml:"deprecated"`
    AllowEmptyValue bool        `yaml:"allowEmptyValue"`
    Schema          *Schema     `yaml:"schema"`
}

type RequestBody struct {
    Description string             `yaml:"description"`
    Required    bool               `yaml:"required"`
    Content     map[string]MediaType `yaml:"content"`
}

type Response struct {
    Description string             `yaml:"description"`
    Content     map[string]MediaType `yaml:"content"`
}

type MediaType struct {
    Schema *Schema `yaml:"schema"`
}

type Schema struct {
    Type             string             `yaml:"type"`
    Format           string             `yaml:"format"`
    Description      string             `yaml:"description"`
    Properties       map[string]*Schema `yaml:"properties"`
    Required         []string           `yaml:"required"`
    Items            *Schema            `yaml:"items"`
    Ref              string             `yaml:"$ref"`
    Enum             []interface{}      `yaml:"enum"`
}

type Components struct {
    Schemas         map[string]Schema         `yaml:"schemas"`
    SecuritySchemes map[string]SecurityScheme `yaml:"securitySchemes"`
}

type SecurityScheme struct {
    Type         string `yaml:"type"` // "apiKey", "http", "oauth2", "openIdConnect"
    Description  string `yaml:"description"`
    Name         string `yaml:"name"`
    In           string `yaml:"in"` // "header", "query", "cookie"
    Scheme       string `yaml:"scheme"` // "basic", "bearer"
    BearerFormat string `yaml:"bearerFormat"`
    Flows        map[string]OAuthFlow `yaml:"flows"`
}

type OAuthFlow struct {
    AuthorizationUrl string `yaml:"authorizationUrl"`
    TokenUrl         string `yaml:"tokenUrl"`
    RefreshUrl       string `yaml:"refreshUrl"`
    Scopes           map[string]string `yaml:"scopes"`
}
```

### 4. `tools/openapi-generator/openapi/parser.go` (NEW)

**Purpose:** Parse OpenAPI specs

```go
package openapi

import (
    "fmt"
    "io"
    "os"
    "gopkg.in/yaml.v3"
    "encoding/json"
)

// Parse parses an OpenAPI spec from a reader
func Parse(r io.Reader) (*OpenAPISpec, error) {
    // Detect format (YAML vs JSON)
    // Parse into OpenAPISpec struct
    // Validate required fields
}

// ParseFile parses an OpenAPI spec from a file
func ParseFile(path string) (*OpenAPISpec, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    return Parse(file)
}

// ParseURL parses an OpenAPI spec from a URL
func ParseURL(url string) (*OpenAPISpec, error) {
    // Fetch URL
    // Parse response
    return nil, nil
}

// Validate validates the OpenAPI spec
func (spec *OpenAPISpec) Validate() error {
    // Check required fields
    // Check version (3.x only)
    // Validate paths and operations
    return nil
}
```

### 5. `tools/openapi-generator/generator/generator.go` (NEW)

**Purpose:** Main generator orchestration

```go
package generator

import (
    "fmt"
    "os"
    "github.com/paularlott/scriptling/tools/openapi-generator/openapi"
    "github.com/paularlott/scriptling/tools/openapi-generator/config"
    "github.com/paularlott/scriptling/tools/openapi-generator/output"
)

type Generator struct {
    config *config.Config
    spec   *openapi.OpenAPISpec
}

func New(cfg *config.Config) (*Generator, error) {
    // Load OpenAPI spec
    // Validate spec
    // Validate config
    return &Generator{
        config: cfg,
        spec:   spec,
    }, nil
}

func (g *Generator) Generate() error {
    // Generate library file
    if err := g.generateLibrary(); err != nil {
        return err
    }

    // Generate models (optional)
    if g.config.Output.Models {
        if err := g.generateModels(); err != nil {
            return err
        }
    }

    // Validate generated code
    if g.config.Validation.Enabled {
        if err := g.validateGenerated(); err != nil {
            return err
        }
    }

    return nil
}

func (g *Generator) generateLibrary() error {
    // Build library structure
    // Generate functions for each endpoint
    // Write to output directory
    return nil
}

func (g *Generator) generateModels() error {
    // Generate model classes for schemas
    // Write to models/ subdirectory
    return nil
}

func (g *Generator) validateGenerated() error {
    // Use Phase 2 validator
    // Check syntax and structure
    // Report errors
    return nil
}
```

### 6. `tools/openapi-generator/generator/builder.go` (NEW)

**Purpose:** Build Scriptling code

```go
package generator

import (
    "fmt"
    "strings"
    "github.com/paularlott/scriptling/tools/openapi-generator/openapi"
)

// Builder builds Scriptling code from OpenAPI operations
type Builder struct {
    spec   *openapi.OpenAPISpec
    config *config.Config
}

func NewBuilder(spec *openapi.OpenAPISpec, config *config.Config) *Builder {
    return &Builder{
        spec:   spec,
        config: config,
    }
}

// BuildFunction generates a Scriptling function for an operation
func (b *Builder) BuildFunction(path string, method string, op *openapi.Operation) string {
    funcName := b.getFunctionName(op)
    params := b.buildParameters(op.Parameters)
    body := b.buildBody(op.RequestBody)
    validation := b.buildValidation(op.Parameters, op.RequestBody)
    request := b.buildRequest(path, method, op)
    response := b.buildResponse(op.Responses)

    return fmt.Sprintf(`
def %s(%s):
    """%s"

    %s

    %s

    %s

    %s
`,
        funcName,
        params,
        b.buildDocstring(op),
        validation,
        body,
        request,
        response,
    )
}

func (b *Builder) getFunctionName(op *openapi.Operation) string {
    if op.OperationID != "" {
        return strings.ToLower(op.OperationID)
    }
    // Generate from path and method
    return b.generateFunctionName(path, method)
}

func (b *Builder) buildParameters(params []openapi.Parameter) string {
    // Build **kwargs signature with documentation
    parts := []string{}
    for _, p := range params {
        required := ""
        if !p.Required {
            required = "=None"
        }
        parts = append(parts, fmt.Sprintf("%s%s", p.Name, required))
    }
    return "**kwargs"
}

func (b *Builder) buildValidation(params []openapi.Parameter, body *openapi.RequestBody) string {
    // Use Phase 1 validation helpers
    // Generate RequireKwargs calls
    // Generate ValidateKwargsTypes calls
    return `
    # Validate required parameters
    if not kwargs.Has("name"):
        return Error("Missing required field: name")
`
}

func (b *Builder) buildDocstring(op *openapi.Operation) string {
    doc := []string{op.Summary, ""}

    // Parameters
    if len(op.Parameters) > 0 {
        doc = append(doc, "Required:")
        for _, p := range params {
            if p.Required {
                doc = append(doc, fmt.Sprintf("    %s (%s): %s",
                    p.Name, p.Schema.Type, p.Description))
            }
        }

        doc = append(doc, "Optional:")
        for _, p := range params {
            if !p.Required {
                doc = append(doc, fmt.Sprintf("    %s (%s): %s",
                    p.Name, p.Schema.Type, p.Description))
            }
        }
    }

    // Returns
    doc = append(doc, "\nReturns:")
    doc = append(doc, "    dict: Response data")

    return strings.Join(doc, "\n")
}

func (b *Builder) buildRequest(path, method string, op *openapi.Operation) string {
    // Build HTTP request code
    return fmt.Sprintf(`
    url = f"{self.base_url}%s"
    headers = {}
    if self.auth:
        headers = self.auth.get_headers()

    response = http.%s(url, headers=headers, json=kwargs)
`, path, strings.ToLower(method))
}

func (b *Builder) buildResponse(responses map[string]openapi.Response) string {
    // Build response handling
    return `
    if response.status_code >= 400:
        return Error(f"API error: {response.status_code}")

    return response.json()
`
}
```

### 7. `extlibs/auth/auth.go` (NEW)

**Purpose:** Authentication support library

```go
package auth

import (
    "context"
    "fmt"
    "github.com/paularlott/scriptling/object"
)

// AuthLibrary provides authentication functionality
var AuthLibrary = object.NewLibrary(
    map[string]*object.Builtin{
        "OAuth2Client": {
            Fn:         newOAuth2Client,
            HelpText:   "OAuth2Client(client_id, client_secret, token_url) - OAuth2 client",
        },
        "APIKeyAuth": {
            Fn:         newAPIKeyAuth,
            HelpText:   "APIKeyAuth(api_key, header_name) - API key authentication",
        },
        "BearerAuth": {
            Fn:         newBearerAuth,
            HelpText:   "BearerAuth(access_token) - Bearer token authentication",
        },
    },
    nil,
    "Authentication support library for API clients",
)

func newOAuth2Client(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Create OAuth2 client instance
    // ...
}

func newAPIKeyAuth(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Create API key auth instance
    // ...
}

func newBearerAuth(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Create bearer auth instance
    // ...
}
```

## Implementation Steps

### Phase 3.1: Core Generator (MVP)

**Tasks:**
- [ ] Set up project structure
- [ ] Implement OpenAPI parser (YAML/JSON)
- [ ] Implement basic generator
- [ ] Generate simple GET endpoints
- [ ] Implement auth library (API key only)
- [ ] Generate basic client class
- [ ] Write unit tests
- [ ] Test with Petstore OpenAPI spec

### Phase 3.2: Enhanced Generation

**Tasks:**
- [ ] Support POST/PUT/PATCH with request bodies
- [ ] Support query parameters
- [ ] Support path parameters
- [ ] Generate proper validation (using Phase 1)
- [ ] Generate comprehensive docstrings
- [ ] Support OAuth2 authentication
- [ ] Support Bearer token authentication
- [ ] Generate error handling
- [ ] Test with real-world APIs (GitHub, Stripe, etc.)

### Phase 3.3: CLI & Configuration

**Tasks:**
- [ ] Implement CLI interface
- [ ] Support TOML configuration files
- [ ] Support environment variables
- [ ] Add interactive mode (select endpoints)
- [ ] Add `--all` flag to generate all endpoints
- [ ] Add `--validate` flag to validate output
- [ ] Add `--dry-run` flag to preview generation
- [ ] Documentation and examples

### Phase 3.4: Validation Integration

**Tasks:**
- [ ] Integrate Phase 2 validator
- [ ] Auto-validate generated code
- [ ] Report validation errors
- [ ] Support `--strict` mode
- [ ] Fix common validation issues in generator

### Phase 3.5: Advanced Features (Optional)

**Tasks:**
- [ ] Generate model classes for schemas
- [ ] Support response unpacking into typed objects
- [ ] Support enums and constants
- [ ] Support webhooks
- [ ] Support file uploads
- [ ] Support streaming responses
- [ ] Generate tests for client
- [ ] Support custom templates

## Usage Examples

### CLI Usage

```bash
# Interactive mode - select endpoints
openapi-gen generate petstore.yaml --interactive

# Generate specific endpoints
openapi-gen generate petstore.yaml \
  --endpoints GET:/pets,POST:/pets \
  --output extlibs/petstore/

# Generate all endpoints
openapi-gen generate petstore.yaml --all

# Using config file
openapi-gen generate --config petstore.toml

# Validate mode (no generation)
openapi-gen validate petstore.yaml

# Dry run (preview what will be generated)
openapi-gen generate petstore.yaml --dry-run

# With validation
openapi-gen generate petstore.yaml --validate --strict
```

### Config File Example

```toml
# petstore.toml

[library]
name = "petstore"
version = "1.0.0"
description = "Petstore API Client"
author = "Your Name"

[source]
url = "https://petstore.example.com/openapi.yaml"
# or
file = "./specs/petstore.yaml"

[[endpoints]]
path = "/pets"
method = "GET"
function = "get_pets"
include = true

[[endpoints]]
path = "/pets"
method = "POST"
function = "create_pet"
include = true

[output]
directory = "extlibs/petstore"
package = "petstore"
format = true

[auth]
type = "apikey"
header_name = "X-API-Key"

[validation]
enabled = true
strict = false
suggest = true
```

### Generated Library Example

```python
# extlibs/petstore/petstore.py

"""
Petstore API Client Library
Generated from: https://petstore.example.com/openapi.yaml
Version: 1.0.0
"""

from http import http
from auth import APIKeyAuth

class PetstoreAPI:
    """Petstore API client"""

    def __init__(self, base_url=None, api_key=None):
        """Initialize API client

        Required:
            base_url (str): API base URL
        Optional:
            api_key (str): API key for authentication
        """
        self.base_url = base_url or "https://petstore.example.com"
        self.auth = None

        if api_key:
            self.auth = APIKeyAuth(api_key, "X-API-Key")

    def get_pets(self, **kwargs):
        """List all pets

        Optional:
            limit (int): How many items to return at one time (max 100)
            page (int): Page number for pagination

        Returns:
            list: List of pets
        """
        # Build query parameters
        params = {}
        if kwargs.Has("limit"):
            params["limit"] = kwargs.Get("limit")
        if kwargs.Has("page"):
            params["page"] = kwargs.Get("page")

        # Make request
        url = f"{self.base_url}/pets"
        headers = {}
        if self.auth:
            headers = self.auth.get_headers()

        response = http.get(url, headers=headers, params=params)

        # Handle errors
        if response.status_code >= 400:
            return Error(f"API error: {response.status_code}: {response.text}")

        # Return unpacked response
        return response.json()

    def create_pet(self, **kwargs):
        """Create a new pet

        Required:
            name (str): Pet name
            photo_urls (list): List of photo URLs

        Optional:
            id (int): Pet ID
            status (str): Pet status (available, pending, sold)
            category (dict): Category object

        Returns:
            dict: Created pet object
        """
        # Validate required fields
        if not kwargs.Has("name"):
            return Error("Missing required field: name")
        if not kwargs.Has("photo_urls"):
            return Error("Missing required field: photo_urls")

        # Build request data
        data = {}
        for key in kwargs.Keys():
            data[key] = kwargs.Get(key)

        # Make request
        url = f"{self.base_url}/pets"
        headers = {}
        if self.auth:
            headers = self.auth.get_headers()

        response = http.post(url, json=data, headers=headers)

        # Handle errors
        if response.status_code >= 400:
            return Error(f"API error: {response.status_code}: {response.text}")

        # Return unpacked response
        return response.json()
```

## Testing Strategy

### Unit Tests
- [ ] Test OpenAPI parser with various specs
- [ ] Test config loading and validation
- [ ] Test code generation for each operation type
- [ ] Test auth library functions
- [ ] Test validation integration

### Integration Tests
- [ ] Generate library from Petstore spec
- [ ] Generate library from GitHub API spec
- [ ] Test generated library with actual API calls
- [ ] Test validation catches errors
- [ ] Test CLI with various flags

### Validation Tests
- [ ] Ensure generated code passes syntax validation
- [ ] Ensure generated code passes structure validation
- [ ] Test with --strict mode
- [ ] Test error reporting

## Success Criteria

- [ ] Can parse OpenAPI 3.x specs (YAML/JSON)
- [ ] Generates working API client libraries
- [ ] Supports GET, POST, PUT, PATCH, DELETE operations
- [ ] Validates all generated code (Phase 2)
- [ ] Uses typed kwargs validation (Phase 1)
- [ ] Supports API key, OAuth2, Bearer auth
- [ ] CLI works with config files and flags
- [ ] 80%+ test coverage
- [ ] Tested with 3+ real OpenAPI specs
- [ ] Documentation complete with examples

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| OpenAPI spec variations | High | Test with many specs, handle edge cases |
| Generated code quality | High | Use Phase 2 validator, iterative testing |
| Complex auth schemes | Medium | Start with simple, add complex over time |
| Template limitations | Medium | Make templates flexible, allow customization |
| Breaking changes in OpenAPI | Low | Support 3.x only, clear error messages |

## Open Questions

1. **Template customization:** Should users be able to provide custom templates?
   - **Recommendation:** Yes, in Phase 3.5, add `--templates` flag

2. **Model generation:** Should we generate model classes for schemas?
   - **Recommendation:** Optional in Phase 3.5, start with dicts

3. **Pagination:** Should generator detect and handle pagination patterns?
   - **Recommendation:** Phase 3.5, add hints via config

4. **Rate limiting:** Should generated clients handle rate limiting?
   - **Recommendation:** No, keep simple, users can add middleware

## Timeline Estimate

- **Phase 3.1 (MVP):** 2 weeks
- **Phase 3.2 (Enhanced):** 2 weeks
- **Phase 3.3 (CLI):** 1 week
- **Phase 3.4 (Validation):** 1 week
- **Phase 3.5 (Advanced):** 2-3 weeks
- **Total (MVP):** 6 weeks
- **Total (Complete):** 8-9 weeks

## Next Steps

After completing this phase:
1. Use generator to create libraries for popular APIs
2. Collect user feedback
3. Iterate on features and templates
4. Consider additional generators (GraphQL, gRPC, etc.)

## References

- OpenAPI 3.0 Specification: https://spec.openapis.org/oas/v3.0.0
- Petstore OpenAPI Example: https://petstore.swagger.io/
- GitHub REST API OpenAPI: https://github.com/github/rest-api-description
- Stripe OpenAPI: https://github.com/stripe/openapi
- Phase 1: Typed Kwargs Validation
- Phase 2: Validator Library & CLI Integration
