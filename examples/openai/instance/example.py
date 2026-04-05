# AI Library Example - Creating Client Instance
# This script creates its own OpenAI client without needing Go configuration

import scriptling.ai as ai

print("Creating OpenAI client...")

client = ai.Client("http://127.0.0.1:11434/v1")

print()
print("Fetching available models...")
models_response = client.models()
models = models_response.data
print(f"Found {len(models)} models:")
for model in models:
    print(f"  - {model.id}")

print()
print("Running chat completion with gemma4:e4b...")

response = client.completion(
    "gemma4:e4b",
    [{"role": "user", "content": "What is 2 + 2? Answer with just the number."}]
)

print()
print("Response:")
print(response.choices[0].message.content)
