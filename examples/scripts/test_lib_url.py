# Test: URL library

import lib

print("=== Testing URL Library ===")

# URL encode/decode
print("\n--- URL Encoding/Decoding ---")
text = "hello world"
encoded = lib.quote(text)
print("URL encode '" + text + "': " + encoded)

decoded = lib.unquote(encoded)
print("URL decode '" + encoded + "': " + decoded)

# Test with special characters
special = "name=John Doe&age=30"
enc = lib.quote(special)
dec = lib.unquote(enc)
print("Special chars: '" + special + "' -> '" + enc + "' -> '" + dec + "'")

# Test various strings
test_cases = ["test@example.com", "hello+world", "a/b/c"]
for text in test_cases:
    enc = lib.quote(text)
    dec = lib.unquote(enc)
    print("'" + text + "' -> '" + enc + "' -> '" + dec + "'")

# URL parsing
print("\n--- URL Parsing ---")
url_str = "https://user:pass@example.com:8080/path/to/resource?query=value&other=test#fragment"
parsed = lib.urlparse(url_str)
print("Parsed URL: " + url_str)
print("  scheme: " + parsed["scheme"])
print("  host: " + parsed["host"])
print("  path: " + parsed["path"])
print("  query: " + parsed["query"])
print("  fragment: " + parsed["fragment"])

# URL building
print("\n--- URL Building ---")
components = {
    "scheme": "https",
    "host": "api.example.com",
    "path": "/v1/users",
    "query": "limit=10&offset=0",
    "fragment": "section1"
}
built_url = lib.urlunparse(components)
print("Built URL from components: " + built_url)

# Query parsing
print("\n--- Query Parsing ---")
query_str = "name=Alice&age=30&city=New%20York&tags=python&tags=web"
query_dict = lib.parse_qs(query_str)
print("Query string: " + query_str)
print("Parsed query: " + str(query_dict))

# URL joining
print("\n--- URL Joining ---")
base = "https://api.example.com/v1"
relative = "/users/123"
joined = lib.urljoin(base, relative)
print("Joined '" + base + "' + '" + relative + "' = '" + joined + "'")

# urlsplit
print("\n--- URL Splitting ---")
split_parts = lib.urlsplit("https://example.com/path?query=value#fragment")
print("urlsplit result: " + str(split_parts))
print("  scheme: " + split_parts[0])
print("  netloc: " + split_parts[1])
print("  path: " + split_parts[2])
print("  query: " + split_parts[3])
print("  fragment: " + split_parts[4])

# urlunsplit
print("\n--- URL Unsplitting ---")
parts = ["https", "example.com", "/api/data", "format=json&limit=100", "results"]
unsplit_url = lib.urlunsplit(parts)
print("urlunsplit " + str(parts) + " = '" + unsplit_url + "'")

# parse_qs
print("\n--- Query String Parsing (parse_qs) ---")
qs_str = "name=Alice&name=Bob&age=30&tags=python&tags=web"
qs_dict = lib.parse_qs(qs_str)
print("parse_qs '" + qs_str + "':")
print("  name: " + str(qs_dict["name"]))
print("  age: " + str(qs_dict["age"]))
print("  tags: " + str(qs_dict["tags"]))

# urlencode
print("\n--- URL Encoding (urlencode) ---")
data1 = {"name": "Alice", "age": "30", "city": "New York"}
encoded1 = lib.urlencode(data1)
print("urlencode " + str(data1) + " = '" + encoded1 + "'")

data2 = {"tags": ["python", "web", "api"], "active": "true"}
encoded2 = lib.urlencode(data2)
print("urlencode " + str(data2) + " = '" + encoded2 + "'")

print("\n✓ All URL library tests passed")

# URL building
print("\n--- URL Building ---")
components = {
    "scheme": "https",
    "host": "api.example.com",
    "path": "/v1/users",
    "query": "limit=10&offset=0",
    "fragment": "section1"
}
built_url = lib.urlunparse(components)
print("Built URL from components: " + built_url)

# Query parsing
print("\n--- Query Parsing ---")
query_str = "name=Alice&age=30&city=New%20York&tags=python&tags=web"
query_dict = lib.parse_qs(query_str)
print("Query string: " + query_str)
print("Parsed query: " + str(query_dict))

# URL joining
print("\n--- URL Joining ---")
base = "https://api.example.com/v1"
relative = "/users/123"
joined = lib.urljoin(base, relative)
print("Joined '" + base + "' + '" + relative + "' = '" + joined + "'")

# urlsplit
print("\n--- URL Splitting ---")
split_parts = lib.urlsplit("https://example.com/path?query=value#fragment")
print("urlsplit result: " + str(split_parts))
print("  scheme: " + split_parts[0])
print("  netloc: " + split_parts[1])
print("  path: " + split_parts[2])
print("  query: " + split_parts[3])
print("  fragment: " + split_parts[4])

# urlunsplit
print("\n--- URL Unsplitting ---")
parts = ["https", "example.com", "/api/data", "format=json&limit=100", "results"]
unsplit_url = lib.urlunsplit(parts)
print("urlunsplit " + str(parts) + " = '" + unsplit_url + "'")

# parse_qs
print("\n--- Query String Parsing (parse_qs) ---")
qs_str = "name=Alice&name=Bob&age=30&tags=python&tags=web"
qs_dict = lib.parse_qs(qs_str)
print("parse_qs '" + qs_str + "':")
print("  name: " + str(qs_dict["name"]))
print("  age: " + str(qs_dict["age"]))
print("  tags: " + str(qs_dict["tags"]))

# urlencode
print("\n--- URL Encoding (urlencode) ---")
data1 = {"name": "Alice", "age": "30", "city": "New York"}
encoded1 = lib.urlencode(data1)
print("urlencode " + str(data1) + " = '" + encoded1 + "'")

data2 = {"tags": ["python", "web", "api"], "active": "true"}
encoded2 = lib.urlencode(data2)
print("urlencode " + str(data2) + " = '" + encoded2 + "'")

print("\n✓ All URL library tests passed")
