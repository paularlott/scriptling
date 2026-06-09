#include "scriptling_plugin.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdarg.h>
#include <math.h>

/* ================================================================== */
/*  Thread-local server pointer (for callbacks and logging)           */
/* ================================================================== */

#ifdef _WIN32
static __declspec(thread) sl_server *tl_server = NULL;
#else
static __thread sl_server *tl_server = NULL;
#endif

/* ================================================================== */
/*  Internal string buffer                                            */
/* ================================================================== */

typedef struct {
    char  *b;
    size_t n;
    size_t c;
} sbuf;

static void sb_init(sbuf *s) {
    s->c = 256;
    s->n = 0;
    s->b = malloc(s->c);
    s->b[0] = '\0';
}

static void sb_free(sbuf *s) {
    free(s->b);
    s->b = NULL;
    s->n = 0;
    s->c = 0;
}

static void sb_grow(sbuf *s, size_t need) {
    if (s->n + need + 1 >= s->c) {
        while (s->n + need + 1 >= s->c) s->c = s->c ? s->c * 2 : 256;
        s->b = realloc(s->b, s->c);
    }
}

static void sb_append(sbuf *s, const char *p, size_t n) {
    if (n == 0) return;
    sb_grow(s, n);
    memcpy(s->b + s->n, p, n);
    s->n += n;
    s->b[s->n] = '\0';
}

static void sb_puts(sbuf *s, const char *p) { sb_append(s, p, strlen(p)); }

static void sb_putc(sbuf *s, char c) { sb_append(s, &c, 1); }

static void sb_printf(sbuf *s, const char *fmt, ...) {
    va_list a;
    va_start(a, fmt);
    char tmp[1024];
    int n = vsnprintf(tmp, sizeof(tmp), fmt, a);
    va_end(a);
    if (n > 0) sb_append(s, tmp, (size_t)n);
}

static void sb_json_str(sbuf *s, const char *p, size_t len) {
    sb_putc(s, '"');
    for (size_t i = 0; i < len; i++) {
        unsigned char c = (unsigned char)p[i];
        switch (c) {
            case '"':  sb_puts(s, "\\\""); break;
            case '\\': sb_puts(s, "\\\\"); break;
            case '\b': sb_puts(s, "\\b");  break;
            case '\f': sb_puts(s, "\\f");  break;
            case '\n': sb_puts(s, "\\n");  break;
            case '\r': sb_puts(s, "\\r");  break;
            case '\t': sb_puts(s, "\\t");  break;
            default:
                if (c < 0x20) {
                    char hex[8];
                    snprintf(hex, sizeof(hex), "\\u%04x", c);
                    sb_puts(s, hex);
                } else {
                    sb_putc(s, (char)c);
                }
                break;
        }
    }
    sb_putc(s, '"');
}

/* ================================================================== */
/*  Minimal JSON parser                                               */
/* ================================================================== */

typedef enum { JT_NULL, JT_BOOL, JT_NUM, JT_STR, JT_ARR, JT_OBJ } jtype;

typedef struct jval jval;

struct jval {
    jtype t;
    union {
        int    bv;
        double nv;
        struct { char *b; size_t n; } sv;
        struct { jval **a; size_t n, c; } arr;
        struct { char **k; jval **v; size_t n, c; } obj;
    } u;
};

static jval *jnew(jtype t) { jval *v = calloc(1, sizeof(*v)); v->t = t; return v; }

static void jfree(jval *v) {
    if (!v) return;
    switch (v->t) {
        case JT_STR: free(v->u.sv.b); break;
        case JT_ARR:
            for (size_t i = 0; i < v->u.arr.n; i++) jfree(v->u.arr.a[i]);
            free(v->u.arr.a);
            break;
        case JT_OBJ:
            for (size_t i = 0; i < v->u.obj.n; i++) {
                free(v->u.obj.k[i]);
                jfree(v->u.obj.v[i]);
            }
            free(v->u.obj.k);
            free(v->u.obj.v);
            break;
        default: break;
    }
    free(v);
}

typedef struct { const char *s; size_t p; } jp;

static void jws(jp *p) {
    while (p->s[p->p]==' '||p->s[p->p]=='\t'||p->s[p->p]=='\n'||p->s[p->p]=='\r') p->p++;
}

static int jp_val(jp *p, jval **out);

static int jp_str(jp *p, sbuf *out) {
    if (p->s[p->p] != '"') return -1;
    p->p++;
    while (p->s[p->p]) {
        char c = p->s[p->p];
        if (c == '\\') {
            p->p++;
            switch (p->s[p->p]) {
                case '"':  sb_putc(out, '"');  break;
                case '\\': sb_putc(out, '\\'); break;
                case '/':  sb_putc(out, '/');  break;
                case 'b':  sb_putc(out, '\b'); break;
                case 'f':  sb_putc(out, '\f'); break;
                case 'n':  sb_putc(out, '\n'); break;
                case 'r':  sb_putc(out, '\r'); break;
                case 't':  sb_putc(out, '\t'); break;
                case 'u': {
                    char hex[5] = {0};
                    if (p->s[p->p+1]) memcpy(hex, p->s + p->p + 1, 4);
                    unsigned cp = 0;
                    for (int i = 0; i < 4; i++) {
                        cp <<= 4;
                        if (hex[i]>='0'&&hex[i]<='9')      cp |= (unsigned)(hex[i]-'0');
                        else if (hex[i]>='a'&&hex[i]<='f') cp |= (unsigned)(hex[i]-'a'+10);
                        else if (hex[i]>='A'&&hex[i]<='F') cp |= (unsigned)(hex[i]-'A'+10);
                    }
                    if (cp < 0x80) {
                        sb_putc(out, (char)cp);
                    } else if (cp < 0x800) {
                        char u[2];
                        u[0] = (char)(0xC0 | (cp >> 6));
                        u[1] = (char)(0x80 | (cp & 0x3F));
                        sb_append(out, u, 2);
                    } else {
                        char u[3];
                        u[0] = (char)(0xE0 | (cp >> 12));
                        u[1] = (char)(0x80 | ((cp >> 6) & 0x3F));
                        u[2] = (char)(0x80 | (cp & 0x3F));
                        sb_append(out, u, 3);
                    }
                    p->p += 4;
                    break;
                }
                default: return -1;
            }
        } else if (c == '"') { p->p++; return 0; }
        else sb_putc(out, c);
        p->p++;
    }
    return -1;
}

