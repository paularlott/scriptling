package scriptling

import (
	"context"
	"sync"
	"testing"

	"github.com/paularlott/scriptling/object"
)

type testConfig struct {
	Value string
}

func TestLibraryInstantiateBasic(t *testing.T) {
	// Create a library template
	builder := object.NewLibraryBuilder("testlib", "Test library with instance data")
	
	builder.Function("get_value", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		return &object.String{Value: "template"}
	})
	
	template := builder.Build()
	
	// Instantiate with different configs
	config1 := testConfig{Value: "instance1"}
	config2 := testConfig{Value: "instance2"}
	
	lib1 := template.Instantiate(config1)
	lib2 := template.Instantiate(config2)
	
	// Verify instances have correct data
	if lib1.InstanceData().(testConfig).Value != "instance1" {
		t.Errorf("Expected instance1, got %v", lib1.InstanceData())
	}
	
	if lib2.InstanceData().(testConfig).Value != "instance2" {
		t.Errorf("Expected instance2, got %v", lib2.InstanceData())
	}
	
	// Verify template has no instance data
	if template.InstanceData() != nil {
		t.Errorf("Template should have no instance data, got %v", template.InstanceData())
	}
	
	// Verify names are preserved
	if lib1.Name() != "testlib" {
		t.Errorf("Expected testlib, got %s", lib1.Name())
	}
	
	if lib2.Name() != "testlib" {
		t.Errorf("Expected testlib, got %s", lib2.Name())
	}
}

func TestLibraryInstantiateFunctionAccessesInstanceData(t *testing.T) {
	// Create two interpreters
	p1 := New()
	p2 := New()
	
	// Create a library template with function that accesses instance data from context
	builder := object.NewLibraryBuilder("configlib", "Library with config")
	
	builder.Function("get_config_value", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		// Access instance data from context (injected by Instantiate)
		instanceData := object.InstanceDataFromContext(ctx)
		if instanceData == nil {
			return &object.String{Value: "no-config"}
		}
		config := instanceData.(testConfig)
		return &object.String{Value: config.Value}
	})
	
	template := builder.Build()
	
	// Instantiate with different configs
	config1 := testConfig{Value: "config1-value"}
	config2 := testConfig{Value: "config2-value"}
	
	lib1 := template.Instantiate(config1)
	lib2 := template.Instantiate(config2)
	
	// Register to different interpreters
	p1.RegisterLibrary(lib1)
	p2.RegisterLibrary(lib2)
	
	// Import in both
	if err := p1.Import("configlib"); err != nil {
		t.Fatalf("Failed to import in p1: %v", err)
	}
	
	if err := p2.Import("configlib"); err != nil {
		t.Fatalf("Failed to import in p2: %v", err)
	}
	
	// Call function in p1
	result1, err := p1.Eval("configlib.get_config_value()")
	if err != nil {
		t.Fatalf("Failed to call get_config_value in p1: %v", err)
	}
	
	val1, _ := result1.AsString()
	if val1 != "config1-value" {
		t.Errorf("Expected config1-value from p1, got %s", val1)
	}
	
	// Call function in p2
	result2, err := p2.Eval("configlib.get_config_value()")
	if err != nil {
		t.Fatalf("Failed to call get_config_value in p2: %v", err)
	}
	
	val2, _ := result2.AsString()
	if val2 != "config2-value" {
		t.Errorf("Expected config2-value from p2, got %s", val2)
	}
}

