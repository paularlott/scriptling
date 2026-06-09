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
/*  Main — register everything and run                                */
/* ------------------------------------------------------------------ */

int main(void) {
    sl_server *srv = sl_server_new("hello", "1.0.0", "C hello plugin");

    sl_register_func_help(srv, "greet", greet, "greet(name) - Return a greeting string");
    sl_register_func(srv, "label", label);
    sl_register_func_help(srv, "stream", stream, "stream(callback) - Stream tokens to a callback function");

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

    return sl_server_run(srv);
}
