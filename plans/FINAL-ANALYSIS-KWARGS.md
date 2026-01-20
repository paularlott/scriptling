# CORRECTED: Python 3 Compatible **kwargs for Scriptling

## You're Absolutely Right!

**Python 3 **kwargs syntax:**

```python
# Standard Python 3
def myfunc(**kwargs):
    print(kwargs['abc'])
    print(kwargs.get('xyz', 'default'))

myfunc(abc=3, xyz=10)
# Output: 3, 10
```

**kwargs is just a DICT, accessed with standard dict methods!**

---

## What Scriptling Already Has

✅ **Dict indexing:** `kwargs['name']`
✅ **Dict .get() method:** `kwargs.get('name', 'default')`
✅ **Dict .keys() method:** `kwargs.keys()`
✅ **Dict .values() method:** `kwargs.values()`
✅ **Dict .items() method:** `kwargs.items()`
✅ **'in' operator:** `'name' in kwargs`

**We just need to add the `**kwargs` syntax to function definitions!**

---

## The Implementation (Much Simpler!)

### What We Need to Add

**ONLY the `**kwargs` syntax in function definitions.**

Everything else (dict access, .get(), etc.) already works!

### AST Changes

```go
// ast/ast.go
type Function struct {
    Name          string
    Parameters    []*Identifier
    DefaultValues map[string]Expression
    Variadic      *Identifier  // *args
    KwargParam    *Identifier  // **kwargs (NEW!)
}
```

### Parser Changes

```go
// parser/parser.go - parseFunctionParameters()
func (p *Parser) parseFunctionParameters() {
    // ... existing code ...

    // Check for **kwargs (double asterisk)
    if p.curTokenIs(token.ASTERISK) && p.peekTokenIs(token.ASTERISK) {
        p.nextToken() // consume first *
        p.nextToken() // consume second *
        if !p.expectPeek(token.IDENT) {
            return nil, nil, nil, nil  // Error
        }
        kwargParam = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
        if !p.expectPeek(token.RPAREN) {
            return nil, nil, nil, nil  // Error
        }
        return identifiers, defaults, variadic, kwargParam
    }

    // ... rest of existing code ...
}
```

### Evaluator Changes

```go
// evaluator/evaluator.go - evalCallExpression()
func evalCallExpression(ctx context.Context, node *ast.CallExpression, env *object.Environment) object.Object {
    // ... existing code to get function ...

    // Build kwargs dict from keyword arguments in call
    kwargDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
    for key, valueExpr := range node.Keywords {
        val := evalWithContext(ctx, valueExpr, env)
        kwargDict.Pairs[key] = object.DictPair{
            Key:   &object.String{Value: key},
            Value: val,
        }
    }

    // If function has **kwargs parameter, set it in environment
    if fn.KwargParam != nil {
        env.Set(fn.KwargParam.Value, kwargDict)
    }

    // ... rest of existing code ...
}
```

---

## Example: Generated OpenAPI Library

### What We'll Generate (Python 3 Compatible!)

```python
# extlibs/petstore/petstore.py (GENERATED)

from http import http

class PetstoreAPI:
    """Petstore API Client"""

    def __init__(self, base_url=None, api_key=None):
        self.base_url = base_url or "https://petstore.example.com"
        self.api_key = api_key

    def get_pets(self, **kwargs):
        """List all pets

        Optional:
            limit (int): Max 100, default 10
            page (int): Page number, default 1
            status (str): available, pending, or sold

        Returns:
            list: List of pets
        """
        # Validate limit (if provided)
        if 'limit' in kwargs:
            limit = kwargs.get('limit', 10)
            if limit.Type() != 'INTEGER':
                return Error("'limit' must be an integer")
            if limit.Value > 100:
                return Error("'limit' must be <= 100")

        # Build query parameters from kwargs
        params = {}
        for key in kwargs.keys():
            params[key] = kwargs[key]

        # Make HTTP request
        url = f"{self.base_url}/pets"
        headers = {}
        if self.api_key:
            headers['X-API-Key'] = self.api_key

        response = http.get(url, headers=headers, params=params)

        # Handle errors
        if response.status_code >= 400:
            return Error(f"API error: {response.status_code}")

        # Return response as dict
        return response.json()

    def create_pet(self, **kwargs):
        """Create a new pet

        Required:
            name (str): Pet name
            photo_urls (list): List of photo URLs

        Optional:
            id (int): Pet ID
            status (str): Pet status
            age (int): Pet age (0-30)

        Returns:
            dict: Created pet object
        """
        # Validate required fields
        if 'name' not in kwargs:
            return Error("Missing required field: 'name'")
        if 'photo_urls' not in kwargs:
            return Error("Missing required field: 'photo_urls'")

        # Validate types
        name = kwargs.get('name')
        if name.Type() != 'STRING':
            return Error("'name' must be a string")

        photo_urls = kwargs.get('photo_urls')
        if photo_urls.Type() != 'LIST':
            return Error("'photo_urls' must be a list")

        # Validate optional age range
        if 'age' in kwargs:
            age = kwargs.get('age')
            if age.Type() != 'INTEGER':
                return Error("'age' must be an integer")
            if age.Value < 0 or age.Value > 30:
                return Error("'age' must be between 0 and 30")

        # Make request
        url = f"{self.base_url}/pets"
        response = http.post(url, json=kwargs, headers=self._get_headers())

        # Handle errors
        if response.status_code >= 400:
            return Error(f"API error: {response.status_code}")

        return response.json()
```

