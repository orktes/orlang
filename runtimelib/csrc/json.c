/*
 * Orlang runtime: JSON parsing, building, and serialization.
 *
 * Values are GC-allocated handles inspected and built through accessor
 * functions; the orlang standard library (std/json.or) wraps them in a
 * Json struct. Invalid lookups return a shared "invalid" value instead of
 * NULL so chained access never crashes.
 *
 * Types: 0=invalid 1=null 2=bool 3=number 4=string 5=array 6=object
 */

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifndef ORLANG_WEAK
#if defined(_WIN32)
#define ORLANG_WEAK
#else
#define ORLANG_WEAK __attribute__((weak))
#endif
#endif

void *GC_malloc(size_t size);

#define ORL_JSON_INVALID 0
#define ORL_JSON_NULL 1
#define ORL_JSON_BOOL 2
#define ORL_JSON_NUMBER 3
#define ORL_JSON_STRING 4
#define ORL_JSON_ARRAY 5
#define ORL_JSON_OBJECT 6

#define ORL_JSON_MAX_DEPTH 128

typedef struct orl_json {
  int32_t type;
  int32_t boolean;
  double number;
  char *string;
  int64_t len;
  int64_t cap;
  struct orl_json **items; /* array elements or object values */
  char **keys;             /* object keys */
} orl_json_t;

static orl_json_t orl_json_invalid_value = {ORL_JSON_INVALID, 0, 0, "", 0, 0, NULL, NULL};

static orl_json_t *orl_json_new(int32_t type) {
  orl_json_t *v = (orl_json_t *)GC_malloc(sizeof(orl_json_t));
  v->type = type;
  v->string = "";
  return v;
}

/* ------------------------------------------------------------------ */
/* Building                                                           */
/* ------------------------------------------------------------------ */

ORLANG_WEAK void *json_new_object(void) {
  return orl_json_new(ORL_JSON_OBJECT);
}

ORLANG_WEAK void *json_new_array(void) {
  return orl_json_new(ORL_JSON_ARRAY);
}

ORLANG_WEAK void *json_new_string(char *s) {
  orl_json_t *v = orl_json_new(ORL_JSON_STRING);
  size_t n = strlen(s);
  v->string = (char *)GC_malloc(n + 1);
  memcpy(v->string, s, n + 1);
  return v;
}

ORLANG_WEAK void *json_new_number(double d) {
  orl_json_t *v = orl_json_new(ORL_JSON_NUMBER);
  v->number = d;
  return v;
}

ORLANG_WEAK void *json_new_bool(int32_t b) {
  orl_json_t *v = orl_json_new(ORL_JSON_BOOL);
  v->boolean = b != 0;
  return v;
}

ORLANG_WEAK void *json_new_null(void) {
  return orl_json_new(ORL_JSON_NULL);
}

static void orl_json_grow(orl_json_t *v) {
  if (v->len < v->cap) {
    return;
  }
  int64_t ncap = v->cap == 0 ? 8 : v->cap * 2;
  orl_json_t **items = (orl_json_t **)GC_malloc((size_t)ncap * sizeof(void *));
  char **keys = (char **)GC_malloc((size_t)ncap * sizeof(char *));
  for (int64_t i = 0; i < v->len; i++) {
    items[i] = v->items[i];
    if (v->keys != NULL) {
      keys[i] = v->keys[i];
    }
  }
  v->items = items;
  v->keys = keys;
  v->cap = ncap;
}

ORLANG_WEAK void json_push(void *arrp, void *valp) {
  orl_json_t *arr = (orl_json_t *)arrp;
  if (arr == NULL || arr->type != ORL_JSON_ARRAY || valp == NULL) {
    return;
  }
  orl_json_grow(arr);
  arr->items[arr->len++] = (orl_json_t *)valp;
}

ORLANG_WEAK void json_set(void *objp, char *key, void *valp) {
  orl_json_t *obj = (orl_json_t *)objp;
  if (obj == NULL || obj->type != ORL_JSON_OBJECT || valp == NULL) {
    return;
  }
  for (int64_t i = 0; i < obj->len; i++) {
    if (strcmp(obj->keys[i], key) == 0) {
      obj->items[i] = (orl_json_t *)valp;
      return;
    }
  }
  orl_json_grow(obj);
  size_t n = strlen(key);
  char *k = (char *)GC_malloc(n + 1);
  memcpy(k, key, n + 1);
  obj->keys[obj->len] = k;
  obj->items[obj->len] = (orl_json_t *)valp;
  obj->len++;
}

