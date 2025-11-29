package main

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// CreateExampleLibrary creates a demo library with math utilities and classes
func CreateExampleLibrary() *object.Library {
	// Define Person class
	personClass := &object.Class{
		Name: "Person",
		Methods: map[string]object.Object{
			"__init__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) != 3 {
						return &object.String{Value: "Error: __init__ requires 3 arguments (self, name, age)"}
					}
					person := args[0].(*object.Instance)
					nameObj, ok := args[1].(*object.String)
					if !ok {
						return &object.String{Value: "Error: name must be string"}
					}
					ageObj, ok := toInteger(args[2])
					if !ok {
						return &object.String{Value: "Error: age must be integer"}
					}

					// Set instance fields
					person.Fields["name"] = nameObj
					person.Fields["age"] = ageObj
					person.Fields["type"] = &object.String{Value: "person"}

					return &object.Null{}
				},
			},
			"__str__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) != 1 {
						return &object.String{Value: "Error: __str__ requires 1 argument (self)"}
					}
					person := args[0].(*object.Instance)
					name := person.Fields["name"].(*object.String).Value
					age := person.Fields["age"].(*object.Integer).Value
					return &object.String{Value: fmt.Sprintf("Person(name='%s', age=%d)", name, age)}
				},
			},
		},
	}

	return object.NewLibrary(map[string]*object.Builtin{
		"power": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.String{Value: "Error: power requires 2 arguments"}
				}

				// Type casting: convert to float64
				base, ok := toFloat64(args[0])
				if !ok {
					return &object.String{Value: "Error: base must be number"}
				}

				exp, ok := toFloat64(args[1])
				if !ok {
					return &object.String{Value: "Error: exponent must be number"}
				}

				result := math.Pow(base, exp)
				return &object.Float{Value: result}
			},
		},
		"sum_array": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.String{Value: "Error: sum_array requires 1 argument"}
				}

				// Handle arrays/lists
				listObj, ok := args[0].(*object.List)
				if !ok {
					return &object.String{Value: "Error: argument must be array"}
				}

				sum := 0.0
				for _, item := range listObj.Elements {
					val, ok := toFloat64(item)
					if !ok {
						return &object.String{Value: "Error: all array elements must be numbers"}
					}
					sum += val
				}

				return &object.Float{Value: sum}
			},
		},
		"get_map_value": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.String{Value: "Error: get_map_value requires 2 arguments"}
				}

				// Handle maps/dictionaries
				mapObj, ok := args[0].(*object.Dict)
				if !ok {
					return &object.String{Value: "Error: first argument must be map"}
				}

				keyObj, ok := args[1].(*object.String)
				if !ok {
					return &object.String{Value: "Error: second argument must be string key"}
				}

				// Look up value in map
				if pair, exists := mapObj.Pairs[keyObj.Value]; exists {
					return pair.Value
				}

				return &object.String{Value: "key not found"}
			},
		},
		"create_person": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.String{Value: "Error: create_person requires 2 arguments"}
				}

				nameObj, ok := args[0].(*object.String)
				if !ok {
					return &object.String{Value: "Error: name must be string"}
				}

				ageObj, ok := toInteger(args[1])
				if !ok {
					return &object.String{Value: "Error: age must be integer"}
				}

				// Create and return a Person instance
				personInstance := &object.Instance{
					Class:  personClass,
					Fields: make(map[string]object.Object),
				}

				// Initialize the person
				initMethod := personClass.Methods["__init__"]
				initMethod.(*object.Builtin).Fn(ctx, nil, personInstance, nameObj, ageObj)

				return personInstance
			},
		},
	}, map[string]object.Object{
		"Person": personClass,
	}, "")
}

// Helper function for type casting to float64
func toFloat64(obj object.Object) (float64, bool) {
	switch o := obj.(type) {
	case *object.Float:
		return o.Value, true
	case *object.Integer:
		return float64(o.Value), true
	default:
		return 0, false
	}
}

// Helper function for type casting to integer
func toInteger(obj object.Object) (*object.Integer, bool) {
	switch o := obj.(type) {
	case *object.Integer:
		return o, true
	case *object.Float:
		return &object.Integer{Value: int64(o.Value)}, true
	default:
		return nil, false
	}
}

