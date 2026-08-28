#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "scriptling_plugin.h"

/* ------------------------------------------------------------------ */
/*  Functions                                                         */
/* ------------------------------------------------------------------ */

static sl_value *greet(int argc, sl_value **args, void *ctx) {
    (void)ctx;
    const char *name = (argc > 0) ? sl_as_string(args[0]) : "World";
    char buf[256];
    snprintf(buf, sizeof(buf), "Hello, %s", name);
    return sl_string(buf);
}

static sl_value *label(int argc, sl_value **args, void *ctx) {
    (void)ctx;
    const char *name = (argc > 0) ? sl_as_string(args[0]) : "unknown";
    char buf[256];
    snprintf(buf, sizeof(buf), "built:%s", name);
    return sl_string(buf);
}

static sl_value *stream(int argc, sl_value **args, void *ctx) {
    (void)ctx;
    if (argc == 0 || !args[0] || args[0]->type != SL_CALLBACK) {
        return sl_string("error: expected a callback argument");
    }

    const char *tokens[] = {"Hello", ", ", "Ada"};
    for (int i = 0; i < 3; i++) {
        sl_value *items[2] = { sl_string(tokens[i]), sl_int(i) };
        const char *keys[2] = { "token", "index" };
        sl_value *event = sl_dict(keys, items, 2);

        char *err = NULL;
        sl_value *r = sl_callback_call(args[0], 1, &event, &err);
        sl_value_free(event);
        if (err) {
            sl_value *err_v = sl_string(err);
            free(err);
            return err_v;
        }
        sl_value_free(r);
    }

    return sl_string("Hello, Ada");
}

static sl_value *work(int argc, sl_value **args, void *ctx) {
    (void)ctx;
    const char *name = (argc > 0) ? sl_as_string(args[0]) : "anonymous";
    sl_log_info("work started for %s", name);
    sl_log_debug("args received: %d", argc);
    char buf[256];
    snprintf(buf, sizeof(buf), "done:%s", name);
    return sl_string(buf);
}

/* ------------------------------------------------------------------ */
/*  Config class                                                      */
/* ------------------------------------------------------------------ */

typedef struct {
    char *name;
} config_data;

static void *config_ctor(int argc, sl_value **args, void *ctx) {
    (void)ctx;
    config_data *d = calloc(1, sizeof(*d));
    const char *name = (argc > 0) ? sl_as_string(args[0]) : "";
    d->name = strdup(name);
    return d;
}

static void config_dtor(void *data) {
    if (!data) return;
    config_data *d = data;
    free(d->name);
    free(d);
}

static sl_value *config_get(void *data, int argc, sl_value **args, void *ctx) {
    (void)argc; (void)args; (void)ctx;
    config_data *d = data;
    return sl_string(d->name);
}

/* ------------------------------------------------------------------ */
/*  Counter class (with properties)                                   */
/* ------------------------------------------------------------------ */

typedef struct {
    int64_t value;
} counter_data;

static void *counter_ctor(int argc, sl_value **args, void *ctx) {
    (void)ctx;
    counter_data *d = calloc(1, sizeof(*d));
    d->value = (argc > 0) ? sl_as_int(args[0]) : 0;
    return d;
}

static void counter_dtor(void *data) {
    free(data);
}

static sl_value *counter_inc(void *data, int argc, sl_value **args, void *ctx) {
    (void)ctx;
    counter_data *d = data;
    int64_t amount = (argc > 0) ? sl_as_int(args[0]) : 1;
    d->value += amount;
    return sl_int(d->value);
}

static sl_value *counter_get(void *data, int argc, sl_value **args, void *ctx) {
    (void)argc; (void)args; (void)ctx;
    counter_data *d = data;
    return sl_int(d->value);
}

static sl_value *counter_value_get(void *data, void *ctx) {
    (void)ctx;
    counter_data *d = data;
    return sl_int(d->value);
}

static void counter_value_set(void *data, sl_value *value, void *ctx) {
    (void)ctx;
    counter_data *d = data;
    d->value = sl_as_int(value);
}

static sl_value *counter_label_get(void *data, void *ctx) {
    (void)ctx;
    counter_data *d = data;
    char buf[64];
    snprintf(buf, sizeof(buf), "counter:%lld", (long long)d->value);
    return sl_string(buf);
}

/* ------------------------------------------------------------------ */
/*  Fetcher — serves cdemo:// sources from static content              */
/* ------------------------------------------------------------------ */

typedef struct {
    const char *path;
    const char *content;
} cdemo_file;

/* The virtual package served at cdemo://libs. */
static const cdemo_file cdemo_files[] = {
    { "lib/greet.py",       "def greeting(name):\n    return \"hello from cdemo://libs, \" + name\n" },
    { "lib/cdemo/__init__.py", "def prefix():\n    return \"cdemo\"\n" },
    { "docs/README.md",     "# cdemo://libs\n\nServed on demand by the C hello plugin.\n" },
};

