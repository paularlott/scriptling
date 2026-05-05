# Test ai.estimate_tokens function

import scriptling.ai as ai

# Test 1: String input returns a dict with expected keys and correct types
result = ai.estimate_tokens("Hello!")
assert "prompt_tokens" in result, "Missing prompt_tokens key"
assert "completion_tokens" in result, "Missing completion_tokens key"
assert "total_tokens" in result, "Missing total_tokens key"
assert result["prompt_tokens"] > 0, "prompt_tokens should be > 0 for non-empty string"
assert result["completion_tokens"] == 0, "completion_tokens should be 0 without response"
assert result["total_tokens"] == result["prompt_tokens"] + result["completion_tokens"]

# Test 2: List of messages
messages = [
    {"role": "user", "content": "Hello!"},
    {"role": "assistant", "content": "Hi there!"},
    {"role": "user", "content": "How are you?"}
]
result = ai.estimate_tokens(messages)
assert result["prompt_tokens"] > 0
assert result["completion_tokens"] == 0
assert result["total_tokens"] == result["prompt_tokens"]

# Test 3: Request dict with messages key should match direct messages list
request = {"messages": messages}
result_dict = ai.estimate_tokens(request)
assert result_dict["prompt_tokens"] == result["prompt_tokens"]

# Test 4: With response (completion tokens from response content)
response = {
    "choices": [
        {
            "message": {
                "role": "assistant",
                "content": "I am doing well, thank you!"
            }
        }
    ]
}
result = ai.estimate_tokens(messages, response)
assert result["prompt_tokens"] > 0
assert result["completion_tokens"] > 0, "completion_tokens should be > 0 with response"
assert result["total_tokens"] == result["prompt_tokens"] + result["completion_tokens"]

# Test 5: Empty string has fewer tokens than non-empty string
result_empty = ai.estimate_tokens("")
result_hello = ai.estimate_tokens("Hello!")
assert result_empty["prompt_tokens"] < result_hello["prompt_tokens"]

# Test 6: Longer text produces more tokens than shorter text
short = ai.estimate_tokens("Hi")
long = ai.estimate_tokens("This is a much longer piece of text that should produce more tokens")
assert long["prompt_tokens"] > short["prompt_tokens"]

# Test 7: More messages produce more tokens than fewer messages
one_msg = ai.estimate_tokens([{"role": "user", "content": "Hello"}])
three_msgs = ai.estimate_tokens([
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi"},
    {"role": "user", "content": "How are you?"}
])
assert three_msgs["prompt_tokens"] > one_msg["prompt_tokens"]

# Test 8: None request with response only
result = ai.estimate_tokens(None, response)
assert result["prompt_tokens"] == 0
assert result["completion_tokens"] > 0
assert result["total_tokens"] == result["completion_tokens"]

# Test 9: Without response, only prompt tokens
result = ai.estimate_tokens("Just a prompt")
assert result["completion_tokens"] == 0
assert result["prompt_tokens"] > 0
assert result["total_tokens"] == result["prompt_tokens"]

# Test 10: Error handling - wrong number of arguments should fail
error_caught = False
try:
    ai.estimate_tokens()
    error_caught = False
except:
    error_caught = True
assert error_caught, "estimate_tokens() with no args should raise an error"

error_caught = False
try:
    ai.estimate_tokens("a", "b", "c")
    error_caught = False
except:
    error_caught = True
assert error_caught, "estimate_tokens() with 3 args should raise an error"

# Test 11: Same input produces same output (deterministic)
r1 = ai.estimate_tokens("Deterministic test")
r2 = ai.estimate_tokens("Deterministic test")
assert r1["prompt_tokens"] == r2["prompt_tokens"]
assert r1["total_tokens"] == r2["total_tokens"]

# Test 12: Token counts scale with text length
short_result = ai.estimate_tokens("A")
medium_result = ai.estimate_tokens("A" * 40)
long_result = ai.estimate_tokens("A" * 200)
assert short_result["prompt_tokens"] < medium_result["prompt_tokens"]
assert medium_result["prompt_tokens"] < long_result["prompt_tokens"]

print("All ai.estimate_tokens tests passed!")
True