func runGoExtensionExample() {
	fmt.Println("==========================================")
	fmt.Println("   Extending Scriptling with Go Code")
	fmt.Println("==========================================")

	p := scriptling.New()

	// Register a simple custom function
	p.RegisterFunc("greet", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) != 1 {
			return &object.String{Value: "Error: greet requires 1 argument"}
		}

		nameObj, ok := args[0].(*object.String)
		if !ok {
			return &object.String{Value: "Error: name must be string"}
		}

		return &object.String{Value: fmt.Sprintf("Hello, %s!", nameObj.Value)}
	})

	// Register a function that processes arrays
	p.RegisterFunc("process_numbers", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) != 1 {
			return &object.String{Value: "Error: process_numbers requires 1 argument"}
		}

		listObj, ok := args[0].(*object.List)
		if !ok {
			return &object.String{Value: "Error: argument must be array"}
		}

		// Process each number: multiply by 2 and convert to string
		results := make([]object.Object, len(listObj.Elements))
		for i, item := range listObj.Elements {
			if num, ok := toFloat64(item); ok {
				results[i] = &object.String{Value: strconv.FormatFloat(num*2, 'f', 2, 64)}
			} else {
				results[i] = &object.String{Value: "not a number"}
			}
		}

		return &object.List{Elements: results}
	})

	// Register the custom example library
	p.RegisterLibrary("mathutils", CreateExampleLibrary())

	// Run a comprehensive Scriptling script that demonstrates all features
	_, err := p.Eval(`
print("=== Custom Function Examples ===")

# Test simple function with argument
result = greet("World")
print("Greeting: " + result)

print("\n=== Array Processing ===")

# Test array processing function
numbers = [1, 2, 3.5, 4]
processed = process_numbers(numbers)
print("Original: " + str(numbers))
print("Processed: " + str(processed))

print("\n=== Library Functions ===")

# Import the custom library
import mathutils

# Test library functions with type casting
power_result = mathutils.power(2, 8)
print("2^8 = " + str(power_result))

# Test array summation
values = [1.5, 2.5, 3.0, 4.5]
total = mathutils.sum_array(values)
print("Sum of " + str(values) + " = " + str(total))

print("\n=== Class/Object Operations ===")

# Create a person using library function
person = mathutils.create_person("Alice", 30)
print("Person: " + str(person))  # Shows default object representation

# Access person attributes using dot notation
name = person.name
age = person.age
person_type = person.type
print("Name: " + str(name))
print("Age: " + str(age))
print("Type: " + str(person_type))

# Test accessing non-existent attribute (returns None)
try:
    missing = person.salary
    print("Salary: " + str(missing))
except:
    print("Salary not found (expected)")
`)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	fmt.Println()
}

func runScriptExtensionExample() {
	fmt.Println("==========================================")
	fmt.Println("   Extending Scriptling with Scripts")
	fmt.Println("==========================================")

	p := scriptling.New()

	fmt.Println("=== Registering Scriptling Functions ===")

	// Register a simple Scriptling function
	err := p.RegisterScriptFunc("calculate_area", `
def calculate_area(width, height):
    return width * height
calculate_area
`)
	if err != nil {
		fmt.Printf("Error registering function: %v\n", err)
		return
	}

	// Test the registered function
	_, err = p.Eval(`
area = calculate_area(10, 20)
print("Area of rectangle: " + str(area))
`)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Register a function with default parameters
	err = p.RegisterScriptFunc("format_name", `
def format_name(first, last, title="Mr."):
    return title + " " + first + " " + last
format_name
`)
	if err != nil {
		fmt.Printf("Error registering function: %v\n", err)
		return
	}

	_, err = p.Eval(`
name1 = format_name("John", "Doe")
name2 = format_name("Jane", "Smith", "Dr.")
print("Name 1: " + name1)
print("Name 2: " + name2)
`)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("\n=== Registering Scriptling Libraries ===")

	// Register a simple math utilities library
	err = p.RegisterScriptLibrary("mathutils", `
def square(x):
    return x * x

def cube(x):
    return x * x * x

def sum_of_squares(a, b):
    return square(a) + square(b)

PI = 3.14159
E = 2.71828
`)
	if err != nil {
		fmt.Printf("Error registering library: %v\n", err)
		return
	}

	_, err = p.Eval(`
import mathutils

print("Square of 5: " + str(mathutils.square(5)))
print("Cube of 3: " + str(mathutils.cube(3)))
print("Sum of squares of 3 and 4: " + str(mathutils.sum_of_squares(3, 4)))
print("PI constant: " + str(mathutils.PI))
`)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("\n=== Nested Library Imports ===")

	// Register a base library
	err = p.RegisterScriptLibrary("geometry_base", `
def distance(x1, y1, x2, y2):
    dx = x2 - x1
    dy = y2 - y1
    return (dx * dx + dy * dy) ** 0.5
`)
	if err != nil {
		fmt.Printf("Error registering base library: %v\n", err)
		return
	}

	// Register a library that uses the base library
	err = p.RegisterScriptLibrary("geometry_advanced", `
import geometry_base

def circle_circumference(radius):
    return 2 * 3.14159 * radius

def distance_from_origin(x, y):
    return geometry_base.distance(0, 0, x, y)
`)
	if err != nil {
		fmt.Printf("Error registering advanced library: %v\n", err)
		return
	}

	_, err = p.Eval(`
import geometry_advanced

print("Circumference of circle with radius 5: " + str(geometry_advanced.circle_circumference(5)))
print("Distance from origin to (3, 4): " + str(geometry_advanced.distance_from_origin(3, 4)))
`)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("\n=== Library Using Standard Library ===")

	// Register a library that uses a standard library
	err = p.RegisterScriptLibrary("data_processor", `
import json

def parse_user(json_str):
    user = json.loads(json_str)
    return user["name"] + " (" + str(user["age"]) + ")"

def create_user_json(name, age):
    data = {"name": name, "age": age}
    return json.dumps(data)
`)
	if err != nil {
		fmt.Printf("Error registering data processor library: %v\n", err)
		return
	}

	_, err = p.Eval(`
import data_processor

user_json = data_processor.create_user_json("Alice", 30)
print("Created JSON: " + user_json)

parsed = data_processor.parse_user(user_json)
print("Parsed user: " + parsed)
`)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println()
}

func main() {
	runGoExtensionExample()
	runScriptExtensionExample()
}