static int jp_num(jp *p, jval **out) {
    size_t start = p->p;
    if (p->s[p->p] == '-') p->p++;
    if (p->s[p->p] == '0') p->p++;
    else if (p->s[p->p]>='1'&&p->s[p->p]<='9') while (p->s[p->p]>='0'&&p->s[p->p]<='9') p->p++;
    else return -1;
    if (p->s[p->p] == '.') { p->p++; while (p->s[p->p]>='0'&&p->s[p->p]<='9') p->p++; }
    if (p->s[p->p]=='e'||p->s[p->p]=='E') { p->p++; if (p->s[p->p]=='+'||p->s[p->p]=='-') p->p++; while (p->s[p->p]>='0'&&p->s[p->p]<='9') p->p++; }
    jval *v = jnew(JT_NUM); v->u.nv = strtod(p->s + start, NULL); *out = v; return 0;
}

static int jp_arr(jp *p, jval **out) {
    p->p++; jval *v = jnew(JT_ARR); jws(p);
    if (p->s[p->p] == ']') { p->p++; *out = v; return 0; }
    for (;;) {
        if (v->u.arr.n >= v->u.arr.c) { v->u.arr.c = v->u.arr.c ? v->u.arr.c * 2 : 8; v->u.arr.a = realloc(v->u.arr.a, v->u.arr.c * sizeof(*v->u.arr.a)); }
        if (jp_val(p, &v->u.arr.a[v->u.arr.n++]) != 0) { jfree(v); return -1; }
        jws(p);
        if (p->s[p->p] == ',') { p->p++; jws(p); continue; }
        if (p->s[p->p] == ']') { p->p++; *out = v; return 0; }
        jfree(v); return -1;
    }
}

static int jp_obj(jp *p, jval **out) {
    p->p++; jval *v = jnew(JT_OBJ); jws(p);
    if (p->s[p->p] == '}') { p->p++; *out = v; return 0; }
    for (;;) {
        jws(p); sbuf ks; sb_init(&ks);
        if (jp_str(p, &ks) != 0) { sb_free(&ks); jfree(v); return -1; }
        jws(p);
        if (p->s[p->p] != ':') { sb_free(&ks); jfree(v); return -1; }
        p->p++;
        jval *vv = NULL;
        if (jp_val(p, &vv) != 0) { sb_free(&ks); jfree(v); return -1; }
        if (v->u.obj.n >= v->u.obj.c) {
            v->u.obj.c = v->u.obj.c ? v->u.obj.c * 2 : 8;
            v->u.obj.k = realloc(v->u.obj.k, v->u.obj.c * sizeof(*v->u.obj.k));
            v->u.obj.v = realloc(v->u.obj.v, v->u.obj.c * sizeof(*v->u.obj.v));
        }
        v->u.obj.k[v->u.obj.n] = ks.b;
        v->u.obj.v[v->u.obj.n] = vv;
        v->u.obj.n++;
        jws(p);
        if (p->s[p->p] == ',') { p->p++; continue; }
        if (p->s[p->p] == '}') { p->p++; *out = v; return 0; }
        jfree(v); return -1;
    }
}

static int jp_val(jp *p, jval **out) {
    jws(p); char c = p->s[p->p];
    if (c == '"') {
        sbuf s; sb_init(&s);
        if (jp_str(p, &s) != 0) { sb_free(&s); return -1; }
        jval *v = jnew(JT_STR); v->u.sv.b = s.b; v->u.sv.n = s.n; *out = v; return 0;
    }
    if (c == '{') return jp_obj(p, out);
    if (c == '[') return jp_arr(p, out);
    if (c == 't') { if (strncmp(p->s+p->p,"true",4)!=0) return -1; p->p+=4; jval *v=jnew(JT_BOOL); v->u.bv=1; *out=v; return 0; }
    if (c == 'f') { if (strncmp(p->s+p->p,"false",5)!=0) return -1; p->p+=5; jval *v=jnew(JT_BOOL); v->u.bv=0; *out=v; return 0; }
    if (c == 'n') { if (strncmp(p->s+p->p,"null",4)!=0) return -1; p->p+=4; *out=jnew(JT_NULL); return 0; }
    if (c == '-' || (c >= '0' && c <= '9')) return jp_num(p, out);
    return -1;
}

static jval *json_parse(const char *src) {
    jp p = {src, 0};
    jval *v = NULL;
    return (jp_val(&p, &v) == 0) ? v : NULL;
}

static const jval *jget(const jval *obj, const char *key) {
    if (!obj || obj->t != JT_OBJ) return NULL;
    for (size_t i = 0; i < obj->u.obj.n; i++)
        if (strcmp(obj->u.obj.k[i], key) == 0) return obj->u.obj.v[i];
    return NULL;
}

static const char *jget_str(const jval *obj, const char *key) {
    const jval *v = jget(obj, key);
    return (v && v->t == JT_STR) ? v->u.sv.b : NULL;
}

static int64_t jget_int(const jval *obj, const char *key, int64_t def) {
    const jval *v = jget(obj, key);
    return (v && v->t == JT_NUM) ? (int64_t)v->u.nv : def;
}

/* ================================================================== */
/*  JSON writer                                                       */
/* ================================================================== */