/* ------------------------------------------------------------------ */
/* Inspection                                                         */
/* ------------------------------------------------------------------ */

ORLANG_WEAK int32_t json_type(void *vp) {
  return vp == NULL ? ORL_JSON_INVALID : ((orl_json_t *)vp)->type;
}

ORLANG_WEAK double json_number(void *vp) {
  orl_json_t *v = (orl_json_t *)vp;
  return (v != NULL && v->type == ORL_JSON_NUMBER) ? v->number : 0;
}

ORLANG_WEAK char *json_string(void *vp) {
  orl_json_t *v = (orl_json_t *)vp;
  return (v != NULL && v->type == ORL_JSON_STRING) ? v->string : "";
}

ORLANG_WEAK int32_t json_bool(void *vp) {
  orl_json_t *v = (orl_json_t *)vp;
  return (v != NULL && v->type == ORL_JSON_BOOL) ? v->boolean : 0;
}

ORLANG_WEAK int32_t json_len(void *vp) {
  orl_json_t *v = (orl_json_t *)vp;
  if (v == NULL) {
    return 0;
  }
  if (v->type == ORL_JSON_ARRAY || v->type == ORL_JSON_OBJECT) {
    return (int32_t)v->len;
  }
  return 0;
}

ORLANG_WEAK void *json_get(void *vp, char *key) {
  orl_json_t *v = (orl_json_t *)vp;
  if (v != NULL && v->type == ORL_JSON_OBJECT) {
    for (int64_t i = 0; i < v->len; i++) {
      if (strcmp(v->keys[i], key) == 0) {
        return v->items[i];
      }
    }
  }
  return &orl_json_invalid_value;
}

ORLANG_WEAK void *json_index(void *vp, int32_t i) {
  orl_json_t *v = (orl_json_t *)vp;
  if (v != NULL && (v->type == ORL_JSON_ARRAY || v->type == ORL_JSON_OBJECT) &&
      i >= 0 && (int64_t)i < v->len) {
    return v->items[i];
  }
  return &orl_json_invalid_value;
}

ORLANG_WEAK char *json_key(void *vp, int32_t i) {
  orl_json_t *v = (orl_json_t *)vp;
  if (v != NULL && v->type == ORL_JSON_OBJECT && i >= 0 && (int64_t)i < v->len) {
    return v->keys[i];
  }
  return "";
}

/* ------------------------------------------------------------------ */
/* Parsing                                                            */
/* ------------------------------------------------------------------ */

typedef struct {
  const char *cur;
  int depth;
  int failed;
} orl_json_parser_t;

static void orl_json_skip_ws(orl_json_parser_t *p) {
  while (*p->cur == ' ' || *p->cur == '\t' || *p->cur == '\n' || *p->cur == '\r') {
    p->cur++;
  }
}

static orl_json_t *orl_json_parse_value(orl_json_parser_t *p);

static int orl_json_hex(char c) {
  if (c >= '0' && c <= '9') return c - '0';
  if (c >= 'a' && c <= 'f') return c - 'a' + 10;
  if (c >= 'A' && c <= 'F') return c - 'A' + 10;
  return -1;
}

/* Parses a JSON string literal (cursor on opening quote). */
static char *orl_json_parse_string(orl_json_parser_t *p) {
  if (*p->cur != '"') {
    p->failed = 1;
    return "";
  }
  p->cur++;

  size_t cap = 16;
  size_t len = 0;
  char *buf = (char *)GC_malloc(cap + 4);

  while (*p->cur != '"') {
    if (*p->cur == '\0') {
      p->failed = 1;
      return "";
    }
    if (len + 4 >= cap) {
      size_t ncap = cap * 2;
      char *nbuf = (char *)GC_malloc(ncap + 4);
      memcpy(nbuf, buf, len);
      buf = nbuf;
      cap = ncap;
    }

    char c = *p->cur;
    if (c != '\\') {
      buf[len++] = c;
      p->cur++;
      continue;
    }

    p->cur++;
    switch (*p->cur) {
    case '"':  buf[len++] = '"'; break;
    case '\\': buf[len++] = '\\'; break;
    case '/':  buf[len++] = '/'; break;
    case 'b':  buf[len++] = '\b'; break;
    case 'f':  buf[len++] = '\f'; break;
    case 'n':  buf[len++] = '\n'; break;
    case 'r':  buf[len++] = '\r'; break;
    case 't':  buf[len++] = '\t'; break;
    case 'u': {
      int h1 = orl_json_hex(p->cur[1]);
      int h2 = orl_json_hex(p->cur[2]);
      int h3 = orl_json_hex(p->cur[3]);
      int h4 = orl_json_hex(p->cur[4]);
      if (h1 < 0 || h2 < 0 || h3 < 0 || h4 < 0) {
        p->failed = 1;
        return "";
      }
      unsigned int cp = (unsigned int)((h1 << 12) | (h2 << 8) | (h3 << 4) | h4);
      p->cur += 4;
      /* Encode as UTF-8 (surrogate pairs are not combined). */
      if (cp < 0x80) {
        buf[len++] = (char)cp;
      } else if (cp < 0x800) {
        buf[len++] = (char)(0xC0 | (cp >> 6));
        buf[len++] = (char)(0x80 | (cp & 0x3F));
      } else {
        buf[len++] = (char)(0xE0 | (cp >> 12));
        buf[len++] = (char)(0x80 | ((cp >> 6) & 0x3F));
        buf[len++] = (char)(0x80 | (cp & 0x3F));
      }
      break;
    }
    default:
      p->failed = 1;
      return "";
    }
    p->cur++;
  }
  p->cur++; /* closing quote */
  buf[len] = '\0';
  return buf;
}

