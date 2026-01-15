# AI Library Example - Creating Client Instance
# This script creates its own OpenAI client without needing Go configuration

import sl.ai as ai

print("Creating OpenAI client for LM Studio...")
# Create client directly from script (LM Studio doesn't require an API key)
client = ai.new_client("http://127.0.0.1:1234/v1")

print()
print("Fetching available models from LM Studio...")
models = client.models()
print(f"Found {len(models)} models:")
for model in models:
    print(f"  - {model.id}")

print()
print("Running chat completion with mistralai/ministral-3-3b...")

response = client.completion(
    "mistralai/ministral-3-3b",
    [{"role": "user", "content": "What is 2 + 2? Answer with just the number."}]
)

print()
print("Response:")
print(response.choices[0].message.content)
