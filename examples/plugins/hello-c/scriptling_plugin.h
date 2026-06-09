#ifndef SCRIPTLING_PLUGIN_H
#define SCRIPTLING_PLUGIN_H

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ------------------------------------------------------------------ */
/*  Value types                                                       */
/* ------------------------------------------------------------------ */

typedef enum {
    SL_NULL,
    SL_BOOL,
    SL_INT,
    SL_FLOAT,
    SL_STRING,
    SL_LIST,
    SL_DICT,
    SL_CALLBACK
} sl_type;

typedef struct sl_value sl_value;

struct sl_value {
    sl_type type;

    bool        bool_val;
    int64_t     int_val;
    double      float_val;
    char       *str_val;

    sl_value  **list_items;
    size_t      list_count;

    char       **dict_keys;
    sl_value  **dict_vals;
    size_t      dict_count;

    char       *callback_id;
};

/* Value constructors — caller takes ownership. */
sl_value *sl_null(void);
sl_value *sl_bool(bool v);
sl_value *sl_int(int64_t v);
sl_value *sl_float(double v);
sl_value *sl_string(const char *v);
sl_value *sl_stringn(const char *v, size_t len);
sl_value *sl_list(sl_value **items, size_t count);
sl_value *sl_dict(const char **keys, sl_value **vals, size_t count);
sl_value *sl_callback(const char *id);

/* Deep free a value tree. */
void sl_value_free(sl_value *v);

/* Accessor helpers (return defaults on type mismatch). */
bool        sl_as_bool(const sl_value *v);
int64_t     sl_as_int(const sl_value *v);
double      sl_as_float(const sl_value *v);
const char *sl_as_string(const sl_value *v);
sl_value   *sl_list_get(const sl_value *v, size_t index);
sl_value   *sl_dict_get(const sl_value *v, const char *key);

/* ------------------------------------------------------------------ */
/*  Server                                                            */
/* ------------------------------------------------------------------ */

typedef struct sl_server sl_server;

typedef sl_value *(*sl_func_handler)(int argc, sl_value **args, void *ctx);

typedef void *(*sl_constructor_fn)(int argc, sl_value **args, void *ctx);
typedef void   (*sl_destructor_fn)(void *data);
typedef sl_value *(*sl_method_fn)(void *data, int argc, sl_value **args, void *ctx);
typedef sl_value *(*sl_prop_getter_fn)(void *data, void *ctx);
typedef void      (*sl_prop_setter_fn)(void *data, sl_value *value, void *ctx);

sl_server *sl_server_new(const char *name, const char *version, const char *description);
void       sl_server_free(sl_server *srv);

/* Set a user context pointer passed to all handlers. */
void sl_server_set_context(sl_server *srv, void *ctx);

/* Register a plain function. */
void sl_register_func(sl_server *srv, const char *name, sl_func_handler handler);

/* Register a function with help text shown by help(). */
void sl_register_func_help(sl_server *srv, const char *name,
                           sl_func_handler handler, const char *help_text);

/* Register a function with a custom Scriptling wrapper source. */
void sl_register_script_func(sl_server *srv, const char *name, const char *source);

/* Register a class. Returns a class handle for adding methods/properties. */
typedef struct sl_class sl_class;

sl_class *sl_class_new(const char *name);
void      sl_class_free(sl_class *c);

void sl_class_set_constructor(sl_class *c, sl_constructor_fn fn);
void sl_class_set_destructor(sl_class *c, sl_destructor_fn fn);
void sl_class_add_method(sl_class *c, const char *name, sl_method_fn fn);
void sl_class_add_property(sl_class *c, const char *name,
                           sl_prop_getter_fn getter, sl_prop_setter_fn setter);

void sl_register_class(sl_server *srv, sl_class *c);

/* Register a class with custom Scriptling wrapper source. */
void sl_register_script_class(sl_server *srv, const char *name, const char *source);

/* Register a constant. */
void sl_constant(sl_server *srv, const char *name, sl_value *value);

/* Set a custom Scriptling wrapper source for a previously registered
 * function or class. */
void sl_wrapper(sl_server *srv, const char *name, const char *source);

/* Run the JSON-RPC event loop on stdin/stdout. Returns 0 on clean shutdown. */
int sl_server_run(sl_server *srv);

/* Call a callback value received in arguments. Returns the result or NULL
 * on error. If non-NULL, *err_msg is set to an allocated error string on
 * failure (caller must free). */
sl_value *sl_callback_call(const sl_value *cb, int argc, sl_value **args,
                           char **err_msg);

/* ------------------------------------------------------------------ */
/*  Logging                                                           */
/* ------------------------------------------------------------------ */

void sl_log_trace(const char *msg, ...);
void sl_log_debug(const char *msg, ...);
void sl_log_info(const char *msg, ...);
void sl_log_warn(const char *msg, ...);
void sl_log_error(const char *msg, ...);

#ifdef __cplusplus
}
#endif

#endif /* SCRIPTLING_PLUGIN_H */
