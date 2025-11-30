---
description: "Create a document with title, abstract, and body using basic authentication"
---

# Create a Document to Echo Service

This skill demonstrates how to use Scriptling to post a JSON document to the echo service at `https://httpbin.org/post`.

## Requirements

- Title: The document title
- Abstract: A brief summary
- Body: The main content

## Scriptling Code

```python
import json
import requests

# Document data
title = "Sample Document"
abstract = "This is a test document for demonstration"
body = "This is the body of the document. It contains the main content."

# Prepare JSON payload
data = {
    "title": title,
    "abstract": abstract,
    "body": body
}

# Convert to JSON string
json_data = json.dumps(data)

# Post to echo service with basic auth
response = requests.post(
    "https://httpbin.org/post",
    json_data,
    {
        "auth": ("testing", "testingpwd"),
        "headers": {"Content-Type": "application/json"}
    }
)

# Check response
if response.status_code == 200:
    print("Document posted successfully!")
    print("Response:", response.text)
else:
    print("Failed to post document. Status:", response.status_code)
    print("Response:", response.text)
```

## Validation

Always display the complete response from the echo service to verify the document was posted correctly:

1. Show the HTTP status code
2. Display the full response body
3. Verify that the returned JSON matches the sent data (title, abstract, and body fields)

The echo service will return the exact data that was posted, allowing you to confirm:
- Authentication was successful
- Headers were properly set
- JSON payload was correctly formatted and received
