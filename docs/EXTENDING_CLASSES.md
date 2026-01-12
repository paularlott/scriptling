# Extending Scriptling - Classes Guide

This guide covers how to define custom classes for Scriptling in Go, including both the native API and the Builder API.

## Overview

Scriptling supports object-oriented programming through custom classes. You can define classes using two approaches:

| Approach | When to Use |
|----------|-------------|
| **Native API** | Full control, complex inheritance, performance-critical |
| **Builder API** | Type-safe methods, cleaner syntax, rapid development |

See [EXTENDING_WITH_GO.md](EXTENDING_WITH_GO.md) for a detailed comparison.

## Native API

### Basic Class Creation

A class is an `*object.Class` structure containing methods:

```go
package main

import (
    "context"
    "fmt"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    p := scriptling.New()

    // Define a Person class
    personClass := &object.Class{
        Name: "Person",
        Methods: map[string]object.Object{
            "greet": &object.Builtin{
                Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                    // First argument is always 'self' (the instance)
                    if len(args) < 1 {
                        return &object.Error{Message: "greet requires instance"}
                    }
                    instance := args[0].(*object.Instance)
                    name, _ := instance.Fields["name"].AsString()
                    return &object.String{Value: "Hello, my name is " + name}
                },
                HelpText: "greet() - Return a greeting message",
            },
            "set_age": &object.Builtin{
                Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                    if len(args) < 2 {
                        return &object.Error{Message: "set_age requires instance and age"}
                    }
                    instance := args[0].(*object.Instance)
                    age := args[1]
                    instance.Fields["age"] = age
                    return object.NULL
                },
                HelpText: "set_age(age) - Set the person's age",
            },
        },
    }

    // Register the class as a library constant
    p.RegisterLibrary("person", object.NewLibrary(
        map[string]*object.Builtin{
            "create": {
                Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                    if len(args) < 1 {
                        return &object.Error{Message: "create requires name"}
                    }
                    name := args[0].(*object.String)

                    // Create instance with initial fields
                    instance := &object.Instance{
                        Class: personClass,
                        Fields: map[string]object.Object{
                            "name": name,
                            "age": &object.Integer{Value: 0},
                        },
                    }
                    return instance
                },
                HelpText: "create(name) - Create a new Person",
            },
            "Person": personClass, // Expose the class itself for help() and isinstance()
        },
        nil,
        "Person class library",
    ))

    // Use the class in Scriptling
    p.Eval(`
import person

john = person.create("John")
print(john.greet())  # Hello, my name is John

john.set_age(30)
print("Age:", john.age)  # Age: 30
`)
}
```

### Class with Constructor

The `__init__` method is called when creating instances:

```go
func createPersonClass() *object.Class {
    return &object.Class{
        Name: "Person",
        Methods: map[string]object.Object{
            "__init__": &object.Builtin{
                Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                    if len(args) < 3 {
                        return &object.Error{Message: "__init__ requires instance, name, and age"}
                    }
                    instance := args[0].(*object.Instance)
                    name, _ := args[1].AsString()
                    age, _ := args[2].AsInt()

                    instance.Fields["name"] = &object.String{Value: name}
                    instance.Fields["age"] = &object.Integer{Value: age}
                    return object.NULL
                },
                HelpText: "__init__(self, name, age) - Initialize the Person",
            },
            "introduce": &object.Builtin{
                Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                    instance := args[0].(*object.Instance)
                    name, _ := instance.Fields["name"].AsString()
                    age, _ := instance.Fields["age"].AsInt()
                    return &object.String{Value: fmt.Sprintf("Hi, I'm %s and I'm %d years old", name, age)}
                },
                HelpText: "introduce() - Return an introduction",
            },
        },
    }
}

// Usage with a factory function
p.RegisterLibrary("person", object.NewLibrary(
    map[string]*object.Builtin{
        "Person": personClass,
        "new": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                if len(args) < 2 {
                    return &object.Error{Message: "new requires name and age"}
                }

                // Create instance
                instance := &object.Instance{
                    Class:  personClass,
                    Fields: make(map[string]object.Object),
                }

                // Call constructor
                initMethod := personClass.Methods["__init__"].(*object.Builtin)
                initMethod.Fn(ctx, nil, instance, args[0], args[1])

                return instance
            },
            HelpText: "new(name, age) - Create a new Person",
        },
    },
    nil,
    "Person class",
))

// Use in Scriptling
p.Eval(`
import person

