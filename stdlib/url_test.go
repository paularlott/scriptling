package stdlib

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestURLLibrary(t *testing.T) {
	lib := URLLibrary

	t.Run("quote basic", func(t *testing.T) {
		quote := lib.Functions()["quote"]
		result := quote.Fn(context.Background(), &object.String{Value: "hello world"})

		if str, ok := result.(*object.String); ok {
			expected := "hello%20world"
			if str.Value != expected {
				t.Errorf("Expected %q, got %q", expected, str.Value)
			}
		} else {
			t.Errorf("Expected string, got %T", result)
		}
	})

	t.Run("unquote basic", func(t *testing.T) {
		unquote := lib.Functions()["unquote"]
		result := unquote.Fn(context.Background(), &object.String{Value: "hello%20world"})

		if str, ok := result.(*object.String); ok {
			expected := "hello world"
			if str.Value != expected {
				t.Errorf("Expected %q, got %q", expected, str.Value)
			}
		} else {
			t.Errorf("Expected string, got %T", result)
		}
	})

	t.Run("urlparse basic", func(t *testing.T) {
		urlparse := lib.Functions()["urlparse"]
		result := urlparse.Fn(context.Background(), &object.String{Value: "https://user:pass@example.com:8080/path?query=value#fragment"})

		if dict, ok := result.(*object.Dict); ok {
			expected := map[string]string{
				"scheme":   "https",
				"host":     "example.com:8080",
				"path":     "/path",
				"query":    "query=value",
				"fragment": "fragment",
			}
			for key, expectedVal := range expected {
				if pair, exists := dict.Pairs[key]; exists {
					if str, ok := pair.Value.(*object.String); ok {
						if str.Value != expectedVal {
							t.Errorf("Expected %s=%q, got %q", key, expectedVal, str.Value)
						}
					} else {
						t.Errorf("Expected string for %s, got %T", key, pair.Value)
					}
				} else {
					t.Errorf("Expected key %s not found", key)
				}
			}
		} else {
			t.Errorf("Expected dict, got %T", result)
		}
	})

	t.Run("urlunparse basic", func(t *testing.T) {
		urlunparse := lib.Functions()["urlunparse"]
		components := &object.Dict{
			Pairs: map[string]object.DictPair{
				"scheme": {
					Key:   &object.String{Value: "scheme"},
					Value: &object.String{Value: "https"},
				},
				"host": {
					Key:   &object.String{Value: "host"},
					Value: &object.String{Value: "api.example.com"},
				},
				"path": {
					Key:   &object.String{Value: "path"},
					Value: &object.String{Value: "/v1/users"},
				},
				"query": {
					Key:   &object.String{Value: "query"},
					Value: &object.String{Value: "limit=10&offset=0"},
				},
				"fragment": {
					Key:   &object.String{Value: "fragment"},
					Value: &object.String{Value: "section1"},
				},
			},
		}
		result := urlunparse.Fn(context.Background(), components)

		if str, ok := result.(*object.String); ok {
			expected := "https://api.example.com/v1/users?limit=10&offset=0#section1"
			if str.Value != expected {
				t.Errorf("Expected %q, got %q", expected, str.Value)
			}
		} else {
			t.Errorf("Expected string, got %T", result)
		}
	})

	t.Run("urljoin basic", func(t *testing.T) {
		urljoin := lib.Functions()["urljoin"]
		result := urljoin.Fn(context.Background(),
			&object.String{Value: "https://api.example.com/v1"},
			&object.String{Value: "/users/123"})

		if str, ok := result.(*object.String); ok {
			expected := "https://api.example.com/users/123"
			if str.Value != expected {
				t.Errorf("Expected %q, got %q", expected, str.Value)
			}
		} else {
			t.Errorf("Expected string, got %T", result)
		}
	})

	t.Run("urlsplit basic", func(t *testing.T) {
		urlsplit := lib.Functions()["urlsplit"]
		result := urlsplit.Fn(context.Background(), &object.String{Value: "https://example.com/path?query=value#fragment"})

		if list, ok := result.(*object.List); ok {
			expected := []string{"https", "example.com", "/path", "query=value", "fragment"}
			if len(list.Elements) != len(expected) {
				t.Errorf("Expected %d elements, got %d", len(expected), len(list.Elements))
			}
			for i, exp := range expected {
				if str, ok := list.Elements[i].(*object.String); ok {
					if str.Value != exp {
						t.Errorf("Element %d: expected %q, got %q", i, exp, str.Value)
					}
				} else {
					t.Errorf("Element %d: expected string, got %T", i, list.Elements[i])
				}
			}
		} else {
			t.Errorf("Expected list, got %T", result)
		}
	})

	t.Run("urlunsplit basic", func(t *testing.T) {
		urlunsplit := lib.Functions()["urlunsplit"]
		components := &object.List{
			Elements: []object.Object{
				&object.String{Value: "https"},
				&object.String{Value: "example.com"},
				&object.String{Value: "/path"},
				&object.String{Value: "query=value"},
				&object.String{Value: "fragment"},
			},
		}
		result := urlunsplit.Fn(context.Background(), components)

		if str, ok := result.(*object.String); ok {
			expected := "https://example.com/path?query=value#fragment"
			if str.Value != expected {
				t.Errorf("Expected %q, got %q", expected, str.Value)
			}
		} else {
			t.Errorf("Expected string, got %T", result)
		}
	})

	t.Run("parse_qs single value", func(t *testing.T) {
		parseQs := lib.Functions()["parse_qs"]
		result := parseQs.Fn(context.Background(), &object.String{Value: "key=value"})

		if dict, ok := result.(*object.Dict); ok {
			if pair, exists := dict.Pairs["key"]; exists {
				if list, ok := pair.Value.(*object.List); ok {
					if len(list.Elements) == 1 {
						if str, ok := list.Elements[0].(*object.String); ok {
							if str.Value != "value" {
								t.Errorf("Expected 'value', got %q", str.Value)
							}
						} else {
							t.Errorf("Expected string element, got %T", list.Elements[0])
						}
					} else {
						t.Errorf("Expected 1 element, got %d", len(list.Elements))
					}
				} else {
					t.Errorf("Expected list, got %T", pair.Value)
				}
			} else {
				t.Errorf("Expected key 'key' not found")
			}
		} else {
			t.Errorf("Expected dict, got %T", result)
		}
	})

	t.Run("parse_qs multiple values", func(t *testing.T) {
		parseQs := lib.Functions()["parse_qs"]
		result := parseQs.Fn(context.Background(), &object.String{Value: "key=value1&key=value2"})

		if dict, ok := result.(*object.Dict); ok {
			if pair, exists := dict.Pairs["key"]; exists {
				if list, ok := pair.Value.(*object.List); ok {
					expected := []string{"value1", "value2"}
					if len(list.Elements) != len(expected) {
						t.Errorf("Expected %d elements, got %d", len(expected), len(list.Elements))
					}
					for i, exp := range expected {
						if str, ok := list.Elements[i].(*object.String); ok {
							if str.Value != exp {
								t.Errorf("Element %d: expected %q, got %q", i, exp, str.Value)
							}
						} else {
							t.Errorf("Element %d: expected string, got %T", i, list.Elements[i])
						}
					}
				} else {
					t.Errorf("Expected list, got %T", pair.Value)
				}
			} else {
				t.Errorf("Expected key 'key' not found")
			}
		} else {
			t.Errorf("Expected dict, got %T", result)
		}
	})

	t.Run("urlencode dict", func(t *testing.T) {
		urlencode := lib.Functions()["urlencode"]
		dict := &object.Dict{
			Pairs: map[string]object.DictPair{
				"key": {
					Key:   &object.String{Value: "key"},
					Value: &object.String{Value: "value"},
				},
				"foo": {
					Key:   &object.String{Value: "foo"},
					Value: &object.String{Value: "bar"},
				},
			},
		}
		result := urlencode.Fn(context.Background(), dict)

		if str, ok := result.(*object.String); ok {
			// Check that it contains the expected key-value pairs
			if !containsKV(str.Value, "key=value") || !containsKV(str.Value, "foo=bar") {
				t.Errorf("Expected encoded string to contain 'key=value' and 'foo=bar', got %q", str.Value)
			}
		} else {
			t.Errorf("Expected string, got %T", result)
		}
	})

	t.Run("urlencode with list values", func(t *testing.T) {
		urlencode := lib.Functions()["urlencode"]
		dict := &object.Dict{
			Pairs: map[string]object.DictPair{
				"key": {
					Key: &object.String{Value: "key"},
					Value: &object.List{
						Elements: []object.Object{
							&object.String{Value: "value1"},
							&object.String{Value: "value2"},
						},
					},
				},
			},
		}
		result := urlencode.Fn(context.Background(), dict)

		if str, ok := result.(*object.String); ok {
			// Should contain key=value1&key=value2
			if !containsKV(str.Value, "key=value1") || !containsKV(str.Value, "key=value2") {
				t.Errorf("Expected encoded string to contain 'key=value1' and 'key=value2', got %q", str.Value)
			}
		} else {
			t.Errorf("Expected string, got %T", result)
		}
	})
}

// Helper function to check if a query string contains a key-value pair
func containsKV(query, kv string) bool {
	// Simple check - in a real implementation you'd parse the query string
	return len(query) > 0 && (query == kv || contains(query, kv+"&") || contains(query, "&"+kv) || contains(query, kv))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || strings.Contains(s, substr)))
}
