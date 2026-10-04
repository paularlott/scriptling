package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	mcpai "github.com/paularlott/mcp/ai"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

// decideMethod implements client.decide(model, state, questions=, images=,
// keep_alive=): a System One decision request (Ollama /v1/systemone), where
// a decision model classifies the state, estimates yes/no probabilities or
// scores it against an ordered rubric — one non-streaming response with
// per-question probabilities and confidence. Only providers whose client
// implements ai.DecisionCaller support this (Ollama).
func decideMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, model string, state any) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}
	if ci.client == nil {
		return &object.Error{Message: "decide: no client configured"}
	}

	dc, ok := ci.client.(mcpai.DecisionCaller)
	if !ok {
		return &object.Error{Message: fmt.Sprintf("decide: decision models are not supported by provider '%s'", ci.client.Provider())}
	}

	questionsObj := kwargs.Get("questions")
	if questionsObj == nil || questionsObj.Type() == object.NULL_OBJ {
		return &object.Error{Message: "decide: questions is required (a dict of named questions)"}
	}
	questionsMap, errObj := questionsObj.AsDict()
	if errObj != nil {
		return &object.Error{Message: "decide: questions must be a dict of named questions"}
	}
	if len(questionsMap) == 0 || len(questionsMap) > 64 {
		return &object.Error{Message: fmt.Sprintf("decide: questions must hold 1 to 64 entries, got %d", len(questionsMap))}
	}

	req := mcpai.SystemOneRequest{Model: model, State: state}
	req.Questions = make(map[string]mcpai.SystemOneQuestion, len(questionsMap))
	for name, qObj := range questionsMap {
		q, err := systemOneQuestion(name, qObj)
		if err != nil {
			return err
		}
		req.Questions[name] = q
	}

	if imagesObj := kwargs.Get("images"); imagesObj != nil && imagesObj.Type() != object.NULL_OBJ {
		imagesList, errObj := imagesObj.AsList()
		if errObj != nil {
			return &object.Error{Message: "decide: images must be a list of base64 strings or bytes"}
		}
		req.Images = make([]string, 0, len(imagesList))
		for i, img := range imagesList {
			switch v := img.(type) {
			case *object.String:
				req.Images = append(req.Images, v.StringValue())
			case *object.Bytes:
				req.Images = append(req.Images, base64.StdEncoding.EncodeToString(v.BytesValue()))
			default:
				return &object.Error{Message: fmt.Sprintf("decide: images[%d] must be a base64 string or bytes", i)}
			}
		}
	}

	if ka := kwargs.Get("keep_alive"); ka != nil && ka.Type() != object.NULL_OBJ {
		req.KeepAlive = conversion.ToGo(ka)
	}

	var resp *mcpai.SystemOneResponse
	var err error
	object.RunBlocking(ctx, func() { resp, err = dc.Decide(ctx, req) })
	if err != nil {
		return &object.Error{Message: "decide: " + err.Error()}
	}
	return systemOneResponseToObject(resp)
}

// systemOneResponseToObject converts the response with exact number types
// (token counts as integers, probabilities as floats). conversion.FromGo's
// struct path round-trips JSON without UseNumber, which would turn every
// integer into a float; decoding with json.Number here keeps 174 as 174.
func systemOneResponseToObject(resp *mcpai.SystemOneResponse) object.Object {
	data, err := json.Marshal(resp)
	if err != nil {
		return &object.Error{Message: "decide: " + err.Error()}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return &object.Error{Message: "decide: " + err.Error()}
	}
	return conversion.FromGo(generic)
}

// systemOneQuestion validates one named question and converts it to its
// wire shape. The three question types have different criteria forms:
// choice — a dict of 2–26 option descriptions; noul — optional "false"/
// "true" descriptions; score — an ordered list of 2–26 descriptions.
func systemOneQuestion(name string, qObj object.Object) (mcpai.SystemOneQuestion, object.Object) {
	qMap, errObj := qObj.AsDict()
	if errObj != nil {
		return mcpai.SystemOneQuestion{}, &object.Error{Message: fmt.Sprintf("decide: questions[%s] must be a dict", name)}
	}

	var q mcpai.SystemOneQuestion
	typeObj, ok := qMap["type"]
	if !ok {
		return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s] is missing \"type\"", name)}
	}
	qType, err := typeObj.AsString()
	if err != nil {
		return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].type must be a string", name)}
	}
	q.Type = qType

	instrObj, ok := qMap["instructions"]
	if !ok {
		return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s] is missing \"instructions\"", name)}
	}
	instructions, err := instrObj.AsString()
	if err != nil {
		return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].instructions must be a string", name)}
	}
	q.Instructions = instructions

	criteriaObj, hasCriteria := qMap["criteria"]
	switch qType {
	case "choice":
		if !hasCriteria {
			return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s] (choice) requires criteria", name)}
		}
		criteriaMap, errObj := criteriaObj.AsDict()
		if errObj != nil {
			return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].criteria must be a dict of option descriptions", name)}
		}
		if len(criteriaMap) < 2 || len(criteriaMap) > 26 {
			return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].criteria must have 2 to 26 options, got %d", name, len(criteriaMap))}
		}
	case "noul":
		if hasCriteria {
			criteriaMap, errObj := criteriaObj.AsDict()
			if errObj != nil {
				return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].criteria (noul) must be a dict with \"false\" and \"true\" descriptions", name)}
			}
			for key := range criteriaMap {
				if key != "false" && key != "true" {
					return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].criteria (noul) may only have \"false\" and \"true\" keys, got %q", name, key)}
				}
			}
		}
	case "score":
		if !hasCriteria {
			return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s] (score) requires criteria", name)}
		}
		levels, errObj := criteriaObj.AsList()
		if errObj != nil {
			return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].criteria (score) must be an ordered list of descriptions, lowest to highest", name)}
		}
		if len(levels) < 2 || len(levels) > 26 {
			return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].criteria (score) must have 2 to 26 levels, got %d", name, len(levels))}
		}
	default:
		return q, &object.Error{Message: fmt.Sprintf("decide: questions[%s].type must be \"choice\", \"noul\" or \"score\", got %q", name, qType)}
	}

	if hasCriteria {
		q.Criteria = conversion.ToGo(criteriaObj)
	}
	return q, nil
}
