# CallMethod and CreateInstance Example

This example demonstrates how to create instances of Scriptling classes from Go and call methods on them using the new `CreateInstance`, `CallMethod`, and `CallMethodWithContext` APIs.

## Features Demonstrated

1. **CreateInstance** - Create instances of Scriptling classes from Go
2. **CallMethod** - Call methods on Scriptling instances from Go
3. **Kwargs support** - Pass keyword arguments to methods
4. **Multiple instances** - Create and manage multiple independent instances
5. **Bidirectional integration** - Store instances in the environment and use them from both Go and Scriptling

## Running the Example

```bash
go run main.go
```

## Key Concepts

### Creating Instances

```go
// Create an instance with positional arguments
instance, err := p.CreateInstance("Counter", 100)

// Create an instance with kwargs
account, err := p.CreateInstance("BankAccount", "Alice", scriptling.Kwargs{"balance": 1000})
```

### Calling Methods

```go
// Call a method with no arguments
result, err := p.CallMethod(instance, "get")

// Call a method with positional arguments
result, err := p.CallMethod(instance, "increment", 10)

// Call a method with kwargs
result, err := p.CallMethod(instance, "increment", scriptling.Kwargs{"amount": 10})
```

### Extracting Results

```go
// Extract different types from results
intVal, _ := result.AsInt()
strVal, _ := result.AsString()
floatVal, _ := result.AsFloat()
boolVal, _ := result.AsBool()
```

### Storing Instances

```go
// Store instance in environment for use in scripts
p.SetObjectVar("counter", instance)

// Now use it from Scriptling
p.Eval(`
counter.increment(25)
value = counter.get()
`)
```

## Use Cases

- **Stateful operations** - Maintain state across multiple Go function calls
- **Object-oriented integration** - Work with Scriptling classes naturally from Go
- **Plugin systems** - Load and instantiate Scriptling classes dynamically
- **Testing** - Create test fixtures and verify behavior from Go tests
- **Hybrid applications** - Mix Go and Scriptling code seamlessly