static void json_write_val(sbuf *s, const jval *v) {
    if (!v) { sb_puts(s, "null"); return; }
    switch (v->t) {
        case JT_NULL: sb_puts(s, "null"); break;
        case JT_BOOL: sb_puts(s, v->u.bv ? "true" : "false"); break;
        case JT_NUM: {
            double n = v->u.nv;
            if (!isinf(n) && !isnan(n) && n == (double)(long long)n)
                sb_printf(s, "%lld", (long long)n);
            else sb_printf(s, "%.17g", n);
            break;
        }
        case JT_STR: sb_json_str(s, v->u.sv.b, v->u.sv.n); break;
        case JT_ARR:
            sb_putc(s, '[');
            for (size_t i = 0; i < v->u.arr.n; i++) {
                if (i) sb_putc(s, ',');
                json_write_val(s, v->u.arr.a[i]);
            }
            sb_putc(s, ']');
            break;
        case JT_OBJ:
            sb_putc(s, '{');
            for (size_t i = 0; i < v->u.obj.n; i++) {
                if (i) sb_putc(s, ',');
                sb_json_str(s, v->u.obj.k[i], strlen(v->u.obj.k[i]));
                sb_putc(s, ':');
                json_write_val(s, v->u.obj.v[i]);
            }
            sb_putc(s, '}');
            break;
    }
}

/* ================================================================== */
/*  Transport value → JSON                                            */
/* ================================================================== */

static void val_to_json(sbuf *s, const sl_value *v) {
    if (!v || v->type == SL_NULL) { sb_puts(s, "{\"type\":\"null\"}"); return; }
    switch (v->type) {
        case SL_BOOL:
            sb_puts(s, v->bool_val ? "{\"type\":\"bool\",\"value\":true}"
                                   : "{\"type\":\"bool\",\"value\":false}");
            break;
        case SL_INT:
            sb_printf(s, "{\"type\":\"int\",\"value\":%lld}", (long long)v->int_val);
            break;
        case SL_FLOAT:
            sb_printf(s, "{\"type\":\"float\",\"value\":%.17g}", v->float_val);
            break;
        case SL_STRING:
            sb_puts(s, "{\"type\":\"string\",\"value\":");
            sb_json_str(s, v->str_val, strlen(v->str_val));
            sb_putc(s, '}');
            break;
        case SL_LIST:
            sb_puts(s, "{\"type\":\"list\",\"items\":[");
            for (size_t i = 0; i < v->list_count; i++) {
                if (i) sb_putc(s, ',');
                val_to_json(s, v->list_items[i]);
            }
            sb_puts(s, "]}");
            break;
        case SL_DICT:
            sb_puts(s, "{\"type\":\"dict\",\"entries\":{");
            for (size_t i = 0; i < v->dict_count; i++) {
                if (i) sb_putc(s, ',');
                sb_json_str(s, v->dict_keys[i], strlen(v->dict_keys[i]));
                sb_putc(s, ':');
                val_to_json(s, v->dict_vals[i]);
            }
            sb_puts(s, "}}");
            break;
        case SL_CALLBACK:
            sb_puts(s, "{\"type\":\"callback\",\"callback\":{\"id\":");
            sb_json_str(s, v->callback_id, strlen(v->callback_id));
            sb_puts(s, "}}");
            break;
        case SL_NULL:
            sb_puts(s, "{\"type\":\"null\"}");
            break;
    }
}

/* ================================================================== */
/*  JSON → Transport value                                            */
/* ================================================================== */

static sl_value *parse_transport_val(const jval *jv) {
    if (!jv || jv->t != JT_OBJ) return sl_null();
    const char *type = jget_str(jv, "type");
    if (!type) return sl_null();

    if (strcmp(type, "null") == 0) return sl_null();
    if (strcmp(type, "bool") == 0) {
        const jval *vv = jget(jv, "value");
        bool b = false;
        if (vv) {
            if (vv->t == JT_BOOL) b = (bool)vv->u.bv;
            else if (vv->t == JT_NUM) b = vv->u.nv != 0;
        }
        return sl_bool(b);
    }
    if (strcmp(type, "int") == 0) {
        const jval *vv = jget(jv, "value");
        return sl_int(vv && vv->t == JT_NUM ? (int64_t)vv->u.nv : 0);
    }
    if (strcmp(type, "float") == 0) {
        const jval *vv = jget(jv, "value");
        return sl_float(vv && vv->t == JT_NUM ? vv->u.nv : 0.0);
    }
    if (strcmp(type, "string") == 0) {
        const char *sv = jget_str(jv, "value");
        return sl_string(sv ? sv : "");
    }
    if (strcmp(type, "list") == 0) {
        const jval *items = jget(jv, "items");
        if (!items || items->t != JT_ARR) return sl_list(NULL, 0);
        size_t n = items->u.arr.n;
        sl_value **vals = n ? malloc(n * sizeof(*vals)) : NULL;
        for (size_t i = 0; i < n; i++) vals[i] = parse_transport_val(items->u.arr.a[i]);
        sl_value *r = sl_list(vals, n);
        free(vals);
        return r;
    }
    if (strcmp(type, "dict") == 0) {
        const jval *entries = jget(jv, "entries");
        if (!entries || entries->t != JT_OBJ) return sl_dict(NULL, NULL, 0);
        size_t n = entries->u.obj.n;
        const char **keys = n ? malloc(n * sizeof(*keys)) : NULL;
        sl_value **vals = n ? malloc(n * sizeof(*vals)) : NULL;
        for (size_t i = 0; i < n; i++) {
            keys[i] = entries->u.obj.k[i];
            vals[i] = parse_transport_val(entries->u.obj.v[i]);
        }
        sl_value *r = sl_dict(keys, vals, n);
        free(keys);
        free(vals);
        return r;
    }
    if (strcmp(type, "callback") == 0) {
        const jval *cb = jget(jv, "callback");
        const char *id = cb ? jget_str(cb, "id") : NULL;
        return sl_callback(id ? id : "");
    }
    return sl_null();
}

/* ================================================================== */
/*  sl_value implementation                                           */
/* ================================================================== */

sl_value *sl_null(void) {
    sl_value *v = calloc(1, sizeof(*v)); v->type = SL_NULL; return v;
}

sl_value *sl_bool(bool b) {
    sl_value *v = calloc(1, sizeof(*v)); v->type = SL_BOOL; v->bool_val = b; return v;
}

sl_value *sl_int(int64_t i) {
    sl_value *v = calloc(1, sizeof(*v)); v->type = SL_INT; v->int_val = i; return v;
}