### User Code (Clean Python 3 Syntax!)

```python
# user_script.py
import petstore

# Create API client
api = petstore.PetstoreAPI(api_key="xxx")

# Call with keyword arguments (Python 3 style!)
pets = api.get_pets(limit=10, status="available", tags=["dog", "cat"])

# Create a pet
pet = api.create_pet(
    name="Fluffy",
    photo_urls=["https://example.com/fluffy.jpg"],
    age=3,
    status="available"
)

print(pets)
print(pet)
```

---

## Key Validation Helpers (Simple!)

**NO special Kwargs class needed!** Just use standard dict methods:

### Required Fields Check

```python
def create_pet(**kwargs):
    # Required fields
    if 'name' not in kwargs:
        return Error("Missing required field: 'name'")
```

### Type Validation

```python
def create_pet(**kwargs):
    # Type check
    name = kwargs.get('name')
    if name.Type() != 'STRING':
        return Error("'name' must be a string")
```

### Range Validation

```python
def create_pet(**kwargs):
    # Range check
    if 'age' in kwargs:
        age = kwargs.get('age')
        if age.Value < 0 or age.Value > 30:
            return Error("'age' must be between 0 and 30")
```

### With Defaults

```python
def get_pets(**kwargs):
    # With default
    limit = kwargs.get('limit', 10)  # Default to 10
```

### Enum Validation

```python
def get_pets(**kwargs):
    # Enum check
    if 'status' in kwargs:
        status = kwargs.get('status')
        valid = ['available', 'pending', 'sold']
        if status.Value not in valid:
            return Error(f"'status' must be one of: {', '.join(valid)}")
```

---

## Complete Implementation Plan

### Phase 1: Add **kwargs Support (1-2 days)

**Files to modify:**

1. **ast/ast.go** (5 minutes)
   - Add `KwargParam *Identifier` to Function struct

2. **parser/parser.go** (2-3 hours)
   - Add `**kwargs` parsing in `parseFunctionParameters()`
   - Handle `*args, **kwargs` combination
   - Handle `def func(a, b, **kwargs)` pattern
   - Error handling for invalid syntax

3. **evaluator/evaluator.go** (2-3 hours)
   - Build kwargs dict in `evalCallExpression()`
   - Set kwargs in environment for function
   - Handle edge cases

4. **tests/** (2-3 hours)
   - Test basic **kwargs
   - Test *args, **kwargs combination
   - Test named params + **kwargs
   - Test nested calls
   - Test error cases

5. **docs/** (1 hour)
   - Update LANGUAGE_GUIDE.md
   - Add examples
   - Document differences from Python 3 (if any)

**Total:** 1-2 days

---

### Phase 2: OpenAPI Generator (2-3 weeks)

**No extra helpers needed!** Generate code that uses:
- `kwargs.get(key, default)`
- `kwargs.keys()`
- `'key' in kwargs`
- `kwargs[key]`

**Templates will be clean and Python 3 compatible.**

---

## Minimal Changes - Maximum Compatibility

### What We're NOT Adding

❌ No special Kwargs class
❌ No kwargs.Has() method
❌ No kwargs.GetString() method
❌ No helper libraries
❌ No new dict methods

### What We ARE Adding

✅ Only `**kwargs` syntax in function definitions
✅ kwargs is just a standard dict
✅ Use existing dict methods (.get(), .keys(), etc.)
✅ Full Python 3 compatibility

---

## Updated Plan Files

Need to update:

1. **01-typed-kwargs-validation.md** → **01-add-kwargs-support.md**
   - Remove validation helpers (not needed)
   - Focus on adding **kwargs syntax only
   - 1-2 days, not 4-6 hours

2. **02-validator-library-and-cli.md** → (unchanged)

3. **03-openapi-to-scriptling-generator.md**
   - Remove Kwargs class
   - Use standard dict methods
   - Simpler templates

---

## Summary

**You were right to question my approach!**

The solution is MUCH simpler:
1. Add `**kwargs` syntax to function definitions (1-2 days)
2. kwargs is just a dict (already supported!)
3. Use standard dict methods (.get(), .keys(), 'in', etc.)
4. No special helper classes needed
5. Full Python 3 compatibility

**Generated code will be clean, simple, and Python 3 compatible:**

```python
def get_pets(self, **kwargs):
    if 'limit' in kwargs:
        limit = kwargs.get('limit', 10)
        if limit.Value > 100:
            return Error("'limit' must be <= 100")
```

Much better! Thank you for catching that. 🎉