func TestLibraryInstantiateNoDataCrossover(t *testing.T) {
	// Create library that retrieves instance data from context
	builder := object.NewLibraryBuilder("datalib", "Library that uses instance data")
	
	builder.Function("get_data", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		instanceData := object.InstanceDataFromContext(ctx)
		if instanceData == nil {
			return &object.String{Value: "no-data"}
		}
		config := instanceData.(testConfig)
		return &object.String{Value: config.Value}
	})
	
	template := builder.Build()
	
	// Create 3 instances with different data
	lib1 := template.Instantiate(testConfig{Value: "data1"})
	lib2 := template.Instantiate(testConfig{Value: "data2"})
	lib3 := template.Instantiate(testConfig{Value: "data3"})
	
	// Create interpreters and register different instances
	p1 := New()
	p2 := New()
	p3 := New()
	
	p1.RegisterLibrary(lib1)
	p2.RegisterLibrary(lib2)
	p3.RegisterLibrary(lib3)
	
	// Import in all
	if err := p1.Import("datalib"); err != nil {
		t.Fatalf("Failed to import in p1: %v", err)
	}
	if err := p2.Import("datalib"); err != nil {
		t.Fatalf("Failed to import in p2: %v", err)
	}
	if err := p3.Import("datalib"); err != nil {
		t.Fatalf("Failed to import in p3: %v", err)
	}
	
	// Call functions and verify each gets its own data
	result1, err1 := p1.Eval("datalib.get_data()")
	if err1 != nil {
		t.Fatalf("p1 failed: %v", err1)
	}
	val1, _ := result1.AsString()
	if val1 != "data1" {
		t.Errorf("Expected data1 from p1, got %s", val1)
	}
	
	result2, err2 := p2.Eval("datalib.get_data()")
	if err2 != nil {
		t.Fatalf("p2 failed: %v", err2)
	}
	val2, _ := result2.AsString()
	if val2 != "data2" {
		t.Errorf("Expected data2 from p2, got %s", val2)
	}
	
	result3, err3 := p3.Eval("datalib.get_data()")
	if err3 != nil {
		t.Fatalf("p3 failed: %v", err3)
	}
	val3, _ := result3.AsString()
	if val3 != "data3" {
		t.Errorf("Expected data3 from p3, got %s", val3)
	}
	
	// Call again in different order to verify no crossover
	result2b, _ := p2.Eval("datalib.get_data()")
	val2b, _ := result2b.AsString()
	if val2b != "data2" {
		t.Errorf("Expected data2 from p2 (second call), got %s", val2b)
	}
	
	result1b, _ := p1.Eval("datalib.get_data()")
	val1b, _ := result1b.AsString()
	if val1b != "data1" {
		t.Errorf("Expected data1 from p1 (second call), got %s", val1b)
	}
}

func TestLibraryInstantiateConcurrent(t *testing.T) {
	// Create library that retrieves instance data from context
	builder := object.NewLibraryBuilder("conclib", "Library for concurrent testing")
	
	builder.Function("get_data", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		instanceData := object.InstanceDataFromContext(ctx)
		if instanceData == nil {
			return &object.String{Value: "no-data"}
		}
		config := instanceData.(testConfig)
		return &object.String{Value: config.Value}
	})
	
	template := builder.Build()
	
	// Create 3 instances with different data
	lib1 := template.Instantiate(testConfig{Value: "concurrent1"})
	lib2 := template.Instantiate(testConfig{Value: "concurrent2"})
	lib3 := template.Instantiate(testConfig{Value: "concurrent3"})
	
	// Create interpreters and register different instances
	p1 := New()
	p2 := New()
	p3 := New()
	
	p1.RegisterLibrary(lib1)
	p2.RegisterLibrary(lib2)
	p3.RegisterLibrary(lib3)
	
	// Import in all
	if err := p1.Import("conclib"); err != nil {
		t.Fatalf("Failed to import in p1: %v", err)
	}
	if err := p2.Import("conclib"); err != nil {
		t.Fatalf("Failed to import in p2: %v", err)
	}
	if err := p3.Import("conclib"); err != nil {
		t.Fatalf("Failed to import in p3: %v", err)
	}
	
	// Run concurrent calls
	var wg sync.WaitGroup
	errors := make(chan string, 300)
	
	// Run 100 calls on each interpreter concurrently
	for i := 0; i < 100; i++ {
		wg.Add(3)
		
		go func() {
			defer wg.Done()
			result, err := p1.Eval("conclib.get_data()")
			if err != nil {
				errors <- err.Error()
				return
			}
			val, _ := result.AsString()
			if val != "concurrent1" {
				errors <- "p1 got wrong value: " + val
			}
		}()
		
		go func() {
			defer wg.Done()
			result, err := p2.Eval("conclib.get_data()")
			if err != nil {
				errors <- err.Error()
				return
			}
			val, _ := result.AsString()
			if val != "concurrent2" {
				errors <- "p2 got wrong value: " + val
			}
		}()
		
		go func() {
			defer wg.Done()
			result, err := p3.Eval("conclib.get_data()")
			if err != nil {
				errors <- err.Error()
				return
			}
			val, _ := result.AsString()
			if val != "concurrent3" {
				errors <- "p3 got wrong value: " + val
			}
		}()
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent test error: %v", err)
	}
}
