import scriptling.mcp.tool as tool

code = tool.get_string("code")
language = tool.get_string("language", "the given")

tool.return_object({
    "messages": [
        {
            "role": "user",
            "content": "Review this " + language + " code and list any issues:\n\n" + code,
        }
    ]
})
