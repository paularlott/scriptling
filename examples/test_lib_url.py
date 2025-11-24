# Test url library

import url

print("=== URL Library Test ===")
print("")

# Test 1: URL encoding
print("1. URL encoding")
text = "hello world"
encoded = url.encode(text)
print("   Original:", text)
print("   Encoded:", encoded)
print("")

# Test 2: URL decoding
print("2. URL decoding")
decoded = url.decode(encoded)
print("   Decoded:", decoded)
print("   Match:", decoded == text)
print("")

# Test 3: Encode special characters
print("3. Encode special characters")
special = "name=John Doe&age=30"
enc_special = url.encode(special)
print("   Original:", special)
print("   Encoded:", enc_special)
print("")

# Test 4: Parse URL
print("4. Parse URL")
test_url = "https://example.com:8080/path/to/page?key=value&foo=bar#section"
parsed = url.parse(test_url)
print("   URL:", test_url)
print("   Scheme:", parsed["scheme"])
print("   Host:", parsed["host"])
print("   Path:", parsed["path"])
print("   Query:", parsed["query"])
print("   Fragment:", parsed["fragment"])
print("")

# Test 5: Build URL
print("5. Build URL")
parts = {"scheme": "https", "host": "api.example.com", "path": "/v1/users", "query": "limit=10&offset=0"}
built = url.build(parts)
print("   Built URL:", built)
print("")

# Test 6: Join URLs
print("6. Join URLs")
base = "https://example.com/api/"
ref = "users/123"
joined = url.join(base, ref)
print("   Base:", base)
print("   Reference:", ref)
print("   Joined:", joined)
print("")

# Test 7: Parse query string
print("7. Parse query string")
query_str = "name=Alice&age=30&city=NYC"
query_dict = url.query_parse(query_str)
print("   Query string:", query_str)
print("   Parsed name:", query_dict["name"])
print("   Parsed age:", query_dict["age"])
print("   Parsed city:", query_dict["city"])
print("")

# Test 8: Roundtrip encode/decode
print("8. Roundtrip encode/decode")
original = "Hello, World! 123 @#$%"
enc = url.encode(original)
dec = url.decode(enc)
print("   Original:", original)
print("   Roundtrip match:", dec == original)
print("")

# Test 9: Parse simple URL
print("9. Parse simple URL")
simple = "http://localhost:3000/api"
parsed_simple = url.parse(simple)
print("   URL:", simple)
print("   Scheme:", parsed_simple["scheme"])
print("   Host:", parsed_simple["host"])
print("   Path:", parsed_simple["path"])
print("")

# Test 10: Practical example - build API URL
print("10. Practical example - build API URL")
def build_api_url(endpoint, params):
    # Build query string
    query_parts = []
    for item in items(params):
        key = item[0]
        value = item[1]
        encoded_value = url.encode(value)
        query_part = key + "=" + encoded_value
        append(query_parts, query_part)
    query_string = join(query_parts, "&")
    
    # Build URL
    url_parts = {"scheme": "https", "host": "api.example.com", "path": endpoint, "query": query_string}
    return url.build(url_parts)

params = {"search": "hello world", "limit": "10"}
api_url = build_api_url("/v1/search", params)
print("   API URL:", api_url)
print("")

print("=== All URL Tests Complete ===")