sl_value *sl_float(double f) {
    sl_value *v = calloc(1, sizeof(*v)); v->type = SL_FLOAT; v->float_val = f; return v;
}

sl_value *sl_string(const char *s) {
    return sl_stringn(s, s ? strlen(s) : 0);
}

sl_value *sl_stringn(const char *s, size_t len) {
    sl_value *v = calloc(1, sizeof(*v)); v->type = SL_STRING;
    v->str_val = malloc(len + 1);
    if (s && len) memcpy(v->str_val, s, len);
    v->str_val[len] = '\0';
    return v;
}

sl_value *sl_list(sl_value **items, size_t count) {
    sl_value *v = calloc(1, sizeof(*v)); v->type = SL_LIST; v->list_count = count;
    if (count) { v->list_items = malloc(count * sizeof(*v->list_items)); memcpy(v->list_items, items, count * sizeof(*v->list_items)); }
    return v;
}

sl_value *sl_dict(const char **keys, sl_value **vals, size_t count) {
    sl_value *v = calloc(1, sizeof(*v)); v->type = SL_DICT; v->dict_count = count;
    if (count) {
        v->dict_keys = malloc(count * sizeof(*v->dict_keys));
        v->dict_vals = malloc(count * sizeof(*v->dict_vals));
        for (size_t i = 0; i < count; i++) {
            v->dict_keys[i] = strdup(keys[i]);
            v->dict_vals[i] = vals[i];
        }
    }
    return v;
}

sl_value *sl_callback(const char *id) {
    sl_value *v = calloc(1, sizeof(*v)); v->type = SL_CALLBACK;
    v->callback_id = strdup(id ? id : "");
    return v;
}

void sl_value_free(sl_value *v) {
    if (!v) return;
    switch (v->type) {
        case SL_STRING:  free(v->str_val); break;
        case SL_LIST:
            for (size_t i = 0; i < v->list_count; i++) sl_value_free(v->list_items[i]);
            free(v->list_items);
            break;
        case SL_DICT:
            for (size_t i = 0; i < v->dict_count; i++) { free(v->dict_keys[i]); sl_value_free(v->dict_vals[i]); }
            free(v->dict_keys); free(v->dict_vals);
            break;
        case SL_CALLBACK: free(v->callback_id); break;
        default: break;
    }
    free(v);
}

bool        sl_as_bool(const sl_value *v)   { return v ? (v->type == SL_BOOL ? v->bool_val : v->type == SL_INT ? v->int_val != 0 : false) : false; }
int64_t     sl_as_int(const sl_value *v)    { return v ? (v->type == SL_INT ? v->int_val : v->type == SL_FLOAT ? (int64_t)v->float_val : v->type == SL_BOOL ? (v->bool_val?1:0) : 0) : 0; }
double      sl_as_float(const sl_value *v)  { return v ? (v->type == SL_FLOAT ? v->float_val : v->type == SL_INT ? (double)v->int_val : 0.0) : 0.0; }
const char *sl_as_string(const sl_value *v) { return (v && v->type == SL_STRING) ? v->str_val : ""; }

sl_value *sl_list_get(const sl_value *v, size_t idx) {
    return (v && v->type == SL_LIST && idx < v->list_count) ? v->list_items[idx] : NULL;
}

sl_value *sl_dict_get(const sl_value *v, const char *key) {
    if (!v || v->type != SL_DICT) return NULL;
    for (size_t i = 0; i < v->dict_count; i++)
        if (strcmp(v->dict_keys[i], key) == 0) return v->dict_vals[i];
    return NULL;
}

/* ================================================================== */
/*  Internal server types                                             */
/* ================================================================== */

typedef struct {
    char             *name;
    sl_func_handler   handler;
    char             *source;
    char             *description;
} sl_func_entry;

struct sl_class {
    char              *name;
    sl_constructor_fn  ctor;
    sl_destructor_fn   dtor;
    char              *source;
    struct { char **names; sl_method_fn *fns; size_t count, cap; } methods;
    struct { char **names; sl_prop_getter_fn *getters; sl_prop_setter_fn *setters; size_t count, cap; } props;
};

typedef struct {
    void     *data;
    sl_class *cls;
} sl_object;

typedef struct {
    char     *name;
    sl_value *value;
} sl_const_entry;

struct sl_server {
    char *name, *version, *desc;
    void *user_ctx;

    sl_func_entry *funcs;   size_t func_count, func_cap;
    sl_class     **classes; size_t class_count, class_cap;
    sl_const_entry *consts; size_t const_count, const_cap;

    sl_object   **objects;  size_t object_count, object_cap;
    int64_t       next_id;
    int64_t       next_rpc_id;
};

/* ================================================================== */
/*  I/O helpers                                                       */
/* ================================================================== */

static void send_line(const char *line) {
    fputs(line, stdout);
    fputc('\n', stdout);
    fflush(stdout);
}

static char *read_line(void) {
    sbuf buf; sb_init(&buf);
    int ch;
    while ((ch = fgetc(stdin)) != EOF) {
        if (ch == '\n') break;
        if (ch == '\r') continue;
        sb_putc(&buf, (char)ch);
    }
    if (buf.n == 0 && ch == EOF) { sb_free(&buf); return NULL; }
    return buf.b;
}

static void send_error(int64_t id, const char *msg) {
    sbuf s; sb_init(&s);
    sb_printf(&s, "{\"jsonrpc\":\"2.0\",\"id\":%lld,\"error\":{\"code\":-32000,\"message\":", (long long)id);
    sb_json_str(&s, msg, strlen(msg));
    sb_puts(&s, "}}");
    send_line(s.b); sb_free(&s);
}

static void send_result_null(int64_t id) {
    sbuf s; sb_init(&s);
    sb_printf(&s, "{\"jsonrpc\":\"2.0\",\"id\":%lld,\"result\":null}", (long long)id);
    send_line(s.b); sb_free(&s);
}

static void send_result_json(int64_t id, const char *json) {
    sbuf s; sb_init(&s);
    sb_printf(&s, "{\"jsonrpc\":\"2.0\",\"id\":%lld,\"result\":%s}", (long long)id, json);
    send_line(s.b); sb_free(&s);
}

