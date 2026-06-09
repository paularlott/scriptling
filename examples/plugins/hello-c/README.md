# Scriptling C Plugin SDK

A multi-threaded C library for building Scriptling plugins. Add
`scriptling_plugin.h` and `scriptling_plugin.c` to your project, write
a `main()`, and compile a standalone executable that Scriptling loads via
`--plugin-dir`.

Each incoming request is handled in its own thread, matching the
concurrency model of the Go plugin server. The object store, stdout writes,
and callback routing are all thread-safe.

## Quick Start

```c
// hello.c
#include "scriptling_plugin.h"

static sl_value *greet(int argc, sl_value **args, void *ctx) {
    const char *name = (argc > 0) ? sl_as_string(args[0]) : "World";
    char buf[256];
    snprintf(buf, sizeof(buf), "Hello, %s", name);
    return sl_string(buf);
}

int main(void) {
    sl_server *srv = sl_server_new("hello", "1.0.0", "My first plugin");
    sl_register_func(srv, "greet", greet);
    return sl_server_run(srv);
}
```

```bash
gcc -std=c11 -O2 -o hello hello.c scriptling_plugin.c -lm
mkdir -p /tmp/plugins && cp hello /tmp/plugins/
scriptling --plugin-dir /tmp/plugins -c 'import plugin.hello; print(plugin.hello.greet("Ada"))'
```

## Features

- **Functions** — `sl_register_func()`
- **Classes** — constructors, methods, destructors via `sl_class_new()` / `sl_register_class()`
- **Properties** — read-only and read/write via `sl_class_add_property()`
- **Constants** — `sl_constant()`
- **Callbacks** — call back into Scriptling from within handlers via `sl_callback_call()`
- **Logging** — route logs through the host logger via `sl_log_info()` etc.
- **Custom wrappers** — `sl_register_script_func()`, `sl_register_script_class()`, `sl_wrapper()`
- Multi-threaded — each request runs in its own thread
- Thread-safe object store with per-object locking
- No external dependencies beyond the C standard library and pthreads

## Reference

### Values

Scriptling values are represented as `sl_value` tagged unions. Use the
constructor functions to create them — ownership transfers to the caller
(or the SDK, depending on context):

| Constructor                   | Scriptling type |
| ----------------------------- | --------------- |
| `sl_null()`                   | `null`          |
| `sl_bool(bool)`               | `bool`          |
| `sl_int(int64_t)`             | `int`           |
| `sl_float(double)`            | `float`         |
| `sl_string(const char *)`     | `string`        |
| `sl_list(items, count)`       | `list`          |
| `sl_dict(keys, vals, count)`  | `dict`          |
| `sl_callback(const char *id)` | callback handle |

Accessors coerce between types:

```c
sl_as_bool(v)       // → bool (also converts int)
sl_as_int(v)        // → int64_t (also converts float, bool)
sl_as_float(v)      // → double (also converts int)
sl_as_string(v)     // → const char* (returns "" on mismatch)
sl_list_get(v, idx) // → sl_value* (NULL on out of range)
sl_dict_get(v, key) // → sl_value* (NULL if not found)
```

Free values with `sl_value_free(v)`.

### Server Lifecycle

```c
sl_server *srv = sl_server_new(name, version, description);
// ... register functions, classes, constants ...
int rc = sl_server_run(srv);     // blocks until shutdown
sl_server_free(srv);
```

`sl_server_set_context(srv, ptr)` stores a user pointer that is passed as the
last argument to every handler.

### Functions

```c
sl_value *handler(int argc, sl_value **args, void *ctx) {
    // return an sl_value, or NULL for null
}

sl_register_func(srv, "name", handler);
```

### Classes

```c
// Constructor — returns a heap-allocated pointer that becomes the instance data.
// Free it in the destructor.
void *my_ctor(int argc, sl_value **args, void *ctx);
void  my_dtor(void *data);

// Method — receives the instance data pointer.
sl_value *my_method(void *data, int argc, sl_value **args, void *ctx);

// Property getter/setter
sl_value *my_prop_get(void *data, void *ctx);
void      my_prop_set(void *data, sl_value *value, void *ctx);

sl_class *cls = sl_class_new("MyClass");
sl_class_set_constructor(cls, my_ctor);
sl_class_set_destructor(cls, my_dtor);
sl_class_add_method(cls, "do_thing", my_method);
sl_class_add_property(cls, "name", my_prop_get, NULL);           // read-only
sl_class_add_property(cls, "value", my_prop_get, my_prop_set);   // read/write
sl_register_class(srv, cls);
```

Properties are accessed as attributes from Scriptling:

```python
c = plugin.mylib.MyClass(10)
c.value = c.value + 5   # calls setter then getter
print(c.name)            # calls getter
```

### Constants

```c
sl_constant(srv, "pi", sl_float(3.14159));
sl_constant(srv, "default_name", sl_string("World"));
```

### Callbacks

When Scriptling passes a function as an argument, the SDK represents it as an
`sl_value` with `type == SL_CALLBACK`. Invoke it with `sl_callback_call()`:

```c
static sl_value *my_handler(int argc, sl_value **args, void *ctx) {
    if (argc < 1 || args[0]->type != SL_CALLBACK)
        return sl_string("expected a callback");

    for (int i = 0; i < 3; i++) {
        sl_value *event = sl_int(i);
        char *err = NULL;
        sl_value *result = sl_callback_call(args[0], 1, &event, &err);
        sl_value_free(event);
        if (err) {
            sl_value *err_v = sl_string(err);
            free(err);
            return err_v;
        }
        sl_value_free(result);
    }
    return sl_string("done");
}
```

Call from Scriptling:

```python
import plugin.hello
events = []
result = plugin.hello.stream(lambda e: events.append(e))
print(result)   # "Hello, Ada"
print(events)   # [{token: Hello, index: 0}, {token: , , index: 1}, {token: Ada, index: 2}]
```

### Logging

```c
sl_log_info("processing item %d", item_id);
sl_log_warn("low memory: %zu bytes", remaining);
sl_log_error("failed: %s", errmsg);
```

Logs are forwarded through the host's logger. Levels: `sl_log_trace`,
`sl_log_debug`, `sl_log_info`, `sl_log_warn`, `sl_log_error`.

### Custom Scriptling Wrappers

Replace the auto-generated proxy with custom Scriptling source:

```c
sl_register_func(srv, "greet", greet_handler);
sl_wrapper(srv, "greet",
    "import scriptling.plugin\n"
    "def greet(name):\n"
    "    return scriptling.plugin.call_function(\"plugin.mylib\", \"greet\", name) + \"!\"\n"
);

sl_register_script_class(srv, "Config",
    "import scriptling.plugin\n"
    "class Config:\n"
    "    def __init__(self, name):\n"
    "        self._plugin_remote = scriptling.plugin._new_object(\"plugin.mylib\", \"Config\", name)\n"
    "    def get(self, key, default=\"\"):\n"
    "        return scriptling.plugin.call_method(self._plugin_remote, \"get\", key) or default\n"
    "    def __del__(self):\n"
    "        scriptling.plugin.release(self._plugin_remote)\n"
);
```

## File Layout

```
your-plugin/
  scriptling_plugin.h   — public header
  scriptling_plugin.c   — implementation
  main.c                — your plugin
  Makefile
```

## Compilation

```bash
gcc -std=c11 -Wall -Wextra -O2 -o myplugin main.c scriptling_plugin.c -lm -lpthread
```

Requires a C11 compiler and pthreads. No other external dependencies.
