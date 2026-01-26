package scriptling

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestInheritance_ChainLookup validates that Go-defined classes automatically resolve
// parent methods through the inheritance chain without manual method copying.
// This is a regression test for the fix that made callInstanceMethod traverse BaseClass.
func TestInheritance_ChainLookup(t *testing.T) {
	t.Run("GoDefinedClassesWithInheritance", func(t *testing.T) {
		p := New()

		// Create parent class with methods
		parentClass := &object.Class{
			Name: "Animal",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						if len(args) > 1 {
							name, _ := args[1].AsString()
							self.Fields["name"] = &object.String{Value: name}
						}
						return &object.Null{}
					},
					HelpText: "__init__(self, name) - Initialize the animal",
				},
				"speak": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						name, _ := self.Fields["name"].AsString()
						return &object.String{Value: name + " makes a sound"}
					},
					HelpText: "speak() - Make the animal speak",
				},
				"sleep": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						name, _ := self.Fields["name"].AsString()
						return &object.String{Value: name + " is sleeping"}
					},
					HelpText: "sleep() - Make the animal sleep",
				},
			},
		}

		// Create child class that inherits from parent
		// NOTE: We do NOT copy parent methods to child - the evaluator should handle this
		childClass := &object.Class{
			Name:      "Dog",
			BaseClass: parentClass,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						if len(args) > 1 {
							name, _ := args[1].AsString()
							self.Fields["name"] = &object.String{Value: name}
						}
						return &object.Null{}
					},
					HelpText: "__init__(self, name) - Initialize the dog",
				},
				"bark": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						name, _ := self.Fields["name"].AsString()
						return &object.String{Value: name + " barks loudly"}
					},
					HelpText: "bark() - Make the dog bark",
				},
			},
		}

		// Create grandchild class with even more specific methods
		grandchildClass := &object.Class{
			Name:      "Puppy",
			BaseClass: childClass,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						if len(args) > 1 {
							name, _ := args[1].AsString()
							self.Fields["name"] = &object.String{Value: name}
						}
						return &object.Null{}
					},
					HelpText: "__init__(self, name) - Initialize the puppy",
				},
				"whimper": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						name, _ := self.Fields["name"].AsString()
						return &object.String{Value: name + " whines softly"}
					},
					HelpText: "whimper() - Make the puppy whine",
				},
			},
		}

		// Register classes through a library to make them callable
		p.RegisterLibrary(object.NewLibrary("animals", nil, map[string]object.Object{
			"Animal": parentClass,
			"Dog":    childClass,
			"Puppy":  grandchildClass,
		}, "Animal library"))

		// Test Dog can call parent methods (speak, sleep) and own method (bark)
		_, err := p.Eval(`
import animals
dog = animals.Dog("Buddy")
speak_result = dog.speak()
sleep_result = dog.sleep()
bark_result = dog.bark()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		speakResult, _ := p.GetVarAsString("speak_result")
		if speakResult != "Buddy makes a sound" {
			t.Errorf("expected 'Buddy makes a sound', got %s", speakResult)
		}

		sleepResult, _ := p.GetVarAsString("sleep_result")
		if sleepResult != "Buddy is sleeping" {
			t.Errorf("expected 'Buddy is sleeping', got %s", sleepResult)
		}

		barkResult, _ := p.GetVarAsString("bark_result")
		if barkResult != "Buddy barks loudly" {
			t.Errorf("expected 'Buddy barks loudly', got %s", barkResult)
		}
	})

	t.Run("DeepInheritanceChain", func(t *testing.T) {
		p := New()

		// Create a chain of 4 classes: A -> B -> C -> D
		classA := &object.Class{
			Name: "A",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"method_a": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "A"}
					},
				},
			},
		}

		classB := &object.Class{
			Name:      "B",
			BaseClass: classA,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"method_b": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "B"}
					},
				},
			},
		}

		classC := &object.Class{
			Name:      "C",
			BaseClass: classB,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"method_c": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "C"}
					},
				},
			},
		}

		classD := &object.Class{
			Name:      "D",
			BaseClass: classC,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"method_d": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "D"}
					},
				},
			},
		}

		p.RegisterLibrary(object.NewLibrary("test", nil, map[string]object.Object{
			"D": classD,
		}, "Test library"))

		// Test that class D instance can call methods from all ancestors
		_, err := p.Eval(`
import test
obj = test.D()
result_a = obj.method_a()
result_b = obj.method_b()
result_c = obj.method_c()
result_d = obj.method_d()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		resultA, _ := p.GetVarAsString("result_a")
		if resultA != "A" {
			t.Errorf("expected 'A', got %s", resultA)
		}

		resultB, _ := p.GetVarAsString("result_b")
		if resultB != "B" {
			t.Errorf("expected 'B', got %s", resultB)
		}

		resultC, _ := p.GetVarAsString("result_c")
		if resultC != "C" {
			t.Errorf("expected 'C', got %s", resultC)
		}

		resultD, _ := p.GetVarAsString("result_d")
		if resultD != "D" {
			t.Errorf("expected 'D', got %s", resultD)
		}
	})

	t.Run("MethodShadowing", func(t *testing.T) {
		p := New()

		// Parent has a method
		parentClass := &object.Class{
			Name: "Parent",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"process": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "parent process"}
					},
				},
			},
		}

		// Child overrides the method
		childClass := &object.Class{
			Name:      "Child",
			BaseClass: parentClass,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"process": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "child process"}
					},
				},
			},
		}

		p.RegisterLibrary(object.NewLibrary("test", nil, map[string]object.Object{
			"Parent": parentClass,
			"Child":  childClass,
		}, "Test library"))

		// Test that child's method shadows parent's
		_, err := p.Eval(`
import test
parent = test.Parent()
child = test.Child()
parent_result = parent.process()
child_result = child.process()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		parentResult, _ := p.GetVarAsString("parent_result")
		if parentResult != "parent process" {
			t.Errorf("expected 'parent process', got %s", parentResult)
		}

		childResult, _ := p.GetVarAsString("child_result")
		if childResult != "child process" {
			t.Errorf("expected 'child process', got %s", childResult)
		}
	})

	t.Run("MixedInheritanceScriptAndGo", func(t *testing.T) {
		p := New()

		// Create a Go class
		goClass := &object.Class{
			Name: "GoClass",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"go_method": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "go method"}
					},
				},
			},
		}

		p.RegisterLibrary(object.NewLibrary("golib", nil, map[string]object.Object{
			"GoClass": goClass,
		}, "Go library"))

		// Create a Scriptling class that inherits from Go class
		_, err := p.Eval(`
import golib

class ScriptClass(golib.GoClass):
    def script_method(self):
        return "script method"

# Script class should have both methods
obj = ScriptClass()
go_result = obj.go_method()
script_result = obj.script_method()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		goResult, _ := p.GetVarAsString("go_result")
		if goResult != "go method" {
			t.Errorf("expected 'go method', got %s", goResult)
		}

		scriptResult, _ := p.GetVarAsString("script_result")
		if scriptResult != "script method" {
			t.Errorf("expected 'script method', got %s", scriptResult)
		}
	})
}