/* ================================================================== */
/*  Argument extraction helpers                                       */
/* ================================================================== */

static sl_value **extract_args(const jval *params, int *out_count) {
    *out_count = 0;
    const jval *args = jget(params, "args");
    if (!args || args->t != JT_ARR) return NULL;
    int n = (int)args->u.arr.n;
    if (n == 0) return NULL;
    sl_value **vals = malloc((size_t)n * sizeof(*vals));
    for (int i = 0; i < n; i++) vals[i] = parse_transport_val(args->u.arr.a[i]);
    *out_count = n;
    return vals;
}

static void free_args(sl_value **args, int count) {
    if (!args) return;
    for (int i = 0; i < count; i++) sl_value_free(args[i]);
    free(args);
}

/* ================================================================== */
/*  Object store                                                      */
/* ================================================================== */

static void store_object(sl_server *srv, sl_class *cls, void *data) {
    if (srv->object_count >= srv->object_cap) {
        srv->object_cap = srv->object_cap ? srv->object_cap * 2 : 16;
        srv->objects = realloc(srv->objects, srv->object_cap * sizeof(*srv->objects));
    }
    sl_object *obj = calloc(1, sizeof(*obj));
    obj->cls = cls;
    obj->data = data;
    srv->objects[srv->object_count++] = obj;
}

static sl_object *get_object(sl_server *srv, const char *id_str) {
    if (!id_str) return NULL;
    int64_t id = atoll(id_str);
    if (id <= 0 || (size_t)id > srv->object_count) return NULL;
    return srv->objects[(size_t)id - 1];
}

static void destroy_object(sl_server *srv, const char *id_str) {
    sl_object *obj = get_object(srv, id_str);
    if (!obj) return;
    size_t idx = (size_t)(atoll(id_str) - 1);
    if (obj->cls->dtor) obj->cls->dtor(obj->data);
    free(obj);
    srv->objects[idx] = NULL;
}

/* ================================================================== */
/*  Forward declarations for dispatch                                 */
/* ================================================================== */

static void dispatch_request(sl_server *srv, const char *method, const jval *params, int64_t id);

/* ================================================================== */
/*  RPC call with nested request handling (for callbacks/logging)     */
/* ================================================================== */

static char *do_rpc_call(sl_server *srv, const char *method, const char *params_json) {
    int64_t id = ++srv->next_rpc_id;
    {
        sbuf s; sb_init(&s);
        sb_printf(&s, "{\"jsonrpc\":\"2.0\",\"id\":%lld,\"method\":", (long long)id);
        sb_json_str(&s, method, strlen(method));
        if (params_json) { sb_puts(&s, ",\"params\":"); sb_puts(&s, params_json); }
        sb_putc(&s, '}');
        send_line(s.b); sb_free(&s);
    }

    for (;;) {
        char *line = read_line();
        if (!line) return NULL;
        jval *msg = json_parse(line);
        if (!msg || msg->t != JT_OBJ) { jfree(msg); free(line); continue; }

        /* If the host sends us a request while we're waiting (shouldn't
         * happen in single-threaded mode, but handle gracefully), respond. */
        const char *m = jget_str(msg, "method");
        if (m) {
            int64_t rid = jget_int(msg, "id", -1);
            const jval *p = jget(msg, "params");
            dispatch_request(srv, m, p ? p : jnew(JT_NULL), rid);
            jfree(msg); free(line);
            continue;
        }

        int64_t rid = jget_int(msg, "id", -1);
        if (rid == id) {
            const jval *err = jget(msg, "error");
            if (err) {
                const char *emsg = jget_str(err, "message");
                sbuf s; sb_init(&s);
                sb_puts(&s, "{\"__rpc_error\":");
                sb_json_str(&s, emsg ? emsg : "unknown", emsg ? strlen(emsg) : 7);
                sb_putc(&s, '}');
                char *result = s.b;
                jfree(msg); free(line);
                return result;
            }
            const jval *res = jget(msg, "result");
            sbuf s; sb_init(&s);
            json_write_val(&s, res ? res : jnew(JT_NULL));
            char *result = s.b;
            jfree(msg); free(line);
            return result;
        }
        jfree(msg); free(line);
    }
}

/* ================================================================== */
/*  Server API                                                        */
/* ================================================================== */

sl_server *sl_server_new(const char *name, const char *version, const char *description) {
    sl_server *srv = calloc(1, sizeof(*srv));
    srv->name = strdup(name);
    srv->version = strdup(version);
    srv->desc = strdup(description);
    srv->next_id = 1;
    srv->next_rpc_id = 100000;
    return srv;
}

void sl_server_free(sl_server *srv) {
    if (!srv) return;
    free(srv->name); free(srv->version); free(srv->desc);
    for (size_t i = 0; i < srv->func_count; i++) { free(srv->funcs[i].name); free(srv->funcs[i].source); free(srv->funcs[i].description); }
    free(srv->funcs);
    for (size_t i = 0; i < srv->class_count; i++) sl_class_free(srv->classes[i]);
    free(srv->classes);
    for (size_t i = 0; i < srv->const_count; i++) { free(srv->consts[i].name); sl_value_free(srv->consts[i].value); }
    free(srv->consts);
    for (size_t i = 0; i < srv->object_count; i++) {
        if (srv->objects[i]) { if (srv->objects[i]->cls->dtor) srv->objects[i]->cls->dtor(srv->objects[i]->data); free(srv->objects[i]); }
    }
    free(srv->objects);
    free(srv);
}

void sl_server_set_context(sl_server *srv, void *ctx) { srv->user_ctx = ctx; }

void sl_register_func(sl_server *srv, const char *name, sl_func_handler handler) {
    if (srv->func_count >= srv->func_cap) { srv->func_cap = srv->func_cap ? srv->func_cap * 2 : 8; srv->funcs = realloc(srv->funcs, srv->func_cap * sizeof(*srv->funcs)); }
    sl_func_entry *e = &srv->funcs[srv->func_count++];
    e->name = strdup(name); e->handler = handler; e->source = NULL; e->description = NULL;
}

