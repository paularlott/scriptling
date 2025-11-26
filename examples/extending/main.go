package main

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// CreateMathUtilsLibrary creates a custom math utilities library
func CreateMathUtilsLibrary() *object.Library {
	return object.NewLibrary(map[string]*object.Builtin{
		"power": {
			Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
			Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
			Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
			Fn: func(ctx context.Context, args ...object.Object) object.Object {
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

				// Create and return a map/dictionary
				pairs := make(map[string]object.DictPair)
				pairs["name"] = object.DictPair{
					Key:   &object.String{Value: "name"},
					Value: nameObj,
				}
				pairs["age"] = object.DictPair{
					Key:   &object.String{Value: "age"},
					Value: ageObj,
				}
				pairs["type"] = object.DictPair{
					Key:   &object.String{Value: "type"},
					Value: &object.String{Value: "person"},
				}

				return &object.Dict{Pairs: pairs}
			},
		},
	})
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

func main() {
	p := scriptling.New()

	// Register a simple custom function
	p.RegisterFunc("greet", func(ctx context.Context, args ...object.Object) object.Object {
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
	p.RegisterFunc("process_numbers", func(ctx context.Context, args ...object.Object) object.Object {
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

	// Register the custom math utilities library
	p.RegisterLibrary("mathutils", CreateMathUtilsLibrary())

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

print("\n=== Map/Dictionary Operations ===")

# Create a person using library function
person = mathutils.create_person("Alice", 30)
print("Person: " + str(person))

# Get values from the person map
name = mathutils.get_map_value(person, "name")
age = mathutils.get_map_value(person, "age")
person_type = mathutils.get_map_value(person, "type")
print("Name: " + str(name))
print("Age: " + str(age))
print("Type: " + str(person_type))

# Test missing key
missing = mathutils.get_map_value(person, "salary")
print("Missing key result: " + str(missing))

print("\n=== All Examples Completed ===")
`)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