/* The single-file script sources. */
static const char *cdemo_scripts[] = {
    "cdemo://scripts/hello",
};
static const char *cdemo_script_bodies[] = {
    "#!/usr/bin/env scriptling\nimport greet\nimport sys\nprint(greet.greeting(sys.argv[1] if len(sys.argv) > 1 else \"World\"))\n",
};

/* The host caches nothing it fetches, so this just returns the bytes. A fetcher
 * whose backend is slow enough to want caching does it inside this handler. */
static sl_fetch_result *cdemo_read(const char *source, const char *path, void *ctx) {
    (void)ctx;

    if (path[0] == '\0') {
        /* No path: the source itself is a single script file. */
        for (size_t i = 0; i < sizeof(cdemo_scripts) / sizeof(cdemo_scripts[0]); i++) {
            if (strcmp(source, cdemo_scripts[i]) == 0) {
                return sl_fetch_data(cdemo_script_bodies[i], strlen(cdemo_script_bodies[i]));
            }
        }
        return sl_fetch_not_found();
    }

    if (strncmp(source, "cdemo://libs", strlen("cdemo://libs")) != 0) {
        return sl_fetch_not_found();
    }
    for (size_t i = 0; i < sizeof(cdemo_files) / sizeof(cdemo_files[0]); i++) {
        if (strcmp(path, cdemo_files[i].path) == 0) {
            return sl_fetch_data(cdemo_files[i].content, strlen(cdemo_files[i].content));
        }
    }
    return sl_fetch_not_found();
}

static sl_fetch_entry *cdemo_list(const char *source, const char *path, size_t *count, void *ctx) {
    (void)ctx;
    if (strncmp(source, "cdemo://libs", strlen("cdemo://libs")) != 0) {
        *count = (size_t)-1;
        return NULL;
    }
    if (path[0] == '\0') path = ".";

    char prefix[256];
    if (strcmp(path, ".") == 0) prefix[0] = '\0';
    else snprintf(prefix, sizeof(prefix), "%s/", path);

    /* Names point at static storage, which outlives the handler call. */
    static char names[sizeof(cdemo_files) / sizeof(cdemo_files[0])][64];
    static bool is_dirs[sizeof(cdemo_files) / sizeof(cdemo_files[0])];
    static sl_fetch_entry entries[sizeof(cdemo_files) / sizeof(cdemo_files[0])];
    size_t n = 0;

    for (size_t i = 0; i < sizeof(cdemo_files) / sizeof(cdemo_files[0]); i++) {
        const char *name = cdemo_files[i].path;
        if (prefix[0] != '\0') {
            if (strncmp(name, prefix, strlen(prefix)) != 0) continue;
            name += strlen(prefix);
        }
        const char *slash = strchr(name, '/');
        if (slash) {
            /* Nested path: emit the directory component once. */
            size_t len = (size_t)(slash - name);
            bool seen = false;
            for (size_t j = 0; j < n; j++) {
                if (is_dirs[j] && strlen(names[j]) == len && strncmp(names[j], name, len) == 0) {
                    seen = true;
                    break;
                }
            }
            if (seen) continue;
            snprintf(names[n], sizeof(names[n]), "%.*s", (int)len, name);
            is_dirs[n] = true;
        } else {
            snprintf(names[n], sizeof(names[n]), "%s", name);
            is_dirs[n] = false;
        }
        entries[n].name = names[n];
        entries[n].is_dir = is_dirs[n];
        n++;
    }

    if (n == 0) { *count = (size_t)-1; return NULL; }

    /* The SDK frees the array, not the names — hand it a heap copy. */
    sl_fetch_entry *out = malloc(n * sizeof(*out));
    memcpy(out, entries, n * sizeof(*out));
    *count = n;
    return out;
}

/* ------------------------------------------------------------------ */
/*  Main — register everything and run                                */
/* ------------------------------------------------------------------ */

int main(void) {
    sl_server *srv = sl_server_new("hello", "1.0.0", "C hello plugin");

    sl_register_func_help(srv, "greet", greet, "greet(name) - Return a greeting string");
    sl_register_func(srv, "label", label);
    sl_register_func_help(srv, "stream", stream, "stream(callback) - Stream tokens to a callback function");
    sl_register_func_help(srv, "work", work, "work(name) - Log a message and return done");

    sl_class *cfg = sl_class_new("Config");
    sl_class_set_constructor(cfg, config_ctor);
    sl_class_set_destructor(cfg, config_dtor);
    sl_class_add_method(cfg, "get", config_get);
    sl_register_class(srv, cfg);

    sl_class *ctr = sl_class_new("Counter");
    sl_class_set_constructor(ctr, counter_ctor);
    sl_class_set_destructor(ctr, counter_dtor);
    sl_class_add_method(ctr, "inc", counter_inc);
    sl_class_add_method(ctr, "get", counter_get);
    sl_class_add_property(ctr, "value", counter_value_get, counter_value_set);
    sl_class_add_property(ctr, "label", counter_label_get, NULL);
    sl_register_class(srv, ctr);

    sl_constant(srv, "default_name", sl_string("World"));

    sl_register_fetcher(srv, "cdemo", cdemo_read, cdemo_list);

    int rc = sl_server_run(srv);
    sl_server_free(srv);
    return rc;
}
