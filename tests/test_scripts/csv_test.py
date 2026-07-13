import scriptling.csv as csv

# parse
content = "name,age,city\nAlice,30,NYC\nBob,25,LA\n"
rows = csv.loads(content)
assert len(rows) == 3, f"expected 3 rows, got {len(rows)}"
assert rows[0] == ["name", "age", "city"], f"header: {rows[0]}"
assert rows[1] == ["Alice", "30", "NYC"], f"row1: {rows[1]}"

# parse with embedded comma (quoted)
content2 = 'a,b\n"hello, world",x\n'
rows2 = csv.loads(content2)
assert rows2[1][0] == "hello, world", f"quoted: {rows2[1]}"

# parse_dict
people = csv.loads_dict(content)
assert len(people) == 2, f"expected 2 dicts, got {len(people)}"
assert people[0]["name"] == "Alice", f"name: {people[0]['name']}"
assert people[1]["city"] == "LA", f"city: {people[1]['city']}"

# format
text = csv.dumps([["a", "b"], ["1", "2"]])
assert text == "a,b\n1,2\n", f"format: {text!r}"

# format with embedded comma (auto-quoted)
text2 = csv.dumps([["hello, world", "x"]])
assert text2 == '"hello, world",x\n', f"format quoted: {text2!r}"

# format_dict
text3 = csv.dumps_dict([{"name": "Alice", "age": "30"}])
# headers are sorted: age, name
lines = text3.strip().split("\n")
assert lines[0] == "age,name", f"header: {lines[0]}"
assert lines[1] == "30,Alice", f"row: {lines[1]}"

# round trip
original = "name,age\nAlice,30\nBob,25\n"
roundtrip = csv.dumps_dict(csv.loads_dict(original))
assert "Alice" in roundtrip and "30" in roundtrip, f"roundtrip: {roundtrip}"

# delimiter
rows_tab = csv.loads("a\tb\n1\t2\n", delimiter="\t")
assert rows_tab[1] == ["1", "2"], f"tab: {rows_tab[1]}"

True
