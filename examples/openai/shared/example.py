# AI Library Example - Using Wrapped Client
# The client was wrapped in Go and passed as the ai_client global variable

print("Using the AI client from the wrapped global variable...")
print()

print("Fetching available models from LM Studio...")
models = ai_client.models()
print(f"Found {len(models)} models:")
for model in models:
    print(f"  - {model.id}")

print()
print("Running chat completion with mistralai/ministral-3-3b...")

response = ai_client.chat(
    "mistralai/ministral-3-3b",
    {"role": "user", "content": "What is 2 + 2? Answer with just the number."}
)

print()
print("Response:")
print(response.choices[0].message.content)