alice = person.new("Alice", 25)
print(alice.introduce())  # Hi, I'm Alice and I'm 25 years old
`)
```

### Class Inheritance (Composition)

Scriptling supports single inheritance through composition:

```go
func createEmployeeClass(personClass *object.Class) *object.Class {
    return &object.Class{
        Name: "Employee",
        Methods: map[string]object.Object{
            "__init__": &object.Builtin{
                Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                    if len(args) < 4 {
                        return &object.Error{Message: "__init__ requires instance, name, age, department"}
                    }
                    instance := args[0].(*object.Instance)
                    name := args[1]
                    age := args[2]
                    department := args[3]

                    // Call parent __init__
                    personInit := personClass.Methods["__init__"].(*object.Builtin)
                    personInit.Fn(ctx, nil, instance, name, age)

                    // Add employee-specific fields
                    instance.Fields["department"] = department
                    return object.NULL
                },
                HelpText: "__init__(self, name, age, department) - Initialize Employee",
            },
            "get_info": &object.Builtin{
                Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                    instance := args[0].(*object.Instance)
                    dept, _ := instance.Fields["department"].AsString()
                    age, _ := instance.Fields["age"].AsInt()
                    return &object.String{Value: fmt.Sprintf("Dept: %s, Age: %d", dept, age)}
                },
                HelpText: "get_info() - Get employee information",
            },
            // Inherit greet method from parent
            "greet": personClass.Methods["greet"],
        },
    }
}
```

### Special Methods

Scriptling supports special methods for custom behavior:

#### `__getitem__(key)` - Custom Indexing

```go
counterClass := &object.Class{
    Name: "Counter",
    Methods: map[string]object.Object{
        "__init__": &object.Builtin{
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                instance := args[0].(*object.Instance)
                instance.Fields["data"] = &object.Dict{Pairs: make(map[string]object.DictPair)}
                return object.NULL
            },
        },
        "__getitem__": &object.Builtin{
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                instance := args[0].(*object.Instance)
                key := args[1].Inspect()
                data := instance.Fields["data"].(*object.Dict)
                if val, ok := data.Pairs[key]; ok {
                    return val.Value
                }
                return &object.Integer{Value: 0}  // Default for missing keys
            },
            HelpText: "__getitem__(key) - Get count for key",
        },
        "set": &object.Builtin{
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                instance := args[0].(*object.Instance)
                key := args[1].Inspect()
                value := args[2]
                data := instance.Fields["data"].(*object.Dict)
                data.Pairs[key] = object.DictPair{Key: &object.String{Value: key}, Value: value}
                return object.NULL
            },
            HelpText: "set(key, value) - Set a count",
        },
    },
}

// Enables: c[key] syntax
p.Eval(`
c = Counter()
c.set("apples", 5)
print(c["apples"])  # 5
print(c["oranges"])  # 0 (default)
`)
```

#### Other Special Methods

| Method | Purpose |
|--------|---------|
| `__init__` | Constructor called when creating instances |
| `__str__` | Custom string representation (for `str()` function) |
| `__len__` | Custom length (for `len()` function) |
| `__getitem__` | Custom indexing (for `obj[key]` syntax) |

### Classes in Libraries

Add classes to libraries via the constants map:

```go
myLib := object.NewLibrary(
    map[string]*object.Builtin{
        "create_counter": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                // Factory function
                return &object.Instance{
                    Class: counterClass,
                    Fields: map[string]object.Object{
                        "count": &object.Integer{Value: 0},
                    },
                }
            },
            HelpText: "create_counter() - Create a new Counter",
        },
    },
    map[string]object.Object{
        // Classes and constants go here
        "Counter": counterClass,
        "VERSION": &object.String{Value: "1.0.0"},
    },
    "Counter utilities library",
)

p.RegisterLibrary("counters", myLib)

// Use in Scriptling
p.Eval(`
import counters
c = counters.Counter()
# or use factory
c = counters.create_counter()
`)
```

## Builder API (Fluent Class)

The Builder API provides a type-safe way to create classes with automatic parameter conversion.

### Creating a Class

```go
import "github.com/paularlott/scriptling/object"

// Create class builder
cb := object.NewClassBuilder("Person")

// Add methods
cb.Method("greet", func(self *object.Instance) string {
    name, _ := self.Fields["name"].AsString()
    return "Hello, my name is " + name
})

cb.Method("set_age", func(self *object.Instance, age int) {
    self.Fields["age"] = &object.Integer{Value: int64(age)}
})

// Build the class
personClass := cb.Build()
```

