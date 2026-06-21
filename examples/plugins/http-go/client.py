import scriptling.plugin


plugin_name = scriptling.plugin.load(
    "hello_http",
    "http://127.0.0.1:8081/json-rpc",
    scriptling=True,
)

import plugin.hello_http

print(scriptling.plugin.describe(plugin_name)["version"])
print(plugin.hello_http.greet("Ada"))

counter = plugin.hello_http.Counter(10)
print(counter.inc(5))

scriptling.plugin.unload(plugin_name)

try:
    import plugin.hello_http
except ImportError:
    print("unloaded")
