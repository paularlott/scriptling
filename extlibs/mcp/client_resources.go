package mcp

import (
	"context"
	"fmt"
	"strings"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/object"
)

// list_resources method implementation
func listResourcesMethod(self *object.Instance, ctx context.Context) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}
	if ci.client == nil {
		return &object.Error{Message: "list_resources: no client configured"}
	}

	resources, err := ci.client.ListResources(ctx)
	if err != nil {
		return &object.Error{Message: "failed to list resources: " + err.Error()}
	}
	return convertResourcesToList(resources)
}

// list_resource_templates method implementation
func listResourceTemplatesMethod(self *object.Instance, ctx context.Context) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}
	if ci.client == nil {
		return &object.Error{Message: "list_resource_templates: no client configured"}
	}

	templates, err := ci.client.ListResourceTemplates(ctx)
	if err != nil {
		return &object.Error{Message: "failed to list resource templates: " + err.Error()}
	}
	return convertResourceTemplatesToList(templates)
}

// read_resource method implementation
func readResourceMethod(self *object.Instance, ctx context.Context, uri string) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}
	if ci.client == nil {
		return &object.Error{Message: "read_resource: no client configured"}
	}

	resp, err := ci.client.ReadResource(ctx, uri)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return DecodeResourceResponse(resp)
}

// list_prompts method implementation
func listPromptsMethod(self *object.Instance, ctx context.Context) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}
	if ci.client == nil {
		return &object.Error{Message: "list_prompts: no client configured"}
	}

	prompts, err := ci.client.ListPrompts(ctx)
	if err != nil {
		return &object.Error{Message: "failed to list prompts: " + err.Error()}
	}
	return convertPromptsToList(prompts)
}

// get_prompt method implementation. Arguments are coerced to strings (MCP prompt
// arguments are always strings).
func getPromptMethod(self *object.Instance, ctx context.Context, name string, arguments map[string]any) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}
	if ci.client == nil {
		return &object.Error{Message: "get_prompt: no client configured"}
	}

	args := make(map[string]string, len(arguments))
	for k, v := range arguments {
		switch val := v.(type) {
		case string:
			args[k] = val
		default:
			args[k] = fmt.Sprintf("%v", val)
		}
	}

	resp, err := ci.client.GetPrompt(ctx, name, args)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return DecodePromptResponse(resp)
}

// convertResourcesToList converts []MCPResource to a scriptling List of dicts.
func convertResourcesToList(resources []mcplib.MCPResource) object.Object {
	elements := make([]object.Object, 0, len(resources))
	for _, r := range resources {
		d := object.NewStringDict(map[string]object.Object{
			"uri":         object.NewString(r.URI),
			"name":        object.NewString(r.Name),
			"description": object.NewString(r.Description),
			"mimeType":    object.NewString(r.MimeType),
		})
		elements = append(elements, d)
	}
	return &object.List{Elements: elements}
}

// convertResourceTemplatesToList converts []MCPResourceTemplate to a scriptling List.
func convertResourceTemplatesToList(templates []mcplib.MCPResourceTemplate) object.Object {
	elements := make([]object.Object, 0, len(templates))
	for _, t := range templates {
		d := object.NewStringDict(map[string]object.Object{
			"uriTemplate": object.NewString(t.URITemplate),
			"name":        object.NewString(t.Name),
			"description": object.NewString(t.Description),
			"mimeType":    object.NewString(t.MimeType),
		})
		elements = append(elements, d)
	}
	return &object.List{Elements: elements}
}

// convertPromptsToList converts []MCPPrompt to a scriptling List of dicts.
func convertPromptsToList(prompts []mcplib.MCPPrompt) object.Object {
	elements := make([]object.Object, 0, len(prompts))
	for _, p := range prompts {
		d := object.NewStringDict(map[string]object.Object{
			"name":        object.NewString(p.Name),
			"description": object.NewString(p.Description),
		})
		if len(p.Arguments) > 0 {
			argElems := make([]object.Object, 0, len(p.Arguments))
			for _, a := range p.Arguments {
				argElems = append(argElems, object.NewStringDict(map[string]object.Object{
					"name":        object.NewString(a.Name),
					"description": object.NewString(a.Description),
					"required":    object.NewBoolean(a.Required),
				}))
			}
			d.SetByString("arguments", &object.List{Elements: argElems})
		}
		elements = append(elements, d)
	}
	return &object.List{Elements: elements}
}

