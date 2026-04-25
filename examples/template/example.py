#!/usr/bin/env scriptling
"""Example demonstrating scriptling.template.html and scriptling.template.text"""

import scriptling.template.html as html
import scriptling.template.text as text
import os

# ── Text templates ────────────────────────────────────────────────────────────

print("=== Text Templates ===\n")

# Simple anonymous template
tmpl = text.Set()
tmpl.add("Hello, {{.Name}}! You have {{.Count}} new messages.")
print(tmpl.render({"Name": "Alice", "Count": 5}))

# Conditionals and loops
tmpl = text.Set()
tmpl.add("""Order #{{.OrderID}}
Status: {{if .Shipped}}Shipped{{else}}Pending{{end}}
{{- if .TrackingCode}}
Tracking: {{.TrackingCode}}
{{- end}}""")
print(tmpl.render({"OrderID": 1001, "Shipped": True, "TrackingCode": "TRK-9876"}))
print(tmpl.render({"OrderID": 1002, "Shipped": False, "TrackingCode": ""}))

# Loops
tmpl = text.Set()
tmpl.add("""Invoice for {{.Customer}}
{{range .Items}}- {{.Name}}: ${{.Price}}
{{end}}Total items: {{len .Items}}""")
print(tmpl.render({
    "Customer": "Bob",
    "Items": [
        {"Name": "Widget", "Price": 9.99},
        {"Name": "Gadget", "Price": 24.99},
        {"Name": "Doohickey", "Price": 4.99},
    ]
}))

# Partials with {{define}}
tmpl = text.Set()
tmpl.add('{{define "greeting"}}Hello, {{.Name}}!{{end}}')
tmpl.add('{{define "email"}}{{template "greeting" .}}' + "\n\n" + 'Your {{.Product}} trial expires in {{.ExpiryDays}} days.{{end}}')
print(tmpl.render("email", {"Name": "Charlie", "Product": "Scriptling Pro", "ExpiryDays": 14}))

# Template loaded from file
tmpl = text.Set()
tmpl.add(os.read_file("examples/template/email.txt"))
print(tmpl.render({"Name": "Dave", "Product": "Scriptling Pro", "ExpiryDays": 3}))

# ── HTML templates ────────────────────────────────────────────────────────────

print("\n=== HTML Templates ===\n")

# Simple anonymous template
tmpl = html.Set()
tmpl.add("""<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>
  <h1>{{.Title}}</h1>
  <p>Welcome, {{.User}}!</p>
  <ul>
  {{range .Items}}  <li>{{.}}</li>
  {{end}}</ul>
</body>
</html>""")
print(tmpl.render({"Title": "My Page", "User": "Alice", "Items": ["Apples", "Bananas", "Cherries"]}))

# XSS protection — user-supplied content is automatically escaped
tmpl = html.Set()
tmpl.add("<p>Comment: {{.Comment}}</p>")
print(tmpl.render({"Comment": "<script>alert('xss')</script>"}))

# Partials with {{define}}
tmpl = html.Set()
tmpl.add('{{define "header"}}<header><h1>{{.Title}}</h1></header>{{end}}')
tmpl.add('{{define "footer"}}<footer><p>© {{.Year}}</p></footer>{{end}}')
tmpl.add('{{define "page"}}<!DOCTYPE html><html><body>{{template "header" .}}<main>{{.Body}}</main>{{template "footer" .}}</body></html>{{end}}')
print(tmpl.render("page", {"Title": "Home", "Body": "Welcome!", "Year": 2026}))

# Template loaded from file
tmpl = html.Set()
tmpl.add(os.read_file("examples/template/article.html"))
print(tmpl.render({
    "Title": "Getting Started",
    "Author": "Eve",
    "Body": "Scriptling makes scripting in Go easy.",
    "Tags": ["go", "scripting", "templates"],
}))
