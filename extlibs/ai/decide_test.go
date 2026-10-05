package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	mcpai "github.com/paularlott/mcp/ai"
	openaiapi "github.com/paularlott/mcp/ai/openai"
	"github.com/paularlott/scriptling/object"
)

// newDecisionInstance builds a script-facing client instance backed by a real
// Ollama provider pointing at base (an httptest server in tests).
func newDecisionInstance(t *testing.T, base string) *object.Instance {
	t.Helper()
	client, err := mcpai.NewClient(mcpai.Config{
		Provider: mcpai.ProviderOllama,
		Config:   openaiapi.Config{BaseURL: base},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
		"_client": &object.ClientWrapper{Client: &ClientInstance{client: client}},
	})
}

func dictField(t *testing.T, obj object.Object, key string) object.Object {
	t.Helper()
	d, ok := obj.(*object.Dict)
	if !ok {
		t.Fatalf("expected dict, got %v", obj.Type())
	}
	pair, found := d.GetByString(key)
	if !found {
		t.Fatalf("dict has no %q key: %s", key, d.Inspect())
	}
	return pair.Value
}

func TestDecideMethodEndToEnd(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"model": "clef-flash",
			"answers": map[string]any{
				"label": map[string]any{
					"type":          "choice",
					"choice":        "bug",
					"probabilities": map[string]float64{"bug": 0.9781, "billing": 0.0219},
					"confidence":    0.8906,
				},
				"urgent": map[string]any{"type": "noul", "noul": 0.959},
				"tie": map[string]any{
					// Uniform distribution: confidence is exactly 0 and must
					// still be present — dropping the key would hide that.
					"type":          "choice",
					"choice":        "a",
					"probabilities": map[string]float64{"a": 0.5, "b": 0.5},
					"confidence":    0.0,
				},
			},
			"usage": map[string]int{"input_tokens": 174, "output_tokens": 1},
		})
	}))
	defer srv.Close()

	instance := newDecisionInstance(t, srv.URL)

	questions := object.NewStringDict(map[string]object.Object{
		"label": object.NewStringDict(map[string]object.Object{
			"type":         object.NewString("choice"),
			"instructions": object.NewString("Which label fits?"),
			"criteria": object.NewStringDict(map[string]object.Object{
				"bug":     object.NewString("Errors"),
				"billing": object.NewString("Payments"),
			}),
		}),
		"urgent": object.NewStringDict(map[string]object.Object{
			"type":         object.NewString("noul"),
			"instructions": object.NewString("Page a human?"),
		}),
		"tie": object.NewStringDict(map[string]object.Object{
			"type":         object.NewString("choice"),
			"instructions": object.NewString("Which option?"),
			"criteria": object.NewStringDict(map[string]object.Object{
				"a": object.NewString("First"),
				"b": object.NewString("Second"),
			}),
		}),
	})
	kwargs := object.NewKwargs(map[string]object.Object{
		"questions":  questions,
		"keep_alive": object.NewString("5m"),
		"images": &object.List{Elements: []object.Object{
			object.NewBytesFromString("png-bytes"),
			object.NewString("c3RyaW5nLWJhc2U2NA=="),
		}},
	})

	// A dict state exercises the any-binder's object → JSON conversion.
	state := object.NewStringDict(map[string]object.Object{
		"service": object.NewString("checkout"),
		"errors":  object.NewInteger(500),
	})
	result := decideMethod(instance, context.Background(), kwargs, "clef-flash", state)
	if result.Type() == object.ERROR_OBJ {
		t.Fatalf("decide failed: %s", result.Inspect())
	}

	if gotPath != "/v1/systemone" {
		t.Errorf("request path = %q, want /v1/systemone", gotPath)
	}
	sentState, _ := gotBody["state"].(map[string]any)
	if sentState["service"] != "checkout" || sentState["errors"] != float64(500) {
		t.Errorf("state = %v", gotBody["state"])
	}
	if gotBody["model"] != "clef-flash" {
		t.Errorf("request = %v", gotBody)
	}
	if gotBody["keep_alive"] != "5m" {
		t.Errorf("keep_alive = %v", gotBody["keep_alive"])
	}
	sentImages, _ := gotBody["images"].([]any)
	if len(sentImages) != 2 || sentImages[0] != "cG5nLWJ5dGVz" || sentImages[1] != "c3RyaW5nLWJhc2U2NA==" {
		t.Errorf("images = %v (bytes must arrive base64-encoded)", sentImages)
	}
	sentQuestions, _ := gotBody["questions"].(map[string]any)
	if len(sentQuestions) != 3 {
		t.Errorf("questions sent = %v", sentQuestions)
	}

	if model := dictField(t, result, "model").Inspect(); model != "clef-flash" {
		t.Errorf("model = %q", model)
	}
	answers := dictField(t, result, "answers")
	label := dictField(t, answers, "label")
	if choice := dictField(t, label, "choice").Inspect(); choice != "bug" {
		t.Errorf("label choice = %q", choice)
	}
	probs := dictField(t, label, "probabilities")
	if p := dictField(t, probs, "bug").Inspect(); p != "0.9781" {
		t.Errorf("label probabilities[bug] = %s", p)
	}
	urgent := dictField(t, answers, "urgent")
	if noul := dictField(t, urgent, "noul").Inspect(); noul != "0.959" {
		t.Errorf("urgent noul = %s", noul)
	}
	// A noul answer without a server-sent confidence must not grow one.
	urgentDict := urgent.(*object.Dict)
	if _, has := urgentDict.GetByString("confidence"); has {
		t.Errorf("urgent answer should have no confidence key: %s", urgent.Inspect())
	}
	// Uniform choice: confidence 0.0 must survive as an explicit zero.
	tie := dictField(t, answers, "tie")
	if c := dictField(t, tie, "confidence"); c.Inspect() != "0.0" {
		t.Errorf("tie confidence = %s, want 0.0", c.Inspect())
	}
	usage := dictField(t, result, "usage")
	if it := dictField(t, usage, "input_tokens").Inspect(); it != "174" {
		t.Errorf("input_tokens = %s", it)
	}
}