static orl_json_t *orl_json_parse_value(orl_json_parser_t *p) {
  if (p->depth >= ORL_JSON_MAX_DEPTH) {
    p->failed = 1;
    return &orl_json_invalid_value;
  }

  orl_json_skip_ws(p);

  switch (*p->cur) {
  case '{': {
    p->cur++;
    p->depth++;
    orl_json_t *obj = orl_json_new(ORL_JSON_OBJECT);
    orl_json_skip_ws(p);
    if (*p->cur == '}') {
      p->cur++;
      p->depth--;
      return obj;
    }
    for (;;) {
      orl_json_skip_ws(p);
      char *key = orl_json_parse_string(p);
      if (p->failed) {
        return &orl_json_invalid_value;
      }
      orl_json_skip_ws(p);
      if (*p->cur != ':') {
        p->failed = 1;
        return &orl_json_invalid_value;
      }
      p->cur++;
      orl_json_t *val = orl_json_parse_value(p);
      if (p->failed) {
        return &orl_json_invalid_value;
      }
      json_set(obj, key, val);
      orl_json_skip_ws(p);
      if (*p->cur == ',') {
        p->cur++;
        continue;
      }
      if (*p->cur == '}') {
        p->cur++;
        p->depth--;
        return obj;
      }
      p->failed = 1;
      return &orl_json_invalid_value;
    }
  }
  case '[': {
    p->cur++;
    p->depth++;
    orl_json_t *arr = orl_json_new(ORL_JSON_ARRAY);
    orl_json_skip_ws(p);
    if (*p->cur == ']') {
      p->cur++;
      p->depth--;
      return arr;
    }
    for (;;) {
      orl_json_t *val = orl_json_parse_value(p);
      if (p->failed) {
        return &orl_json_invalid_value;
      }
      json_push(arr, val);
      orl_json_skip_ws(p);
      if (*p->cur == ',') {
        p->cur++;
        continue;
      }
      if (*p->cur == ']') {
        p->cur++;
        p->depth--;
        return arr;
      }
      p->failed = 1;
      return &orl_json_invalid_value;
    }
  }
  case '"': {
    char *s = orl_json_parse_string(p);
    if (p->failed) {
      return &orl_json_invalid_value;
    }
    orl_json_t *v = orl_json_new(ORL_JSON_STRING);
    v->string = s;
    return v;
  }
  case 't':
    if (strncmp(p->cur, "true", 4) == 0) {
      p->cur += 4;
      orl_json_t *v = orl_json_new(ORL_JSON_BOOL);
      v->boolean = 1;
      return v;
    }
    p->failed = 1;
    return &orl_json_invalid_value;
  case 'f':
    if (strncmp(p->cur, "false", 5) == 0) {
      p->cur += 5;
      return orl_json_new(ORL_JSON_BOOL);
    }
    p->failed = 1;
    return &orl_json_invalid_value;
  case 'n':
    if (strncmp(p->cur, "null", 4) == 0) {
      p->cur += 4;
      return orl_json_new(ORL_JSON_NULL);
    }
    p->failed = 1;
    return &orl_json_invalid_value;
  default: {
    char *end = NULL;
    double d = strtod(p->cur, &end);
    if (end == p->cur) {
      p->failed = 1;
      return &orl_json_invalid_value;
    }
    p->cur = end;
    orl_json_t *v = orl_json_new(ORL_JSON_NUMBER);
    v->number = d;
    return v;
  }
  }
}