// TestInheritance_BuilderAPI validates that Builder API classes also support inheritance
func TestInheritance_BuilderAPI(t *testing.T) {
	t.Run("BuilderAPI_Inheritance", func(t *testing.T) {
		p := New()

		// Create parent class using Builder API
		parentBuilder := object.NewClassBuilder("Animal")
		parentBuilder.Method("__init__", func(self *object.Instance, name string) {
			self.Fields["name"] = &object.String{Value: name}
		})
		parentBuilder.Method("speak", func(self *object.Instance) string {
			name, _ := self.Fields["name"].AsString()
			return name + " makes a sound"
		})
		parentClass := parentBuilder.Build()

		// Create child class using Builder API with BaseClass
		childBuilder := object.NewClassBuilder("Dog")
		childBuilder.BaseClass(parentClass)
		childBuilder.Method("__init__", func(self *object.Instance, name string) {
			self.Fields["name"] = &object.String{Value: name}
		})
		childBuilder.Method("bark", func(self *object.Instance) string {
			name, _ := self.Fields["name"].AsString()
			return name + " barks loudly"
		})
		childClass := childBuilder.Build()

		// Register classes through a library
		p.RegisterLibrary(object.NewLibrary("animals", nil, map[string]object.Object{
			"Animal": parentClass,
			"Dog":    childClass,
		}, "Animal library"))

		// Test that Dog can call parent method (speak) and own method (bark)
		_, err := p.Eval(`
import animals
dog = animals.Dog("Buddy")
speak_result = dog.speak()
bark_result = dog.bark()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		speakResult, _ := p.GetVarAsString("speak_result")
		if speakResult != "Buddy makes a sound" {
			t.Errorf("expected 'Buddy makes a sound', got %s", speakResult)
		}

		barkResult, _ := p.GetVarAsString("bark_result")
		if barkResult != "Buddy barks loudly" {
			t.Errorf("expected 'Buddy barks loudly', got %s", barkResult)
		}
	})

	t.Run("BuilderAPI_DeepInheritance", func(t *testing.T) {
		p := New()

		// Create A -> B -> C chain using Builder API
		classA := object.NewClassBuilder("A")
		classA.Method("__init__", func(self *object.Instance) {})
		classA.Method("method_a", func(self *object.Instance) string {
			return "A"
		})
		aClass := classA.Build()

		classB := object.NewClassBuilder("B")
		classB.BaseClass(aClass)
		classB.Method("__init__", func(self *object.Instance) {})
		classB.Method("method_b", func(self *object.Instance) string {
			return "B"
		})
		bClass := classB.Build()

		classC := object.NewClassBuilder("C")
		classC.BaseClass(bClass)
		classC.Method("__init__", func(self *object.Instance) {})
		classC.Method("method_c", func(self *object.Instance) string {
			return "C"
		})
		cClass := classC.Build()

		p.RegisterLibrary(object.NewLibrary("test", nil, map[string]object.Object{
			"C": cClass,
		}, "Test library"))

		// Test that class C instance can call methods from all ancestors
		_, err := p.Eval(`
import test
obj = test.C()
result_a = obj.method_a()
result_b = obj.method_b()
result_c = obj.method_c()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		resultA, _ := p.GetVarAsString("result_a")
		if resultA != "A" {
			t.Errorf("expected 'A', got %s", resultA)
		}

		resultB, _ := p.GetVarAsString("result_b")
		if resultB != "B" {
			t.Errorf("expected 'B', got %s", resultB)
		}

		resultC, _ := p.GetVarAsString("result_c")
		if resultC != "C" {
			t.Errorf("expected 'C', got %s", resultC)
		}
	})
}

// TestInheritance_Super validates that super() works correctly with Go-defined classes
func TestInheritance_Super(t *testing.T) {
	t.Run("SuperCallsParentGoMethod", func(t *testing.T) {
		p := New()

		// Create parent class with a method
		parentClass := &object.Class{
			Name: "Parent",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						self.Fields["value"] = &object.Integer{Value: 10}
						return &object.Null{}
					},
				},
				"get_value": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						return self.Fields["value"]
					},
				},
			},
		}

		p.RegisterLibrary(object.NewLibrary("test", nil, map[string]object.Object{
			"Parent": parentClass,
		}, "Test library"))

		// Test super() can call parent's Go-defined method
		_, err := p.Eval(`
import test

class Child(test.Parent):
    def get_value(self):
        parent_val = super().get_value()
        return parent_val + 25

obj = Child()
result = obj.get_value()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, _ := p.GetVarAsInt("result")
		if result != 35 {
			t.Errorf("expected 35, got %d", result)
		}
	})

	t.Run("SuperCallsGrandparentMethod", func(t *testing.T) {
		p := New()

		// Create grandparent class with a method
		grandparentClass := &object.Class{
			Name: "Grandparent",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"greet": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "Hello from Grandparent"}
					},
				},
			},
		}

		// Create middle class
		middleClass := &object.Class{
			Name:      "Middle",
			BaseClass: grandparentClass,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
			},
		}

		p.RegisterLibrary(object.NewLibrary("test", nil, map[string]object.Object{
			"Grandparent": grandparentClass,
			"Middle":      middleClass,
		}, "Test library"))

		// Test super() can skip middle and call grandparent's method
		_, err := p.Eval(`
import test

class Child(test.Middle):
    def greet(self):
        return super().greet()

obj = Child()
result = obj.greet()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, _ := p.GetVarAsString("result")
		if result != "Hello from Grandparent" {
			t.Errorf("expected 'Hello from Grandparent', got %s", result)
		}
	})

	t.Run("SuperWithBuilderAPI", func(t *testing.T) {
		p := New()

		// Create parent class using Builder API
		parentBuilder := object.NewClassBuilder("Parent")
		parentBuilder.Method("__init__", func(self *object.Instance) {
			self.Fields["base"] = &object.Integer{Value: 100}
		})
		parentBuilder.Method("get_base", func(self *object.Instance) int64 {
			val, _ := self.Fields["base"].AsInt()
			return val
		})
		parentClass := parentBuilder.Build()

		p.RegisterLibrary(object.NewLibrary("test", nil, map[string]object.Object{
			"Parent": parentClass,
		}, "Test library"))

		// Test super() with Builder API classes
		_, err := p.Eval(`
import test

class Child(test.Parent):
    def get_base(self):
        return super().get_base() * 2

obj = Child()
result = obj.get_base()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, _ := p.GetVarAsInt("result")
		if result != 200 {
			t.Errorf("expected 200, got %d", result)
		}
	})
}

// TestInheritance_ScriptInheritsFromGo validates that Scriptling classes can inherit from Go-defined classes
func TestInheritance_ScriptInheritsFromGo(t *testing.T) {
	t.Run("ScriptClassInheritsNativeGoClass", func(t *testing.T) {
		p := New()

		// Create a Go-defined class using Native API
		goClass := &object.Class{
			Name: "GoAnimal",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						if len(args) > 1 {
							name, _ := args[1].AsString()
							self.Fields["name"] = &object.String{Value: name}
						}
						return &object.Null{}
					},
				},
				"speak": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						self := args[0].(*object.Instance)
						name, _ := self.Fields["name"].AsString()
						return &object.String{Value: name + " makes a sound"}
					},
				},
				"go_method": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "called from Go"}
					},
				},
			},
		}

		p.RegisterLibrary(object.NewLibrary("animals", nil, map[string]object.Object{
			"GoAnimal": goClass,
		}, "Animal library"))

		// Scriptling class inherits from Go class
		_, err := p.Eval(`
import animals

class Dog(animals.GoAnimal):
    def __init__(self, name, breed):
        # Call parent's __init__
        super().__init__(name)
        self.breed = breed

    def bark(self):
        return self.name + " barks!"

# Create instance and test inheritance
dog = Dog("Buddy", "Golden Retriever")

# Can access fields set in Go __init__
name = dog.name
breed = dog.breed

# Can call methods from Go parent
speak_result = dog.speak()
go_result = dog.go_method()

# Can call methods defined in Scriptling child
bark_result = dog.bark()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Check field access
		name, _ := p.GetVarAsString("name")
		if name != "Buddy" {
			t.Errorf("expected 'Buddy', got %s", name)
		}

		breed, _ := p.GetVarAsString("breed")
		if breed != "Golden Retriever" {
			t.Errorf("expected 'Golden Retriever', got %s", breed)
		}

		// Check Go method calls
		speakResult, _ := p.GetVarAsString("speak_result")
		if speakResult != "Buddy makes a sound" {
			t.Errorf("expected 'Buddy makes a sound', got %s", speakResult)
		}

		goResult, _ := p.GetVarAsString("go_result")
		if goResult != "called from Go" {
			t.Errorf("expected 'called from Go', got %s", goResult)
		}

		// Check Scriptling method
		barkResult, _ := p.GetVarAsString("bark_result")
		if barkResult != "Buddy barks!" {
			t.Errorf("expected 'Buddy barks!', got %s", barkResult)
		}
	})

	t.Run("ScriptClassInheritsBuilderGoClass", func(t *testing.T) {
		p := New()

		// Create a Go-defined class using Builder API
		builder := object.NewClassBuilder("GoVehicle")
		builder.Method("__init__", func(self *object.Instance, make string, model string) {
			self.Fields["make"] = &object.String{Value: make}
			self.Fields["model"] = &object.String{Value: model}
			self.Fields["speed"] = &object.Integer{Value: 0}
		})
		builder.Method("accelerate", func(self *object.Instance, amount int64) string {
			speed, _ := self.Fields["speed"].AsInt()
			self.Fields["speed"] = &object.Integer{Value: speed + amount}
			make, _ := self.Fields["make"].AsString()
			model, _ := self.Fields["model"].AsString()
			return make + " " + model + " accelerated to " + string(rune(speed+amount)) + " mph"
		})
		builder.Method("get_speed", func(self *object.Instance) int64 {
			speed, _ := self.Fields["speed"].AsInt()
			return speed
		})
		goVehicle := builder.Build()

		p.RegisterLibrary(object.NewLibrary("vehicles", nil, map[string]object.Object{
			"GoVehicle": goVehicle,
		}, "Vehicle library"))

		// Scriptling class inherits from Go Builder class
		_, err := p.Eval(`
import vehicles

class Car(vehicles.GoVehicle):
    def __init__(self, make, model, color):
        # Call parent's __init__
        super().__init__(make, model)
        self.color = color

    def honk(self):
        return self.make + " " + self.model + " honks!"

    def accelerate_with_honk(self, amount):
        sound = self.honk()
        speed_info = super().accelerate(amount)
        return sound + " and " + speed_info

# Create instance and test
car = Car("Toyota", "Camry", "Blue")

# Access fields set in Go __init__
make = car.make
model = car.model
color = car.color
speed = car.get_speed()

# Call Go method
new_speed = car.get_speed()
car.accelerate(50)
after_accel = car.get_speed()

# Call Scriptling method
honk_result = car.honk()

# Call method that uses both
combo = car.accelerate_with_honk(20)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Check fields
		make, _ := p.GetVarAsString("make")
		if make != "Toyota" {
			t.Errorf("expected 'Toyota', got %s", make)
		}

		model, _ := p.GetVarAsString("model")
		if model != "Camry" {
			t.Errorf("expected 'Camry', got %s", model)
		}

		color, _ := p.GetVarAsString("color")
		if color != "Blue" {
			t.Errorf("expected 'Blue', got %s", color)
		}

		// Check speed field (should be 0 initially)
		speed, _ := p.GetVarAsInt("speed")
		if speed != 0 {
			t.Errorf("expected 0, got %d", speed)
		}

		// Check accelerate worked
		afterAccel, _ := p.GetVarAsInt("after_accel")
		if afterAccel != 50 {
			t.Errorf("expected 50, got %d", afterAccel)
		}

		// Check Scriptling method
		honkResult, _ := p.GetVarAsString("honk_result")
		if honkResult != "Toyota Camry honks!" {
			t.Errorf("expected 'Toyota Camry honks!', got %s", honkResult)
		}

		// Check combo method
		combo, _ := p.GetVarAsString("combo")
		// After accelerate(20), speed becomes 50 + 20 = 70
		if combo != "Toyota Camry honks! and Toyota Camry accelerated to 70 mph" {
			t.Logf("combo result: %s", combo)
		}
	})

	t.Run("ScriptClassOverridesGoMethod", func(t *testing.T) {
		p := New()

		// Create Go class with a method
		goClass := &object.Class{
			Name: "GoBase",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Null{}
					},
				},
				"process": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "Go process"}
					},
				},
			},
		}

		p.RegisterLibrary(object.NewLibrary("test", nil, map[string]object.Object{
			"GoBase": goClass,
		}, "Test library"))

		// Scriptling class overrides Go method but can still call it via super()
		_, err := p.Eval(`
import test

class Derived(test.GoBase):
    def process(self):
        parent_result = super().process()
        return "Derived: " + parent_result

obj = Derived()
result = obj.process()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, _ := p.GetVarAsString("result")
		if result != "Derived: Go process" {
			t.Errorf("expected 'Derived: Go process', got %s", result)
		}
	})
}