// DecodeResourceResponse converts an MCP ResourceResponse to a scriptling Object.
// A single content block returns that block's dict; multiple return a list.
func DecodeResourceResponse(response *mcplib.ResourceResponse) object.Object {
	if response == nil || len(response.Contents) == 0 {
		return &object.Null{}
	}
	if len(response.Contents) == 1 {
		return resourceContentToDict(response.Contents[0])
	}
	elements := make([]object.Object, len(response.Contents))
	for i, c := range response.Contents {
		elements[i] = resourceContentToDict(c)
	}
	return &object.List{Elements: elements}
}

func resourceContentToDict(c mcplib.ResourceContent) object.Object {
	d := object.NewStringDict(map[string]object.Object{
		"uri":      object.NewString(c.URI),
		"mimeType": object.NewString(c.MimeType),
	})
	if c.Text != "" {
		d.SetByString("text", decodeTextContent(c.Text))
	}
	if c.Blob != "" {
		d.SetByString("blob", object.NewString(c.Blob))
	}
	return d
}

// DecodePromptResponse converts an MCP PromptResponse to a scriptling Object.
func DecodePromptResponse(response *mcplib.PromptResponse) object.Object {
	if response == nil {
		return &object.Null{}
	}
	out := object.NewStringDict(map[string]object.Object{
		"description": object.NewString(response.Description),
	})
	msgs := make([]object.Object, 0, len(response.Messages))
	for _, m := range response.Messages {
		msgs = append(msgs, object.NewStringDict(map[string]object.Object{
			"role":    object.NewString(string(m.Role)),
			"content": DecodeToolContent(m.Content),
		}))
	}
	out.SetByString("messages", &object.List{Elements: msgs})
	return out
}

// completeMethod implements MCPClient.complete(ref, argument, value="",
// context=None): completion/complete for a prompt argument or a resource
// template variable.
func completeMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}
	if ci.client == nil {
		return &object.Error{Message: "complete: no client configured"}
	}
	get := func(i int, name string) object.Object {
		if i < len(args) {
			return args[i]
		}
		return kwargs.Get(name)
	}
	if len(args) > 4 {
		return &object.Error{Message: fmt.Sprintf("complete() takes at most 4 positional arguments, got %d", len(args))}
	}

	refObj := get(0, "ref")
	if refObj == nil {
		return &object.Error{Message: "complete: ref is required"}
	}
	var ref mcplib.CompletionRef
	switch r := refObj.(type) {
	case *object.String:
		s := r.StringValue()
		if strings.Contains(s, "://") {
			ref = mcplib.CompletionRef{Type: mcplib.CompletionRefResource, URI: s}
		} else {
			ref = mcplib.CompletionRef{Type: mcplib.CompletionRefPrompt, Name: s}
		}
	case *object.Dict:
		ref.Type, ref.Name, ref.URI = dictString(r, "type"), dictString(r, "name"), dictString(r, "uri")
	default:
		return &object.Error{Message: fmt.Sprintf("complete: ref must be a str or dict, got %s", refObj.Type())}
	}

	argObj := get(1, "argument")
	if argObj == nil {
		return &object.Error{Message: "complete: argument is required"}
	}
	argument, aerr := argObj.AsString()
	if aerr != nil {
		return &object.Error{Message: "complete: argument must be a string"}
	}
	value := ""
	if v := get(2, "value"); v != nil {
		s, verr := v.AsString()
		if verr != nil {
			return &object.Error{Message: "complete: value must be a string"}
		}
		value = s
	}
	var contextArgs map[string]string
	if c := get(3, "context"); c != nil {
		switch d := c.(type) {
		case *object.Null:
		case *object.Dict:
			contextArgs = make(map[string]string, len(d.Pairs))
			for _, pair := range d.Pairs {
				k, kerr := pair.Key.AsString()
				if kerr != nil {
					return &object.Error{Message: "complete: context keys must be strings"}
				}
				if s, ok := pair.Value.(*object.String); ok {
					contextArgs[k] = s.StringValue()
				} else {
					contextArgs[k] = pair.Value.Inspect()
				}
			}
		default:
			return &object.Error{Message: fmt.Sprintf("complete: context must be a dict, got %s", c.Type())}
		}
	}

	res, err := ci.client.Complete(ctx, ref, argument, value, contextArgs)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	values := make([]object.Object, len(res.Values))
	for i, v := range res.Values {
		values[i] = object.NewString(v)
	}
	var total object.Object = &object.Null{}
	if res.Total != nil {
		total = object.NewInteger(int64(*res.Total))
	}
	return object.NewStringDict(map[string]object.Object{
		"values":  &object.List{Elements: values},
		"total":   total,
		"hasMore": object.NewBoolean(res.HasMore),
	})
}

func dictString(d *object.Dict, key string) string {
	if pair, ok := d.GetByString(key); ok {
		if s, err := pair.Value.AsString(); err == nil {
			return s
		}
	}
	return ""
}