void sl_register_func_help(sl_server *srv, const char *name,
                           sl_func_handler handler, const char *help_text) {
    if (srv->func_count >= srv->func_cap) { srv->func_cap = srv->func_cap ? srv->func_cap * 2 : 8; srv->funcs = realloc(srv->funcs, srv->func_cap * sizeof(*srv->funcs)); }
    sl_func_entry *e = &srv->funcs[srv->func_count++];
    e->name = strdup(name); e->handler = handler; e->source = NULL; e->description = strdup(help_text);
}

void sl_register_script_func(sl_server *srv, const char *name, const char *source) {
    if (srv->func_count >= srv->func_cap) { srv->func_cap = srv->func_cap ? srv->func_cap * 2 : 8; srv->funcs = realloc(srv->funcs, srv->func_cap * sizeof(*srv->funcs)); }
    sl_func_entry *e = &srv->funcs[srv->func_count++];
    e->name = strdup(name); e->handler = NULL; e->source = strdup(source); e->description = NULL;
}

sl_class *sl_class_new(const char *name) {
    sl_class *c = calloc(1, sizeof(*c)); c->name = strdup(name); return c;
}

void sl_class_free(sl_class *c) {
    if (!c) return;
    free(c->name); free(c->source);
    for (size_t i = 0; i < c->methods.count; i++) free(c->methods.names[i]);
    free(c->methods.names); free(c->methods.fns);
    for (size_t i = 0; i < c->props.count; i++) free(c->props.names[i]);
    free(c->props.names); free(c->props.getters); free(c->props.setters);
    free(c);
}

void sl_class_set_constructor(sl_class *c, sl_constructor_fn fn) { c->ctor = fn; }
void sl_class_set_destructor(sl_class *c, sl_destructor_fn fn)   { c->dtor = fn; }

void sl_class_add_method(sl_class *c, const char *name, sl_method_fn fn) {
    if (c->methods.count >= c->methods.cap) {
        c->methods.cap = c->methods.cap ? c->methods.cap * 2 : 8;
        c->methods.names = realloc(c->methods.names, c->methods.cap * sizeof(char *));
        c->methods.fns = realloc(c->methods.fns, c->methods.cap * sizeof(sl_method_fn));
    }
    c->methods.names[c->methods.count] = strdup(name);
    c->methods.fns[c->methods.count] = fn;
    c->methods.count++;
}

void sl_class_add_property(sl_class *c, const char *name,
                           sl_prop_getter_fn getter, sl_prop_setter_fn setter) {
    if (c->props.count >= c->props.cap) {
        c->props.cap = c->props.cap ? c->props.cap * 2 : 8;
        c->props.names = realloc(c->props.names, c->props.cap * sizeof(char *));
        c->props.getters = realloc(c->props.getters, c->props.cap * sizeof(sl_prop_getter_fn));
        c->props.setters = realloc(c->props.setters, c->props.cap * sizeof(sl_prop_setter_fn));
    }
    c->props.names[c->props.count] = strdup(name);
    c->props.getters[c->props.count] = getter;
    c->props.setters[c->props.count] = setter;
    c->props.count++;
}

void sl_register_class(sl_server *srv, sl_class *c) {
    if (srv->class_count >= srv->class_cap) { srv->class_cap = srv->class_cap ? srv->class_cap * 2 : 8; srv->classes = realloc(srv->classes, srv->class_cap * sizeof(*srv->classes)); }
    srv->classes[srv->class_count++] = c;
}

void sl_register_script_class(sl_server *srv, const char *name, const char *source) {
    sl_class *c = sl_class_new(name); c->source = strdup(source);
    sl_register_class(srv, c);
}

void sl_constant(sl_server *srv, const char *name, sl_value *value) {
    if (srv->const_count >= srv->const_cap) { srv->const_cap = srv->const_cap ? srv->const_cap * 2 : 8; srv->consts = realloc(srv->consts, srv->const_cap * sizeof(*srv->consts)); }
    srv->consts[srv->const_count].name = strdup(name);
    srv->consts[srv->const_count].value = value;
    srv->const_count++;
}

void sl_wrapper(sl_server *srv, const char *name, const char *source) {
    for (size_t i = 0; i < srv->func_count; i++) {
        if (strcmp(srv->funcs[i].name, name) == 0) { free(srv->funcs[i].source); srv->funcs[i].source = strdup(source); return; }
    }
    for (size_t i = 0; i < srv->class_count; i++) {
        if (strcmp(srv->classes[i]->name, name) == 0) { free(srv->classes[i]->source); srv->classes[i]->source = strdup(source); return; }
    }
}

/* ================================================================== */
/*  Handshake                                                         */
/* ================================================================== */