### Method Signatures

Class methods support flexible signatures. The first parameter is ALWAYS the instance (`self`):

- `func(self *Instance, args...) result` - Instance + positional arguments
- `func(self *Instance, ctx context.Context, args...) result` - Instance + context + positional
- `func(self *Instance, kwargs object.Kwargs, args...) result` - Instance + kwargs + positional
- `func(self *Instance, ctx context.Context, kwargs object.Kwargs, args...) result` - All parameters

**Parameter Order Rules (ALWAYS in this order):**
1. Instance (`self`) - ALWAYS FIRST
2. Context (optional) - comes second if present
3. Kwargs (optional) - comes after context (or second if no context)
4. Positional arguments - ALWAYS LAST

### Examples

**Simple instance method:**

```go
cb.Method("get_name", func(self *object.Instance) string {
    name, _ := self.Fields["name"].AsString()
    return name
})
```

**Method with parameters:**

```go
cb.Method("add_friend", func(self *object.Instance, friendName string) {
    friends, _ := self.Fields["friends"].(*object.List)
    friends.Elements = append(friends.Elements, &object.String{Value: friendName})
})
```

**Method with context and error handling:**

```go
cb.Method("save", func(self *object.Instance, ctx context.Context) error {
    // Simulate async save operation
    select {
    case <-time.After(100 * time.Millisecond):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
})
```

**Method with kwargs:**

```go
cb.Method("configure", func(self *object.Instance, kwargs object.Kwargs) error {
    timeout, _ := kwargs.GetInt("timeout", 30)
    debug, _ := kwargs.GetBool("debug", false)

    self.Fields["timeout"] = &object.Integer{Value: int64(timeout)}
    self.Fields["debug"] = &object.Boolean{Value: debug}
    return nil
})
```

**Method with context and kwargs:**

```go
cb.Method("fetch", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) (string, error) {
    url, _ := kwargs.GetString("url", "")
    timeout, _ := kwargs.GetInt("timeout", 30)
    
    // Use context for timeout
    ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
    defer cancel()
    
    // Fetch data...
    return "data", nil
})
```

### Adding Help Text

```go
cb.MethodWithHelp("calculate", func(self *object.Instance, a, b int) int {
    return a + b
}, "calculate(a, b) - Add two numbers")

cb.MethodWithHelp("process", func(self *object.Instance, data string) error {
    // Implementation
    return nil
}, "process(data) - Process the data")
```

### Inheritance

Set a base class for inheritance:

```go
baseClass := &object.Class{Name: "Base", Methods: map[string]object.Object{}}
cb := object.NewClassBuilder("Derived")
cb.BaseClass(baseClass)
cb.Method("special", func(self *object.Instance) string {
    return "special method"
})
```

### Cross-Approach Inheritance

The Builder API and Native API work seamlessly together for inheritance.

#### Builder Class Inheriting from Native Base

When you have a native base class and want to use the Builder API for the derived class:

```go
// Native base class (Person)
personClass := &object.Class{
    Name: "Person",
    Methods: map[string]object.Object{
        "__init__": &object.Builtin{
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                instance := args[0].(*object.Instance)
                name, _ := args[1].AsString()
                age, _ := args[2].AsInt()
                instance.Fields["name"] = &object.String{Value: name}
                instance.Fields["age"] = &object.Integer{Value: age}
                return object.NULL
            },
            HelpText: "__init__(self, name, age) - Initialize Person",
        },
        "greet": &object.Builtin{
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                instance := args[0].(*object.Instance)
                name, _ := instance.Fields["name"].AsString()
                return &object.String{Value: "Hello, I'm " + name}
            },
            HelpText: "greet() - Return a greeting",
        },
    },
}

// Builder API derived class (Employee)
cb := object.NewClassBuilder("Employee")
cb.BaseClass(personClass)  // Inherit from native class
cb.Method("__init__", func(self *object.Instance, name string, age int, department string) {
    // Call parent __init__ using native API
    parentInit := personClass.Methods["__init__"].(*object.Builtin)
    parentInit.Fn(nil, nil, self, &object.String{Value: name}, &object.Integer{Value: int64(age)})

    // Add employee-specific field
    self.Fields["department"] = &object.String{Value: department}
})

cb.Method("get_info", func(self *object.Instance) string {
    dept, _ := self.Fields["department"].AsString()
    age, _ := self.Fields["age"].AsInt()
    return fmt.Sprintf("Dept: %s, Age: %d", dept, age)
})

// The employee class inherits greet() from Person
employeeClass := cb.Build()
```

