import url

components = {"scheme": "https", "host": "api.example.com", "path": "/v1/users", "query": "limit=10", "fragment": "section1"}
built_url = url.urlunparse(components)
built_url == "https://api.example.com/v1/users?limit=10#section1"