static void handle_handshake(sl_server *srv, int64_t id) {
    sbuf s; sb_init(&s);
    sb_printf(&s, "{\"jsonrpc\":\"2.0\",\"id\":%lld,\"result\":", (long long)id);
    sb_puts(&s, "{\"protocol\":\"1.0\",\"transport\":\"json\",");
    sb_puts(&s, "\"library\":{\"name\":"); sb_json_str(&s, srv->name, strlen(srv->name));
    sb_puts(&s, ",\"version\":"); sb_json_str(&s, srv->version, strlen(srv->version));
    sb_puts(&s, ",\"description\":"); sb_json_str(&s, srv->desc, strlen(srv->desc));
    sb_puts(&s, "},\"capabilities\":[\"remote_objects\"],\"schema\":{");

    sb_puts(&s, "\"functions\":[");
    for (size_t i = 0; i < srv->func_count; i++) {
        if (i) sb_putc(&s, ',');
        sb_putc(&s, '{'); sb_puts(&s, "\"name\":"); sb_json_str(&s, srv->funcs[i].name, strlen(srv->funcs[i].name));
        if (srv->funcs[i].source) { sb_puts(&s, ",\"source\":"); sb_json_str(&s, srv->funcs[i].source, strlen(srv->funcs[i].source)); }
        if (srv->funcs[i].description) { sb_puts(&s, ",\"description\":"); sb_json_str(&s, srv->funcs[i].description, strlen(srv->funcs[i].description)); }
        sb_putc(&s, '}');
    }
    sb_puts(&s, "],\"classes\":[");
    for (size_t i = 0; i < srv->class_count; i++) {
        sl_class *c = srv->classes[i];
        if (i) sb_putc(&s, ',');
        sb_putc(&s, '{'); sb_puts(&s, "\"name\":"); sb_json_str(&s, c->name, strlen(c->name));
        if (c->source) {
            sb_puts(&s, ",\"source\":"); sb_json_str(&s, c->source, strlen(c->source));
        } else {
            sb_puts(&s, ",\"constructor\":{\"name\":"); sb_json_str(&s, c->name, strlen(c->name)); sb_putc(&s, '}');
            sb_puts(&s, ",\"methods\":[");
            for (size_t j = 0; j < c->methods.count; j++) {
                if (j) sb_putc(&s, ',');
                sb_puts(&s, "{\"name\":"); sb_json_str(&s, c->methods.names[j], strlen(c->methods.names[j])); sb_putc(&s, '}');
            }
            sb_puts(&s, "]");
            if (c->props.count > 0) {
                sb_puts(&s, ",\"properties\":[");
                for (size_t j = 0; j < c->props.count; j++) {
                    if (j) sb_putc(&s, ',');
                    sb_puts(&s, "{\"name\":"); sb_json_str(&s, c->props.names[j], strlen(c->props.names[j]));
                    sb_puts(&s, ",\"settable\":"); sb_puts(&s, c->props.setters[j] ? "true" : "false");
                    sb_putc(&s, '}');
                }
                sb_puts(&s, "]");
            }
        }
        sb_putc(&s, '}');
    }
    sb_puts(&s, "],\"constants\":[");
    for (size_t i = 0; i < srv->const_count; i++) {
        if (i) sb_putc(&s, ',');
        sb_putc(&s, '{'); sb_puts(&s, "\"name\":"); sb_json_str(&s, srv->consts[i].name, strlen(srv->consts[i].name));
        sb_puts(&s, ",\"value\":"); val_to_json(&s, srv->consts[i].value);
        sb_putc(&s, '}');
    }
    sb_puts(&s, "]}}}");

    send_line(s.b); sb_free(&s);
}

/* ================================================================== */
/*  Dispatch                                                          */
/* ================================================================== */

static void dispatch_request(sl_server *srv, const char *method,
                             const jval *params, int64_t id) {
    if (strcmp(method, "scriptling.handshake") == 0) {
        handle_handshake(srv, id);
        return;
    }
    if (strcmp(method, "environment.open") == 0 || strcmp(method, "environment.close") == 0) {
        send_result_null(id);
        return;
    }
    if (strcmp(method, "plugin.shutdown") == 0) {
        send_result_null(id);
        for (size_t i = 0; i < srv->object_count; i++) {
            if (srv->objects[i]) {
                if (srv->objects[i]->cls->dtor) srv->objects[i]->cls->dtor(srv->objects[i]->data);
                free(srv->objects[i]);
            }
        }
        srv->object_count = 0;
        return;
    }

    if (strcmp(method, "function.call") == 0) {
        const char *fname = params ? jget_str(params, "name") : NULL;
        if (!fname) { send_error(id, "missing function name"); return; }

        sl_func_entry *fe = NULL;
        for (size_t i = 0; i < srv->func_count; i++) {
            if (strcmp(srv->funcs[i].name, fname) == 0) { fe = &srv->funcs[i]; break; }
        }
        if (!fe || !fe->handler) {
            sbuf e; sb_init(&e); sb_printf(&e, "unknown function %s", fname);
            send_error(id, e.b); sb_free(&e);
            return;
        }

        int argc = 0;
        sl_value **args = extract_args(params, &argc);
        sl_value *result = fe->handler(argc, args, srv->user_ctx);
        free_args(args, argc);

        if (result) {
            sbuf s; sb_init(&s); val_to_json(&s, result);
            send_result_json(id, s.b); sb_free(&s); sl_value_free(result);
        } else {
            send_result_null(id);
        }
        return;
    }

    if (strcmp(method, "object.new") == 0) {
        const char *cls_name = params ? jget_str(params, "class") : NULL;
        if (!cls_name) { send_error(id, "missing class name"); return; }

        sl_class *cls = NULL;
        for (size_t i = 0; i < srv->class_count; i++) {
            if (strcmp(srv->classes[i]->name, cls_name) == 0) { cls = srv->classes[i]; break; }
        }
        if (!cls) {
            sbuf e; sb_init(&e); sb_printf(&e, "unknown class %s", cls_name);
            send_error(id, e.b); sb_free(&e);
            return;
        }

        int argc = 0;
        sl_value **args = extract_args(params, &argc);
        void *data = cls->ctor ? cls->ctor(argc, args, srv->user_ctx) : NULL;
        free_args(args, argc);

        store_object(srv, cls, data);
        int64_t obj_id = (int64_t)srv->object_count;

        sbuf ref; sb_init(&ref);
        sb_puts(&ref, "{\"library\":"); sb_json_str(&ref, srv->name, strlen(srv->name));
        sb_puts(&ref, ",\"class\":"); sb_json_str(&ref, cls->name, strlen(cls->name));
        sb_printf(&ref, ",\"id\":\"%lld\"}", (long long)obj_id);
        send_result_json(id, ref.b); sb_free(&ref);
        return;
    }

    if (strcmp(method, "object.call_method") == 0) {
        const char *obj_id_str = params ? jget_str(params, "object_id") : NULL;
        const char *mname = params ? jget_str(params, "method") : NULL;

        sl_object *obj = get_object(srv, obj_id_str);
        if (!obj) { send_error(id, "unknown object"); return; }

        int argc = 0;
        sl_value **args = extract_args(params, &argc);

        for (size_t i = 0; i < obj->cls->props.count; i++) {
            if (strcmp(obj->cls->props.names[i], mname) == 0) {
                sl_value *result = NULL;
                if (argc == 0 && obj->cls->props.getters[i]) {
                    result = obj->cls->props.getters[i](obj->data, srv->user_ctx);
                } else if (argc == 1 && obj->cls->props.setters[i]) {
                    obj->cls->props.setters[i](obj->data, args[0], srv->user_ctx);
                    result = sl_null();
                } else if (argc == 0) {
                    send_error(id, "property is write-only");
                    free_args(args, argc); return;
                } else {
                    send_error(id, "property is read-only");
                    free_args(args, argc); return;
                }
                free_args(args, argc);
                if (result) {
                    sbuf s; sb_init(&s); val_to_json(&s, result);
                    send_result_json(id, s.b); sb_free(&s); sl_value_free(result);
                } else { send_result_null(id); }
                return;
            }
        }

        sl_method_fn fn = NULL;
        for (size_t i = 0; i < obj->cls->methods.count; i++) {
            if (strcmp(obj->cls->methods.names[i], mname) == 0) { fn = obj->cls->methods.fns[i]; break; }
        }
        if (!fn) {
            sbuf e; sb_init(&e); sb_printf(&e, "unknown method %s on %s", mname ? mname : "(null)", obj->cls->name);
            send_error(id, e.b); sb_free(&e); free_args(args, argc);
            return;
        }

        sl_value *result = fn(obj->data, argc, args, srv->user_ctx);
        free_args(args, argc);

        if (result) {
            sbuf s; sb_init(&s); val_to_json(&s, result);
            send_result_json(id, s.b); sb_free(&s); sl_value_free(result);
        } else { send_result_null(id); }
        return;
    }

    if (strcmp(method, "object.destroy") == 0) {
        const char *obj_id_str = params ? jget_str(params, "object_id") : NULL;
        if (obj_id_str) destroy_object(srv, obj_id_str);
        send_result_null(id);
        return;
    }

    sbuf e; sb_init(&e); sb_printf(&e, "unknown method %s", method ? method : "(null)");
    send_error(id, e.b); sb_free(&e);
}

