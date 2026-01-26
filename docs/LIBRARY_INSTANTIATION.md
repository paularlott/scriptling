# Library Instantiation

This guide shows how to create libraries with instance-specific configuration using `LibraryBuilder` and `Instantiate()`.

## Overview

When you need the same library with different configurations across multiple environments (e.g., different allowed paths, API keys, rate limits), use library instantiation:

1. Build a library template once using `LibraryBuilder`
2. Instantiate it multiple times with different configs
3. Functions access config via `object.InstanceDataFromContext(ctx)`

**Key benefit**: Thread-safe - each instance maintains its own config without shared state.

## Simple Libraries (Functions Only)

### Step 1: Define Config Type

```go
type MyConfig struct {
    AllowedPaths []string
    APIKey       string
}
```

### Step 2: Build Template with LibraryBuilder

```go
import "github.com/paularlott/scriptling/object"

builder := object.NewLibraryBuilder("mylib", "My library with instance config")

builder.Function("do_something", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Retrieve config from context
    config := object.InstanceDataFromContext(ctx).(MyConfig)
    
    // Use config
    if !isPathAllowed(config.AllowedPaths, path) {
        return &object.Error{Message: "access denied"}
    }
    
    return &object.String{Value: "success"}
})

template := builder.Build()
```

### Step 3: Instantiate and Register

```go
// Environment 1 - restricted
config1 := MyConfig{
    AllowedPaths: []string{"/tmp"},
    APIKey:       "key1",
}
lib1 := template.Instantiate(config1)

// Environment 2 - broader access
config2 := MyConfig{
    AllowedPaths: []string{"/tmp", "/home/user"},
    APIKey:       "key2",
}
lib2 := template.Instantiate(config2)

// Register to different interpreters
interpreter1.RegisterLibrary(lib1)
interpreter2.RegisterLibrary(lib2)
```

## Libraries with Classes

When your library provides classes, the pattern is:

1. Build template with constructor function
2. Constructor retrieves config from context
3. Constructor stores config in instance fields
4. Class methods retrieve config from instance

### Step 1: Define Config and Class

```go
type FSConfig struct {
    AllowedPaths []string
}

// Define class with methods
var PathClass = &object.Class{
    Name: "Path",
    Methods: map[string]object.Object{
        "exists": &object.Builtin{
            Fn:       pathExists,
            HelpText: "exists() - Check if path exists",
        },
        "joinpath": &object.Builtin{
            Fn:       pathJoinpath,
            HelpText: "joinpath(*other) - Combine path segments",
        },
    },
}
```

Or use ClassBuilder:

```go
classBuilder := object.NewClassBuilder("Path", "Filesystem path object")
classBuilder.Method("exists", pathExists, "exists() - Check if path exists")
classBuilder.Method("joinpath", pathJoinpath, "joinpath(*other) - Combine path segments")
PathClass := classBuilder.Build()
```

### Step 2: Build Library with Constructor

```go
builder := object.NewLibraryBuilder("pathlib", "Filesystem paths with security")

// Constructor function that creates Path instances
builder.Function("Path", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Retrieve config from context (injected by Instantiate)
    config := object.InstanceDataFromContext(ctx).(FSConfig)
    
    // Call helper to create instance
    return createPath(config, ctx, kwargs, args...)
})

template := builder.Build()
```

### Step 3: Implement Constructor Helper

The constructor receives config as parameter and stores it in the instance:

```go
func createPath(config FSConfig, ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    if len(args) < 1 {
        return &object.Error{Message: "Path() requires a path argument"}
    }
    
    pathStr, err := args[0].AsString()
    if err != nil {
        return err
    }
    
    // Validate against config
    if !config.IsPathAllowed(pathStr) {
        return &object.Error{Message: "access denied"}
    }
    
    // Create instance
    instance := &object.Instance{
        Class:  PathClass,
        Fields: make(map[string]object.Object),
    }
    
    // Store config in instance for methods to access
    instance.Fields["__config__"] = config
    instance.Fields["__path__"] = &object.String{Value: pathStr}
    
    return instance
}
```

### Step 4: Implement Class Methods

Methods retrieve config from `this.Fields["__config__"]`:

