# Scriptling Development Plans

This directory contains development plans for enhancing Scriptling with OpenAPI support and validation capabilities.

## Overview

These plans outline a three-phase approach to add:

1. **Phase 1:** Enhanced kwargs validation for type-safe function parameters
2. **Phase 2:** Validator library and CLI integration for code validation
3. **Phase 3:** OpenAPI to Scriptling library generator

## Goals

- Enable automatic generation of type-safe API client libraries from OpenAPI specs
- Provide robust validation tools for Scriptling code
- Maintain backward compatibility while adding new features
- Follow Scriptling's existing patterns and conventions

## Phase Summaries

### [Phase 1: Typed Kwargs Validation](./01-typed-kwargs-validation.md)

**Status:** Planning
**Complexity:** Low
**Estimated Time:** 4-6 hours

Add validation helpers to Scriptling's core to support robust parameter validation. This provides the foundation for type-safe keyword argument handling in generated libraries.

**Key Deliverables:**
- `errors/validation.go` - Validation helper functions
- `errors/validation_test.go` - Comprehensive tests
- Enhanced error messages for parameter validation

**Dependencies:** None

### [Phase 2: Validator Library & CLI Integration](./02-validator-library-and-cli.md)

**Status:** Planning
**Complexity:** Medium-High
**Estimated Time:** 4-6 weeks

Build a comprehensive validator library that can be used both as a standalone Go package and integrated into `scriptling-cli`. The validator checks Scriptling code for syntax errors, structural issues, and provides detailed feedback with line numbers.

**Key Deliverables:**
- `validator/` package - Reusable validation library
- `scriptling validate` command - CLI integration
- Syntax and structure validation
- JSON and text output formats
- Integration with generated libraries

**Dependencies:** Phase 1

### [Phase 3: OpenAPI to Scriptling Generator](./03-openapi-to-scriptling-generator.md)

**Status:** Planning
**Complexity:** High
**Estimated Time:** 8-9 weeks

Build a standalone tool that converts OpenAPI 3.x specifications into Scriptling library code. The tool generates type-safe, documented, production-ready API client libraries.

**Key Deliverables:**
- `tools/openapi-gen/` - Generator tool
- `extlibs/auth/` - Authentication support library
- CLI and configuration file support
- Support for API key, OAuth2, Bearer token authentication
- Automatic validation of generated code

**Dependencies:** Phase 1, Phase 2

## Implementation Order

Follow phases in order (1 → 2 → 3) as each builds on the previous:

```
Phase 1: Typed Kwargs (Foundation)
    ↓
Phase 2: Validator (Quality Assurance)
    ↓
Phase 3: OpenAPI Generator (Application)
```

## Quick Start

### Phase 1 - Add validation helpers
```bash
# Create validation.go
touch errors/validation.go

# Implement validation functions
# - RequireKwargs
# - ValidateKwargsType
# - ValidateKwargsRange
# - etc.

# Add tests
touch errors/validation_test.go
```

### Phase 2 - Build validator
```bash
# Create validator package
mkdir validator

# Implement validator
# - Syntax validation
# - Structure validation
# - CLI integration

# Test with sample scripts
scriptling validate myscript.py
```

### Phase 3 - Generate API clients
```bash
# Build generator
cd tools/openapi-gen

# Generate library from OpenAPI spec
openapi-gen generate petstore.yaml --output extlibs/petstore/

# Use generated library
scriptling
>>> import petstore
>>> api = petstore.PetstoreAPI(api_key="xxx")
>>> pets = api.get_pets(limit=10)
```

## Usage Example (After All Phases)

```bash
# 1. Generate API client from OpenAPI spec
openapi-gen generate https://api.example.com/openapi.yaml \
  --output extlibs/myapi/ \
  --validate

# 2. Validate the generated library
scriptling validate extlibs/myapi/myapi.py

# 3. Use the library in Scriptling
scriptling
>>> import myapi
>>> api = myapi.MyAPI(api_key="xxx")
>>> result = api.get_user(user_id=123)
>>> print(result)
```

## Testing Strategy

Each phase includes comprehensive testing:

- **Unit tests** for all new components
- **Integration tests** for CLI and tools
- **End-to-end tests** with real OpenAPI specs
- **Validation tests** using Phase 2 validator

## Contributing

When implementing these plans:

1. Follow the implementation order (1 → 2 → 3)
2. Complete each phase fully before moving to the next
3. Ensure all tests pass before marking a phase complete
4. Update documentation as you go
5. Test with real-world scenarios

## Questions or Feedback?

If you have questions about these plans or suggestions for improvements, please:

1. Review the individual plan documents for detailed information
2. Check the "Open Questions" section in each plan
3. Consider creating issues for discussion

## Progress Tracking

Current status of all phases:

- [ ] Phase 1: Typed Kwargs Validation
- [ ] Phase 2: Validator Library & CLI Integration
- [ ] Phase 3: OpenAPI to Scriptling Generator

---

**Last Updated:** 2025-01-17
**Document Version:** 1.0