/* ================================================================== */
/*  Run loop                                                          */
/* ================================================================== */

int sl_server_run(sl_server *srv) {
    tl_server = srv;
    for (;;) {
        char *line = read_line();
        if (!line) break;

        jval *root = json_parse(line);
        free(line);
        if (!root || root->t != JT_OBJ) { jfree(root); continue; }

        const char *method = jget_str(root, "method");
        int64_t id = jget_int(root, "id", -1);
        const jval *params = jget(root, "params");

        if (method) {
            dispatch_request(srv, method, params ? params : jnew(JT_NULL), id);
            if (strcmp(method, "plugin.shutdown") == 0) { jfree(root); break; }
        }
        jfree(root);
    }
    tl_server = NULL;
    return 0;
}

/* ================================================================== */
/*  Callback support                                                  */
/* ================================================================== */

sl_value *sl_callback_call(const sl_value *cb, int argc, sl_value **args,
                           char **err_msg) {
    if (err_msg) *err_msg = NULL;
    if (!cb || cb->type != SL_CALLBACK || !cb->callback_id) {
        if (err_msg) *err_msg = strdup("not a callback value");
        return NULL;
    }

    sl_server *srv = tl_server;
    if (!srv) {
        if (err_msg) *err_msg = strdup("no active server");
        return NULL;
    }

    sbuf params; sb_init(&params);
    sb_puts(&params, "{\"id\":"); sb_json_str(&params, cb->callback_id, strlen(cb->callback_id));
    if (argc > 0) {
        sb_puts(&params, ",\"args\":[");
        for (int i = 0; i < argc; i++) {
            if (i) sb_putc(&params, ',');
            val_to_json(&params, args[i]);
        }
        sb_putc(&params, ']');
    }
    sb_putc(&params, '}');

    char *resp = do_rpc_call(srv, "callback.call", params.b);
    sb_free(&params);

    if (!resp) {
        if (err_msg) *err_msg = strdup("no response from host");
        return NULL;
    }

    if (strncmp(resp, "{\"__rpc_error\":", 15) == 0) {
        jval *r = json_parse(resp);
        if (r) {
            const char *msg = jget_str(r, "__rpc_error");
            if (err_msg) *err_msg = strdup(msg ? msg : "callback error");
            jfree(r);
        } else {
            if (err_msg) *err_msg = strdup("callback error");
        }
        free(resp);
        return NULL;
    }

    jval *r = json_parse(resp);
    free(resp);
    if (!r) {
        if (err_msg) *err_msg = strdup("invalid callback response");
        return NULL;
    }
    sl_value *result = parse_transport_val(r);
    jfree(r);
    return result;
}

/* ================================================================== */
/*  Logging                                                           */
/* ================================================================== */

static void do_log(const char *level, const char *fmt, va_list ap) {
    sl_server *srv = tl_server;
    if (!srv) return;

    char msg[4096];
    vsnprintf(msg, sizeof(msg), fmt, ap);

    sbuf params; sb_init(&params);
    sb_puts(&params, "{\"level\":"); sb_json_str(&params, level, strlen(level));
    sb_puts(&params, ",\"message\":"); sb_json_str(&params, msg, strlen(msg));
    sb_putc(&params, '}');

    char *resp = do_rpc_call(srv, "host.log", params.b);
    sb_free(&params);
    free(resp);
}

void sl_log_trace(const char *fmt, ...) { va_list ap; va_start(ap, fmt); do_log("trace", fmt, ap); va_end(ap); }
void sl_log_debug(const char *fmt, ...) { va_list ap; va_start(ap, fmt); do_log("debug", fmt, ap); va_end(ap); }
void sl_log_info(const char *fmt, ...)  { va_list ap; va_start(ap, fmt); do_log("info", fmt, ap); va_end(ap); }
void sl_log_warn(const char *fmt, ...)  { va_list ap; va_start(ap, fmt); do_log("warn", fmt, ap); va_end(ap); }
void sl_log_error(const char *fmt, ...) { va_list ap; va_start(ap, fmt); do_log("error", fmt, ap); va_end(ap); }
