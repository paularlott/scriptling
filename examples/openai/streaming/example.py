# AI Library Example - Streaming Chat Completion
# This script demonstrates streaming responses from an OpenAI-compatible API

import scriptling.ai as ai

print("Creating OpenAI client for LM Studio...")
# Create client directly from script (LM Studio doesn't require an API key)
client = ai.new_client("http://127.0.0.1:1234/v1")

print()
print("Streaming chat completion with mistralai/ministral-3-3b...")
print("Response (streaming):")
print("-" * 60)

# Create a streaming completion
stream = client.completion_stream(
    "mistralai/ministral-3-3b",
    [{"role": "user", "content": "Write a short haiku about coding in Python. Be creative."}]
)

# Stream the response chunks
while True:
    chunk = stream.next()
    if chunk is None:
        break

    # Each chunk contains delta content
    if chunk.choices and len(chunk.choices) > 0:
        delta = chunk.choices[0].delta
        if delta and delta.content:
            print(delta.content, end='', flush=True)

print()
print("-" * 60)
print("Stream completed!")