#### Native Class Inheriting from Builder Base

When you have a builder base class and want to create a native derived class:

```go
// Builder base class (Animal)
cb := object.NewClassBuilder("Animal")
cb.Method("__init__", func(self *object.Instance, name string) {
    self.Fields["name"] = &object.String{Value: name}
})
cb.Method("speak", func(self *object.Instance) string {
    name, _ := self.Fields["name"].AsString()
    return name + " makes a sound"
})
animalClass := cb.Build()

// Native derived class (Dog) inheriting from builder class
dogClass := &object.Class{
    Name: "Dog",
    Methods: map[string]object.Object{
        "__init__": &object.Builtin{
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                if len(args) < 2 {
                    return &object.Error{Message: "__init__ requires instance and name"}
                }
                instance := args[0].(*object.Instance)
                name, _ := args[1].AsString()

                // Call parent __init__ (from builder class)
                parentInit := animalClass.Methods["__init__"].(*object.Builtin)
                parentInit.Fn(ctx, nil, instance, &object.String{Value: name})

                // Add dog-specific field
                instance.Fields["breed"] = &object.String{Value: "Unknown"}
                return object.NULL
            },
            HelpText: "__init__(self, name) - Initialize Dog",
        },
        "bark": &object.Builtin{
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                instance := args[0].(*object.Instance)
                name, _ := instance.Fields["name"].AsString()
                return &object.String{Value: name + " says: Woof!"}
            },
            HelpText: "bark() - Make the dog bark",
        },
        // Inherit speak() method from Animal (builder class)
        "speak": animalClass.Methods["speak"],
    },
}

// Register both classes
p.SetVar("Animal", animalClass)
p.SetVar("Dog", dogClass)
```

#### Builder Class Inheriting from Builder Base

When both parent and child use the Builder API:

```go
// Builder base class (Vehicle)
vehicleBuilder := object.NewClassBuilder("Vehicle")
vehicleBuilder.Method("__init__", func(self *object.Instance, make string, model string) {
    self.Fields["make"] = &object.String{Value: make}
    self.Fields["model"] = &object.String{Value: model}
})
vehicleBuilder.Method("get_info", func(self *object.Instance) string {
    make, _ := self.Fields["make"].AsString()
    model, _ := self.Fields["model"].AsString()
    return make + " " + model
})
vehicleClass := vehicleBuilder.Build()

// Builder derived class (Car)
carBuilder := object.NewClassBuilder("Car")
carBuilder.BaseClass(vehicleClass)  // Inherit from builder class
carBuilder.Method("__init__", func(self *object.Instance, make string, model string, doors int) {
    // Call parent __init__ using parent's built method
    parentInit := vehicleClass.Methods["__init__"].(*object.Builtin)
    parentInit.Fn(nil, nil, self, &object.String{Value: make}, &object.String{Value: model})

    // Add car-specific field
    self.Fields["doors"] = &object.Integer{Value: int64(doors})
})

carBuilder.Method("honk", func(self *object.Instance) string {
    make, _ := self.Fields["make"].AsString()
    return make + " goes beep beep!"
})

// Car inherits get_info() from Vehicle
carClass := carBuilder.Build()
```

### Complete Example

```go
package main

import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    p := scriptling.New()

    // Create Person class using ClassBuilder
    cb := object.NewClassBuilder("Person")

    cb.Method("__init__", func(self *object.Instance, name string, age int) {
        self.Fields["name"] = &object.String{Value: name}
        self.Fields["age"] = &object.Integer{Value: int64(age)}
    })

    cb.Method("greet", func(self *object.Instance) string {
        name, _ := self.Fields["name"].AsString()
        return "Hello, I'm " + name
    })

    cb.Method("have_birthday", func(self *object.Instance) {
        age, _ := self.Fields["age"].AsInt()
        self.Fields["age"] = &object.Integer{Value: age + 1}
    })

    personClass := cb.Build()

    // Create library with factory function
    lib := object.NewLibraryBuilder("person", "Person class library")
    lib.Constant("Person", personClass)
    lib.Function("create", func(name string, age int) *object.Instance {
        instance := &object.Instance{
            Class:  personClass,
            Fields: make(map[string]object.Object),
        }
        // Call constructor
        initMethod := personClass.Methods["__init__"].(*object.Builtin)
        initMethod.Fn(nil, nil, instance, &object.String{Value: name}, &object.Integer{Value: int64(age)})
        return instance
    })

    p.RegisterLibrary("person", lib.Build())

    // Use in Scriptling
    p.Eval(`
