import urllib.parse

components = {"scheme": "https", "netloc": "api.example.com", "path": "/v1/users", "query": "limit=10", "fragment": "section1"}
built_url = urllib.parse.urlunparse(components)
built_url == "https://api.example.com/v1/users?limit=10#section1"