ORLANG_WEAK void *json_parse(char *text) {
  if (text == NULL) {
    return &orl_json_invalid_value;
  }
  orl_json_parser_t p = {text, 0, 0};
  orl_json_t *v = orl_json_parse_value(&p);
  if (p.failed) {
    return &orl_json_invalid_value;
  }
  orl_json_skip_ws(&p);
  if (*p.cur != '\0') {
    return &orl_json_invalid_value;
  }
  return v;
}

/* ------------------------------------------------------------------ */
/* Serialization                                                      */
/* ------------------------------------------------------------------ */

typedef struct {
  char *buf;
  size_t len;
  size_t cap;
} orl_json_writer_t;

static void orl_json_write(orl_json_writer_t *w, const char *s, size_t n) {
  if (w->len + n + 1 > w->cap) {
    size_t ncap = w->cap == 0 ? 64 : w->cap;
    while (w->len + n + 1 > ncap) {
      ncap *= 2;
    }
    char *nbuf = (char *)GC_malloc(ncap);
    memcpy(nbuf, w->buf, w->len);
    w->buf = nbuf;
    w->cap = ncap;
  }
  memcpy(w->buf + w->len, s, n);
  w->len += n;
  w->buf[w->len] = '\0';
}

static void orl_json_write_string(orl_json_writer_t *w, const char *s) {
  orl_json_write(w, "\"", 1);
  for (const char *c = s; *c != '\0'; c++) {
    switch (*c) {
    case '"':  orl_json_write(w, "\\\"", 2); break;
    case '\\': orl_json_write(w, "\\\\", 2); break;
    case '\b': orl_json_write(w, "\\b", 2); break;
    case '\f': orl_json_write(w, "\\f", 2); break;
    case '\n': orl_json_write(w, "\\n", 2); break;
    case '\r': orl_json_write(w, "\\r", 2); break;
    case '\t': orl_json_write(w, "\\t", 2); break;
    default:
      if ((unsigned char)*c < 0x20) {
        char esc[8];
        int n = snprintf(esc, sizeof(esc), "\\u%04x", *c);
        orl_json_write(w, esc, (size_t)n);
      } else {
        orl_json_write(w, c, 1);
      }
    }
  }
  orl_json_write(w, "\"", 1);
}

static void orl_json_stringify_value(orl_json_writer_t *w, orl_json_t *v) {
  if (v == NULL) {
    orl_json_write(w, "null", 4);
    return;
  }
  switch (v->type) {
  case ORL_JSON_NULL:
  case ORL_JSON_INVALID:
    orl_json_write(w, "null", 4);
    break;
  case ORL_JSON_BOOL:
    if (v->boolean) {
      orl_json_write(w, "true", 4);
    } else {
      orl_json_write(w, "false", 5);
    }
    break;
  case ORL_JSON_NUMBER: {
    char num[32];
    int n;
    if (v->number == (double)(int64_t)v->number &&
        v->number >= -9007199254740992.0 && v->number <= 9007199254740992.0) {
      n = snprintf(num, sizeof(num), "%lld", (long long)v->number);
    } else {
      n = snprintf(num, sizeof(num), "%.17g", v->number);
    }
    orl_json_write(w, num, (size_t)n);
    break;
  }
  case ORL_JSON_STRING:
    orl_json_write_string(w, v->string);
    break;
  case ORL_JSON_ARRAY:
    orl_json_write(w, "[", 1);
    for (int64_t i = 0; i < v->len; i++) {
      if (i > 0) {
        orl_json_write(w, ",", 1);
      }
      orl_json_stringify_value(w, v->items[i]);
    }
    orl_json_write(w, "]", 1);
    break;
  case ORL_JSON_OBJECT:
    orl_json_write(w, "{", 1);
    for (int64_t i = 0; i < v->len; i++) {
      if (i > 0) {
        orl_json_write(w, ",", 1);
      }
      orl_json_write_string(w, v->keys[i]);
      orl_json_write(w, ":", 1);
      orl_json_stringify_value(w, v->items[i]);
    }
    orl_json_write(w, "}", 1);
    break;
  }
}

ORLANG_WEAK char *json_stringify(void *vp) {
  orl_json_writer_t w = {NULL, 0, 0};
  orl_json_write(&w, "", 0);
  orl_json_stringify_value(&w, (orl_json_t *)vp);
  return w.buf;
}