import person

john = person.create("John", 25)
print(john.greet())  # Hello, I'm John

john.have_birthday()
print("Age:", john.age)  # Age: 26
`)
}
```

### Builder Methods Reference

| Method | Description |
|--------|-------------|
| `Method(name, fn)` | Register a typed Go method |
| `MethodWithHelp(name, fn, help)` | Register method with help text |
| `BaseClass(base)` | Set base class for inheritance |
| `Environment(env)` | Set environment (usually not needed) |
| `Build()` | Create and return the Class |

## Choosing Between Native and Builder API

| Factor | Native API | Builder API |
|--------|------------|-------------|
| **Performance** | Faster (no reflection overhead) | Slight overhead |
| **Type Safety** | Manual checking | Automatic conversion |
| **Control** | Full control over method logic | Convention-based |
| **Help Text** | Manual `HelpText` field | Chainable `MethodWithHelp()` |
| **Best For** | Complex inheritance, performance | Type-safe methods, rapid development |

## Best Practices

### 1. Always Provide `__init__`

```go
cb.Method("__init__", func(self *object.Instance, name string, age int) {
    self.Fields["name"] = &object.String{Value: name}
    self.Fields["age"] = &object.Integer{Value: int64(age)}
})
```

### 2. Document Methods with Help Text

```go
cb.MethodWithHelp("calculate", func(self *object.Instance, a, b int) int {
    return a + b
}, `calculate(a, b) - Add two numbers

  Parameters:
    a - First number
    b - Second number

  Returns:
    The sum of a and b`)
```

### 3. Expose Classes in Libraries

```go
lib := object.NewLibrary(
    functions,
    map[string]object.Object{
        "MyClass": myClass,  // Expose for help() and isinstance()
    },
    description,
)
```

### 4. Use Type-Safe Field Access

```go
// Good: Use type-safe accessors
cb.Method("get_age", func(self *object.Instance) int {
    age, err := self.Fields["age"].AsInt()
    if err != nil {
        return 0
    }
    return age
})

// Manual type assertion (avoid if possible)
cb.Method("get_age", func(self *object.Instance) int {
    if ageObj, ok := self.Fields["age"].(*object.Integer); ok {
        return int(ageObj.Value)
    }
    return 0
})
```

## Testing Classes

```go
func TestClass(t *testing.T) {
    p := scriptling.New()

    // Create class
    cb := object.NewClassBuilder("Counter")
    cb.Method("__init__", func(self *object.Instance) {
        self.Fields["count"] = &object.Integer{Value: 0}
    })
    cb.Method("increment", func(self *object.Instance) int {
        count, _ := self.Fields["count"].AsInt()
        self.Fields["count"] = &object.Integer{Value: count + 1}
        return count + 1
    })
    counterClass := cb.Build()

    // Register class
    p.SetVar("Counter", counterClass)

    // Test the class
    result, err := p.Eval(`
c = Counter()
c.increment()
c.increment()
result = c.increment()
`)
    if err != nil {
        t.Fatalf("Eval error: %v", err)
    }

    if value, objErr := result.AsInt(); objErr == nil {
        if value != 3 {
            t.Errorf("Expected 3, got %d", value)
        }
    }
}
```

## See Also

- **[EXTENDING_WITH_GO.md](EXTENDING_WITH_GO.md)** - Overview and common concepts
- **[EXTENDING_FUNCTIONS.md](EXTENDING_FUNCTIONS.md)** - Creating individual functions
- **[EXTENDING_LIBRARIES.md](EXTENDING_LIBRARIES.md)** - Creating libraries
- **[EXTENDING_WITH_SCRIPTS.md](EXTENDING_WITH_SCRIPTS.md)** - Creating extensions in Scriptling
- **[HELP_SYSTEM.md](HELP_SYSTEM.md)** - Adding documentation
