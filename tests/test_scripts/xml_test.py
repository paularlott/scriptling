import scriptling.xml as xml

# Basic parse
data = xml.loads("<root><name>Alice</name><age>30</age></root>")
assert "root" in data, f"missing root key: {data}"
root = data["root"]
assert root["name"] == "Alice", f"name: {root['name']}"
assert root["age"] == "30", f"age: {root['age']}"

# Attributes → @-prefixed keys
data = xml.loads('<user id="123" active="true"><name>Alice</name></user>')
user = data["user"]
assert user["@id"] == "123", f"@id: {user['@id']}"
assert user["@active"] == "true", f"@active: {user['@active']}"
assert user["name"] == "Alice", f"name: {user['name']}"

# Text + attributes → #text key
data = xml.loads('<msg type="greeting">Hello World</msg>')
msg = data["msg"]
assert msg["@type"] == "greeting", f"@type: {msg['@type']}"
assert msg["#text"] == "Hello World", f"#text: {msg['#text']}"

# Repeated elements → list
data = xml.loads("<items><item>a</item><item>b</item><item>c</item></items>")
items = data["items"]["item"]
assert len(items) == 3, f"expected 3 items, got {len(items)}"
assert items[0] == "a", f"item0: {items[0]}"
assert items[2] == "c", f"item2: {items[2]}"

# Empty element → empty string
data = xml.loads("<root><empty></empty></root>")
assert data["root"]["empty"] == "", f"empty: {data['root']['empty']}"

# Nested elements
data = xml.loads("<config><server><host>localhost</host><port>8080</port></server></config>")
server = data["config"]["server"]
assert server["host"] == "localhost", f"host: {server['host']}"
assert server["port"] == "8080", f"port: {server['port']}"

# Dumps: basic
text = xml.dumps({"root": {"name": "Alice", "age": "30"}})
assert "<root>" in text, f"missing root open: {text}"
assert "<name>Alice</name>" in text, f"missing name: {text}"
assert "<age>30</age>" in text, f"missing age: {text}"
assert "</root>" in text, f"missing root close: {text}"

# Dumps: with attributes
text = xml.dumps({"user": {"@id": "123", "name": "Alice"}})
assert '<user id="123">' in text, f"missing attr: {text}"
assert "<name>Alice</name>" in text, f"missing name: {text}"

# Dumps: repeated elements from list
text = xml.dumps({"items": {"item": ["a", "b"]}})
assert text.count("<item>") == 2, f"expected 2 items: {text}"
assert ">a<" in text and ">b<" in text, f"missing values: {text}"

# Dumps: text + attributes
text = xml.dumps({"msg": {"@type": "greeting", "#text": "Hello"}})
assert '<msg type="greeting">' in text, f"missing attr: {text}"
assert "Hello</msg>" in text, f"missing text: {text}"

# Round trip
original = '<root><name>Alice</name><age>30</age></root>'
data = xml.loads(original)
roundtrip = xml.dumps(data)
# Re-parse to verify structure is preserved
data2 = xml.loads(roundtrip)
assert data2["root"]["name"] == "Alice", f"roundtrip name: {data2}"
assert data2["root"]["age"] == "30", f"roundtrip age: {data2}"

# XML escaping in dumps
text = xml.dumps({"root": "a < b & c > d"})
assert "&lt;" in text or "&gt;" in text or "&amp;" in text, f"escaping: {text}"

# Indented output
text = xml.dumps({"root": {"child": "value"}}, indent="  ")
assert "\n" in text, f"indent should add newlines: {text}"

True
