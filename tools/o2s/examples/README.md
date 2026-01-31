# OpenAPI Examples

This directory contains example OpenAPI specifications for testing o2s.

## Files

### Single-File Examples
- **petstore.json** - Complete API spec with internal $ref (JSON)
- **petstore.yaml** - Complete API spec with internal $ref (YAML)

### Multi-File Examples
- **petstore-multifile.json** - Main spec with external $ref to components.json
- **petstore-multifile.yaml** - Main spec with external $ref to components.yaml
- **components.json** - Shared components (parameters, schemas, requestBodies)
- **components.yaml** - Shared components in YAML format

## $ref Support

o2s supports both internal and external $ref resolution:

### Internal References
```json
{
  "parameters": [{
    "$ref": "#/components/parameters/PetIdParam"
  }]
}
```

### External File References
```json
{
  "parameters": [{
    "$ref": "components.json#/parameters/PetIdParam"
  }]
}
```

```yaml
parameters:
  - $ref: 'components.yaml#/parameters/PetIdParam'
```

## Testing

```bash
# Single-file with internal $ref
scriptling o2s.py -- petstore.json --generate --output /tmp/petstore

# Multi-file with external $ref (JSON)
scriptling o2s.py -- petstore-multifile.json --generate --output /tmp/multifile

# Multi-file with external $ref (YAML)
scriptling o2s.py -- petstore-multifile.yaml --generate --output /tmp/multifile_yaml
```

All examples include:
- Security definitions (Bearer JWT)
- Path parameters
- Query parameters
- Request bodies
- Multiple HTTP methods (GET, POST, PUT, DELETE)
