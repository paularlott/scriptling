package scriptling

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

func TestCreateInstance(t *testing.T) {
	t.Run("simple_class", func(t *testing.T) {
		p := New()

		// Define a simple class
		_, err := p.Eval(`
class Counter:
    def __init__(self, start=0):
        self.value = start
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		// Create an instance
		instance, err := p.CreateInstance("Counter", 10)
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}

		// Verify it's an instance
		inst, ok := instance.(*object.Instance)
		if !ok {
			t.Fatalf("expected Instance, got %T", instance)
		}

		// Check the initial value
		valueObj := inst.Fields["value"]
		value, objErr := valueObj.AsInt()
		if objErr != nil || value != 10 {
			t.Errorf("expected value=10, got %v", value)
		}
	})

	t.Run("class_not_found", func(t *testing.T) {
		p := New()

		_, err := p.CreateInstance("NonExistent")
		if err == nil {
			t.Error("expected error for non-existent class")
		}
	})

	t.Run("not_a_class", func(t *testing.T) {
		p := New()

		// Set a non-class variable
		p.SetVar("not_a_class", 42)

		_, err := p.CreateInstance("not_a_class")
		if err == nil {
			t.Error("expected error when trying to instantiate non-class")
		}
	})

	t.Run("with_kwargs", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Person:
    def __init__(self, name, age=0):
        self.name = name
        self.age = age
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		instance, err := p.CreateInstance("Person", "Alice", Kwargs{"age": 30})
		if err != nil {
			t.Fatalf("CreateInstance with kwargs failed: %v", err)
		}

		inst := instance.(*object.Instance)
		nameObj := inst.Fields["name"]
		name, _ := nameObj.AsString()
		if name != "Alice" {
			t.Errorf("expected name=Alice, got %s", name)
		}

		ageObj := inst.Fields["age"]
		age, _ := ageObj.AsInt()
		if age != 30 {
			t.Errorf("expected age=30, got %d", age)
		}
	})

	t.Run("with_context", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Simple:
    def __init__(self):
        self.value = 42
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		instance, err := p.CreateInstanceWithContext(ctx, "Simple")
		if err != nil {
			t.Fatalf("CreateInstanceWithContext failed: %v", err)
		}

		inst := instance.(*object.Instance)
		valueObj := inst.Fields["value"]
		value, _ := valueObj.AsInt()
		if value != 42 {
			t.Errorf("expected value=42, got %d", value)
		}
	})
}

func TestCallMethod(t *testing.T) {
	t.Run("simple_method", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Counter:
    def __init__(self, start=0):
        self.value = start
    
    def increment(self):
        self.value = self.value + 1
        return self.value
    
    def get(self):
        return self.value
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		instance, err := p.CreateInstance("Counter", 10)
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}

		// Call increment method
		result, err := p.CallMethod(instance, "increment")
		if err != nil {
			t.Fatalf("CallMethod failed: %v", err)
		}

		value, objErr := result.AsInt()
		if objErr != nil || value != 11 {
			t.Errorf("expected 11, got %v", value)
		}

		// Call get method
		result, err = p.CallMethod(instance, "get")
		if err != nil {
			t.Fatalf("CallMethod failed: %v", err)
		}

		value, objErr = result.AsInt()
		if objErr != nil || value != 11 {
			t.Errorf("expected 11, got %v", value)
		}
	})

	t.Run("method_with_args", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Calculator:
    def __init__(self):
        self.result = 0
    
    def add(self, a, b):
        self.result = a + b
        return self.result
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		instance, err := p.CreateInstance("Calculator")
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}

		result, err := p.CallMethod(instance, "add", 15, 27)
		if err != nil {
			t.Fatalf("CallMethod failed: %v", err)
		}

		sum, objErr := result.AsInt()
		if objErr != nil || sum != 42 {
			t.Errorf("expected 42, got %v", sum)
		}
	})

	t.Run("method_with_kwargs", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Formatter:
    def format(self, text, prefix=">>", suffix="<<"):
        return prefix + " " + text + " " + suffix
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		instance, err := p.CreateInstance("Formatter")
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}

		result, err := p.CallMethod(instance, "format", "hello",
			Kwargs{
				"prefix": "##",
				"suffix": "##",
			})
		if err != nil {
			t.Fatalf("CallMethod with kwargs failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil || text != "## hello ##" {
			t.Errorf("expected '## hello ##', got %s", text)
		}
	})

	t.Run("method_not_found", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Simple:
    def __init__(self):
        pass
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		instance, err := p.CreateInstance("Simple")
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}

		_, err = p.CallMethod(instance, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent method")
		}
	})

	t.Run("not_an_instance", func(t *testing.T) {
		p := New()

		// Try to call method on a non-instance object
		notInstance := object.NewInteger(42)
		_, err := p.CallMethod(notInstance, "some_method")
		if err == nil {
			t.Error("expected error when calling method on non-instance")
		}
	})

	t.Run("with_context", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Worker:
    def work(self):
        return "done"
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		instance, err := p.CreateInstance("Worker")
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := p.CallMethodWithContext(ctx, instance, "work")
		if err != nil {
			t.Fatalf("CallMethodWithContext failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil || text != "done" {
			t.Errorf("expected 'done', got %s", text)
		}
	})

	t.Run("method_returning_different_types", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class DataHolder:
    def get_int(self):
        return 42
    
    def get_string(self):
        return "hello"
    
    def get_float(self):
        return 3.14
    
    def get_bool(self):
        return True
    
    def get_list(self):
        return [1, 2, 3]
    
    def get_dict(self):
        return {"key": "value"}
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		instance, err := p.CreateInstance("DataHolder")
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}

		// Test int
		result, _ := p.CallMethod(instance, "get_int")
		intVal, _ := result.AsInt()
		if intVal != 42 {
			t.Errorf("expected 42, got %d", intVal)
		}

		// Test string
		result, _ = p.CallMethod(instance, "get_string")
		strVal, _ := result.AsString()
		if strVal != "hello" {
			t.Errorf("expected 'hello', got %s", strVal)
		}

		// Test float
		result, _ = p.CallMethod(instance, "get_float")
		floatVal, _ := result.AsFloat()
		if floatVal != 3.14 {
			t.Errorf("expected 3.14, got %f", floatVal)
		}

		// Test bool
		result, _ = p.CallMethod(instance, "get_bool")
		boolVal, _ := result.AsBool()
		if !boolVal {
			t.Error("expected true")
		}

		// Test list
		result, _ = p.CallMethod(instance, "get_list")
		listVal, _ := result.AsList()
		if len(listVal) != 3 {
			t.Errorf("expected list of length 3, got %d", len(listVal))
		}

		// Test dict
		result, _ = p.CallMethod(instance, "get_dict")
		dictVal, _ := result.AsDict()
		if val, ok := dictVal["key"]; !ok {
			t.Error("expected key 'key' in dict")
		} else {
			keyVal, _ := val.AsString()
			if keyVal != "value" {
				t.Errorf("expected 'value', got %s", keyVal)
			}
		}
	})
}

func TestCreateInstanceAndCallMethodIntegration(t *testing.T) {
	t.Run("counter_workflow", func(t *testing.T) {
		p := New()

		// Define a counter class
		_, err := p.Eval(`
class Counter:
    def __init__(self, start=0):
        self.value = start
    
    def increment(self, amount=1):
        self.value = self.value + amount
        return self.value
    
    def decrement(self, amount=1):
        self.value = self.value - amount
        return self.value
    
    def get(self):
        return self.value
    
    def reset(self, new_value=0):
        self.value = new_value
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		// Create instance with initial value
		instance, err := p.CreateInstance("Counter", 100)
		if err != nil {
			t.Fatalf("CreateInstance failed: %v", err)
		}

		// Get initial value
		result, _ := p.CallMethod(instance, "get")
		value, _ := result.AsInt()
		if value != 100 {
			t.Errorf("expected initial value 100, got %d", value)
		}

		// Increment by default (1)
		result, _ = p.CallMethod(instance, "increment")
		value, _ = result.AsInt()
		if value != 101 {
			t.Errorf("expected 101, got %d", value)
		}

		// Increment by 10
		result, _ = p.CallMethod(instance, "increment", Kwargs{"amount": 10})
		value, _ = result.AsInt()
		if value != 111 {
			t.Errorf("expected 111, got %d", value)
		}

		// Decrement by 5
		result, _ = p.CallMethod(instance, "decrement", Kwargs{"amount": 5})
		value, _ = result.AsInt()
		if value != 106 {
			t.Errorf("expected 106, got %d", value)
		}

		// Reset to 50
		p.CallMethod(instance, "reset", Kwargs{"new_value": 50})
		result, _ = p.CallMethod(instance, "get")
		value, _ = result.AsInt()
		if value != 50 {
			t.Errorf("expected 50 after reset, got %d", value)
		}
	})

	t.Run("multiple_instances", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Account:
    def __init__(self, name, balance=0):
        self.name = name
        self.balance = balance
    
    def deposit(self, amount):
        self.balance = self.balance + amount
        return self.balance
    
    def get_balance(self):
        return self.balance
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		// Create two separate instances
		account1, _ := p.CreateInstance("Account", "Alice", Kwargs{"balance": 100})
		account2, _ := p.CreateInstance("Account", "Bob", Kwargs{"balance": 200})

		// Deposit to account1
		p.CallMethod(account1, "deposit", 50)

		// Check balances are independent
		result1, _ := p.CallMethod(account1, "get_balance")
		balance1, _ := result1.AsInt()
		if balance1 != 150 {
			t.Errorf("expected account1 balance 150, got %d", balance1)
		}

		result2, _ := p.CallMethod(account2, "get_balance")
		balance2, _ := result2.AsInt()
		if balance2 != 200 {
			t.Errorf("expected account2 balance 200, got %d", balance2)
		}
	})

	t.Run("store_and_retrieve_instance", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
class Box:
    def __init__(self, content):
        self.content = content
    
    def get_content(self):
        return self.content
`)
		if err != nil {
			t.Fatalf("failed to define class: %v", err)
		}

		// Create instance
		instance, _ := p.CreateInstance("Box", "treasure")

		// Store it in the environment
		p.SetObjectVar("my_box", instance)

		// Retrieve it
		retrievedObj, _ := p.env.Get("my_box")

		// Call method on retrieved instance
		result, err := p.CallMethod(retrievedObj, "get_content")
		if err != nil {
			t.Fatalf("CallMethod on retrieved instance failed: %v", err)
		}

		content, _ := result.AsString()
		if content != "treasure" {
			t.Errorf("expected 'treasure', got %s", content)
		}
	})
}