func TestDecideMethodProviderNotSupported(t *testing.T) {
	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
		"_client": &object.ClientWrapper{Client: &ClientInstance{client: timeoutMockClient{}}},
	})
	kwargs := object.NewKwargs(map[string]object.Object{
		"questions": object.NewStringDict(map[string]object.Object{
			"q": object.NewStringDict(map[string]object.Object{
				"type":         object.NewString("noul"),
				"instructions": object.NewString("yes?"),
			}),
		}),
	})
	result := decideMethod(instance, context.Background(), kwargs, "gpt-4", "state")
	if result.Type() != object.ERROR_OBJ {
		t.Fatalf("expected error, got %v", result)
	}
	if !strings.Contains(result.Inspect(), "decision models are not supported by provider 'mock'") {
		t.Errorf("error = %s", result.Inspect())
	}
}

func TestDecideMethodQuestionValidation(t *testing.T) {
	instance := newDecisionInstance(t, "http://127.0.0.1:1") // never dialed: validation fails first

	badQuestion := func(fields map[string]object.Object) object.Object {
		return object.NewStringDict(fields)
	}
	cases := []struct {
		name     string
		kwargs   object.Kwargs
		wantPart string
	}{
		{
			name:     "missing questions",
			kwargs:   object.NewKwargs(nil),
			wantPart: "questions is required",
		},
		{
			name: "empty questions",
			kwargs: object.NewKwargs(map[string]object.Object{
				"questions": object.NewStringDict(nil),
			}),
			wantPart: "1 to 64",
		},
		{
			name: "missing type",
			kwargs: object.NewKwargs(map[string]object.Object{
				"questions": object.NewStringDict(map[string]object.Object{
					"q": badQuestion(map[string]object.Object{"instructions": object.NewString("x")}),
				}),
			}),
			wantPart: `missing "type"`,
		},
		{
			name: "unknown type",
			kwargs: object.NewKwargs(map[string]object.Object{
				"questions": object.NewStringDict(map[string]object.Object{
					"q": badQuestion(map[string]object.Object{
						"type":         object.NewString("essay"),
						"instructions": object.NewString("x"),
					}),
				}),
			}),
			wantPart: `"choice", "noul" or "score"`,
		},
		{
			name: "choice criteria too few",
			kwargs: object.NewKwargs(map[string]object.Object{
				"questions": object.NewStringDict(map[string]object.Object{
					"q": badQuestion(map[string]object.Object{
						"type":         object.NewString("choice"),
						"instructions": object.NewString("x"),
						"criteria": object.NewStringDict(map[string]object.Object{
							"a": object.NewString("only"),
						}),
					}),
				}),
			}),
			wantPart: "2 to 26",
		},
		{
			name: "noul criteria bad key",
			kwargs: object.NewKwargs(map[string]object.Object{
				"questions": object.NewStringDict(map[string]object.Object{
					"q": badQuestion(map[string]object.Object{
						"type":         object.NewString("noul"),
						"instructions": object.NewString("x"),
						"criteria": object.NewStringDict(map[string]object.Object{
							"maybe": object.NewString("desc"),
						}),
					}),
				}),
			}),
			wantPart: `"false" and "true"`,
		},
		{
			name: "score criteria not a list",
			kwargs: object.NewKwargs(map[string]object.Object{
				"questions": object.NewStringDict(map[string]object.Object{
					"q": badQuestion(map[string]object.Object{
						"type":         object.NewString("score"),
						"instructions": object.NewString("x"),
						"criteria":     object.NewString("not-a-list"),
					}),
				}),
			}),
			wantPart: "ordered list",
		},
	}

	for _, tc := range cases {
		result := decideMethod(instance, context.Background(), tc.kwargs, "clef-flash", "state")
		if result.Type() != object.ERROR_OBJ {
			t.Errorf("%s: expected error, got %v", tc.name, result)
			continue
		}
		if !strings.Contains(result.Inspect(), tc.wantPart) {
			t.Errorf("%s: error %q should contain %q", tc.name, result.Inspect(), tc.wantPart)
		}
	}
}