```go
func pathExists(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    this := args[0].(*object.Instance)
    
    // Retrieve config from instance
    config := this.Fields["__config__"].(FSConfig)
    pathStr := this.Fields["__path__"].(*object.String).Value
    
    // Validate access
    if !config.IsPathAllowed(pathStr) {
        return &object.Error{Message: "access denied"}
    }
    
    // Check if path exists
    _, err := os.Stat(pathStr)
    return &object.Boolean{Value: err == nil}
}

func pathJoinpath(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    this := args[0].(*object.Instance)
    config := this.Fields["__config__"].(FSConfig)
    basePath := this.Fields["__path__"].(*object.String).Value
    
    // Join path segments
    segments := []string{basePath}
    for _, arg := range args[1:] {
        seg, err := arg.AsString()
        if err != nil {
            return err
        }
        segments = append(segments, seg)
    }
    
    newPath := filepath.Join(segments...)
    
    // Create new Path instance with same config
    return createPath(config, ctx, kwargs, []object.Object{&object.String{Value: newPath}})
}
```

### Step 5: Instantiate and Use

```go
// Create configs
config1 := FSConfig{AllowedPaths: []string{"/tmp"}}
config2 := FSConfig{AllowedPaths: []string{"/tmp", "/home/user"}}

// Instantiate
lib1 := template.Instantiate(config1)
lib2 := template.Instantiate(config2)

// Register
interpreter1.RegisterLibrary(lib1)
interpreter2.RegisterLibrary(lib2)

// Use in scripts
interpreter1.Eval(`
import pathlib
p = pathlib.Path("/tmp/file.txt")
if p.exists():
    print("File exists")
`)
```

## Data Flow Summary

### For Functions:
```
User calls function
    ↓
Wrapped function injects config into context
    ↓
Original function retrieves config via InstanceDataFromContext(ctx)
    ↓
Function uses config
```

### For Classes:
```
User calls constructor (e.g., pathlib.Path("/tmp"))
    ↓
Wrapped constructor injects config into context
    ↓
Constructor retrieves config via InstanceDataFromContext(ctx)
    ↓
Constructor stores config in instance.Fields["__config__"]
    ↓
User calls method (e.g., p.exists())
    ↓
Method retrieves config from this.Fields["__config__"]
    ↓
Method uses config
```

## Best Practices

### 1. Type-Safe Config Retrieval

```go
func getConfig(ctx context.Context) (MyConfig, error) {
    data := object.InstanceDataFromContext(ctx)
    if data == nil {
        return MyConfig{}, fmt.Errorf("no instance data")
    }
    config, ok := data.(MyConfig)
    if !ok {
        return MyConfig{}, fmt.Errorf("invalid config type")
    }
    return config, nil
}
```

### 2. Consistent Field Names

- `__config__` - for instance configuration
- `__data__` - for instance data
- Regular names for user-visible fields

### 3. Validate in Constructor

```go
func createPath(config FSConfig, ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Validate config
    if len(config.AllowedPaths) == 0 {
        return &object.Error{Message: "no allowed paths configured"}
    }
    
    // Validate arguments
    if len(args) < 1 {
        return &object.Error{Message: "Path() requires a path argument"}
    }
    
    // Validate path
    pathStr, err := args[0].AsString()
    if err != nil {
        return err
    }
    
    if !config.IsPathAllowed(pathStr) {
        return &object.Error{Message: "access denied"}
    }
    
    // Create instance...
}
```

### 4. Methods Creating New Instances

When a method needs to create a new instance of the same class, retrieve config from `this` and pass to constructor:

```go
func pathJoinpath(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    this := args[0].(*object.Instance)
    config := this.Fields["__config__"].(FSConfig)
    
    // ... compute newPath ...
    
    // Create new instance with same config
    return createPath(config, ctx, kwargs, []object.Object{&object.String{Value: newPath}})
}
```

## Complete Example

See `extlibs/pathlib.go` for a full implementation of a library with classes and instance data.

## Thread Safety

The implementation is thread-safe:

- Instance data is injected into context per-call
- No shared mutable state between instances
- Each interpreter can run in its own goroutine
- Functions can be called concurrently without data crossover

```go
// Safe to run concurrently
go func() {
    interpreter1.Eval("mylib.do_something()")
}()

go func() {
    interpreter2.Eval("mylib.do_something()")
}()
```

## Quick Reference

```go
// 1. Build template
builder := object.NewLibraryBuilder("mylib", "Description")
builder.Function("func", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    config := object.InstanceDataFromContext(ctx).(MyConfig)
    // Use config...
})
template := builder.Build()

// 2. Instantiate
lib := template.Instantiate(MyConfig{...})

// 3. Register
interpreter.RegisterLibrary(lib)

// 4. Use
interpreter.Import("mylib")
interpreter.Eval("mylib.func()")
```

For classes, add constructor that stores config in instance fields, and methods retrieve from `this.Fields["__config__"]`.
