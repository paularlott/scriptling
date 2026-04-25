# Template Example

Demonstrates `scriptling.template.html` and `scriptling.template.text` using the `Set` API.

## What It Shows

- Simple anonymous templates
- Variable substitution with `{{.Field}}`
- Conditionals with `{{if}}` / `{{else}}` / `{{end}}`
- Loops with `{{range}}`
- Partials with `{{define "name"}}` and `{{template "name" .}}`
- Loading templates from files with `os.read_file()`
- XSS protection via automatic HTML escaping in `html.Set()`

## Files

| File | Purpose |
|------|---------|
| `example.py` | Main example script |
| `email.txt` | Text template for an email |
| `article.html` | HTML template for an article page |

## Running

```bash
./bin/scriptling examples/template/example.py
```

## API Summary

```python
import scriptling.template.html as html
import scriptling.template.text as text

# Simple template
tmpl = html.Set()
tmpl.add("<h1>Hello, {{.Name}}!</h1>")
print(tmpl.render({"Name": "Alice"}))

# With partials
tmpl = html.Set()
tmpl.add('{{define "header"}}<h1>{{.Title}}</h1>{{end}}')
tmpl.add('{{define "page"}}{{template "header" .}}<p>{{.Body}}</p>{{end}}')
print(tmpl.render("page", {"Title": "Home", "Body": "Welcome"}))
```

## Go Template Syntax Quick Reference

| Syntax | Description |
|--------|-------------|
| `{{.Field}}` | Output a field from the data dict |
| `{{if .Cond}} ... {{end}}` | Conditional block |
| `{{range .List}} {{.}} {{end}}` | Loop over a list |
| `{{define "name"}} ... {{end}}` | Define a named partial |
| `{{template "name" .}}` | Include a partial |
| `{{- ... -}}` | Trim surrounding whitespace |

See the [Go template docs](https://pkg.go.dev/text/template) for the full syntax.