// TestLiveDecisionOllama runs against a real Ollama server when
// SCRIPTLING_OLLAMA_DECISION holds its base URL (model from
// SCRIPTLING_OLLAMA_DECISION_MODEL, default clef-flash); skipped otherwise.
func TestLiveDecisionOllama(t *testing.T) {
	base := os.Getenv("SCRIPTLING_OLLAMA_DECISION")
	if base == "" {
		t.Skip("SCRIPTLING_OLLAMA_DECISION not set")
	}
	model := os.Getenv("SCRIPTLING_OLLAMA_DECISION_MODEL")
	if model == "" {
		model = "clef-flash"
	}

	instance := newDecisionInstance(t, base)
	kwargs := object.NewKwargs(map[string]object.Object{
		"questions": object.NewStringDict(map[string]object.Object{
			"label": object.NewStringDict(map[string]object.Object{
				"type":         object.NewString("choice"),
				"instructions": object.NewString("Which label fits this support ticket?"),
				"criteria": object.NewStringDict(map[string]object.Object{
					"billing": object.NewString("Payments, refunds and invoices"),
					"bug":     object.NewString("Software errors and crashes"),
					"account": object.NewString("Login and account access"),
				}),
			}),
			"urgent": object.NewStringDict(map[string]object.Object{
				"type":         object.NewString("noul"),
				"instructions": object.NewString("Does this need immediate human attention?"),
			}),
		}),
	})

	result := decideMethod(instance, context.Background(), kwargs, model, "Our checkout page has returned 500 errors for every customer since 9am; sales are fully stopped.")
	if result.Type() == object.ERROR_OBJ {
		t.Fatalf("live decide failed: %s", result.Inspect())
	}

	label := dictField(t, dictField(t, result, "answers"), "label")
	choice := dictField(t, label, "choice").Inspect()
	if choice != "bug" && choice != "billing" && choice != "account" {
		t.Errorf("unexpected choice %q", choice)
	}
	if choice != "bug" {
		t.Logf("note: model chose %q (expected 'bug' for a 500-errors ticket; a model-quality signal, not an API failure)", choice)
	}
	urgent := dictField(t, dictField(t, result, "answers"), "urgent")
	if noul := dictField(t, urgent, "noul"); noul.Type() != object.FLOAT_OBJ && noul.Type() != object.INTEGER_OBJ {
		t.Errorf("noul = %v", noul)
	}
	t.Logf("live result: %s", result.Inspect())
}
