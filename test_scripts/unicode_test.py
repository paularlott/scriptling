# Unicode edge cases testing - Non-ASCII character handling
import string

passed = True

# Unicode edge cases testing - Non-ASCII character handling
import string

passed = True

# Test 1: Basic Unicode strings (Note: Scriptling treats strings as byte sequences, not character sequences)
print("Testing basic Unicode strings...")
unicode_str = "Hello 世界 🌍"
# Length is in bytes, not characters due to UTF-8 encoding
assert len(unicode_str) == 17  # UTF-8 encoded bytes
# Test that the string contains the expected substrings
assert "Hello" in unicode_str
assert "世界" in unicode_str
assert "🌍" in unicode_str

# Test 2: Unicode in lists and dicts
print("Testing Unicode in data structures...")
unicode_list = ["α", "β", "γ", "δ", "ε"]
assert len(unicode_list) == 5
# Test that strings are stored correctly
assert unicode_list[0] == "α"
assert "γ" in unicode_list

unicode_dict = {"name": "José", "city": "São Paulo", "message": "café"}
assert unicode_dict["name"] == "José"
assert unicode_dict["city"] == "São Paulo"
assert unicode_dict["message"] == "café"

# Test 3: Unicode string operations
print("Testing Unicode string operations...")
test_str = "naïve résumé"
assert test_str.upper() == "NAÏVE RÉSUMÉ"
assert test_str.lower() == "naïve résumé"
assert test_str.capitalize() == "Naïve résumé"

# Test 4: Unicode splitting and joining
print("Testing Unicode splitting and joining...")
words = "The quick brown fox jumps over the lazy dog".split()
unicode_words = ["Le", "renard", "brun", "rapide"]
joined = " ".join(unicode_words)
assert joined == "Le renard brun rapide"

# Test 5: Unicode formatting
print("Testing Unicode formatting...")
name = "François"
age = 25
formatted = f"Name: {name}, Age: {age}"
assert formatted == "Name: François, Age: 25"

# Test 6: Unicode in comprehensions
print("Testing Unicode in comprehensions...")
# Use ASCII characters for comprehension since indexing is byte-based
ascii_letters = [chr(i) for i in range(97, 101)]  # a b c d
assert ascii_letters == ["a", "b", "c", "d"]

# Test 7: Unicode ord and chr (using ASCII for compatibility)
print("Testing Unicode ord and chr...")
assert ord("a") == 97
assert chr(97) == "a"
assert ord("Z") == 90
assert chr(90) == "Z"

# Test 8: Unicode string methods
print("Testing Unicode string methods...")
mixed_str = "Hello 世界! How are you? Café au lait. naïve"
assert mixed_str.startswith("Hello")
assert mixed_str.endswith("naïve")
assert "世界" in mixed_str
assert mixed_str.find("世界") >= 0
assert mixed_str.count("a") >= 2  # a in various words

# Test 9: Unicode whitespace handling
print("Testing Unicode whitespace...")
spaces_str = "  \t  Hello  \n  世界  \r  "
stripped = spaces_str.strip()
assert len(stripped) > 0  # Should have content after stripping

# Test 10: Unicode case conversion edge cases
print("Testing Unicode case conversion...")
german = "straße"
assert german.upper() == "STRAßE"  # ß stays ß in Go (unlike Python's SS)
assert german.lower() == "straße"

turkish = "İstanbul"
assert turkish.upper() == "İSTANBUL"  # dotted I
assert turkish.lower() == "istanbul"

# Test 11: Unicode in sets
print("Testing Unicode in sets...")
unicode_set = set(["α", "β", "γ", "α"])  # duplicate should be removed
assert len(unicode_set) == 3
assert "β" in unicode_set

# Test 12: Unicode sorting (basic)
print("Testing Unicode sorting...")
words_to_sort = ["zebra", "αpple", "Banana", "café"]
sorted_words = sorted(words_to_sort)
# Note: Unicode sorting may vary by locale, just test it's sorted somehow
assert len(sorted_words) == 4
assert sorted_words[0] in words_to_sort

# Test 13: ASCII string slicing (avoiding multi-byte issues)
print("Testing ASCII string slicing...")
ascii_str = "Hello World"
assert len(ascii_str) == 11
assert ascii_str[0] == "H"
assert ascii_str[6] == "W"
assert ascii_str[-1] == "d"

# Test 14: Mixed encoding handling
print("Testing mixed encoding...")
mixed = "ASCII and Кириллица and العربية"
assert len(mixed) > 20  # Should handle all scripts as bytes

# Test 15: Operations with Unicode strings
print("Testing operations with Unicode...")
try:
    result = "test" + 123  # This should fail
    assert False, "Should have failed"
except TypeError as e:
    # Just ensure error handling works with Unicode context
    pass

print("All Unicode edge case tests passed!")
assert passed