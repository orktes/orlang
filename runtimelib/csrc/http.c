/*
 * Orlang runtime: HTTP server primitives.
 *
 * A small, dependency-free HTTP/1.1 server built on POSIX sockets. The
 * orlang standard library (std/http.or) wraps these functions in an
 * Express-style API. Single-threaded and blocking: one request is
 * handled at a time, connections are closed after each response.
 *
 * All memory is GC-allocated so handles and strings returned to orlang
 * code participate in garbage collection.
 */

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifndef _WIN32
#include <arpa/inet.h>
#include <netinet/in.h>
#include <signal.h>
#include <sys/socket.h>
#include <unistd.h>
#endif

#ifndef ORLANG_WEAK
#if defined(_WIN32)
#define ORLANG_WEAK
#else
#define ORLANG_WEAK __attribute__((weak))
#endif
#endif

void *GC_malloc(size_t size);

#define ORL_HTTP_MAX_HEADER_BYTES (64 * 1024)
#define ORL_HTTP_MAX_BODY_BYTES (8 * 1024 * 1024)
#define ORL_HTTP_MAX_HEADERS 64

typedef struct {
  int fd;
} orl_http_server_t;

typedef struct {
  int fd;

  /* request */
  char *method;
  char *path;
  char *query;
  int64_t header_count;
  char **header_names;
  char **header_values;
  char *body;

  /* response */
  int32_t status;
  int64_t res_header_count;
  char **res_header_names;
  char **res_header_values;
  int sent;
} orl_http_request_t;

static char *orl_http_strdup(const char *s, size_t n) {
  char *p = (char *)GC_malloc(n + 1);
  memcpy(p, s, n);
  p[n] = '\0';
  return p;
}

#ifndef _WIN32

ORLANG_WEAK void *http_server_create(int32_t port) {
  signal(SIGPIPE, SIG_IGN);

  int fd = socket(AF_INET, SOCK_STREAM, 0);
  if (fd < 0) {
    perror("orlang http: socket");
    return NULL;
  }

  int one = 1;
  setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));

  struct sockaddr_in addr;
  memset(&addr, 0, sizeof(addr));
  addr.sin_family = AF_INET;
  addr.sin_addr.s_addr = htonl(INADDR_ANY);
  addr.sin_port = htons((uint16_t)port);

  if (bind(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
    perror("orlang http: bind");
    close(fd);
    return NULL;
  }
  if (listen(fd, 16) != 0) {
    perror("orlang http: listen");
    close(fd);
    return NULL;
  }

  orl_http_server_t *srv = (orl_http_server_t *)GC_malloc(sizeof(orl_http_server_t));
  srv->fd = fd;
  return srv;
}

/* Reads from fd until the end of the HTTP header block. Returns the
 * buffer (GC-allocated) and sets *out_len / *out_header_end, or NULL. */
static char *orl_http_read_head(int fd, size_t *out_len, size_t *out_header_end) {
  size_t cap = 4096;
  size_t len = 0;
  char *buf = (char *)GC_malloc(cap + 1);

  for (;;) {
    if (len == cap) {
      if (cap >= ORL_HTTP_MAX_HEADER_BYTES) {
        return NULL;
      }
      size_t ncap = cap * 2;
      char *nbuf = (char *)GC_malloc(ncap + 1);
      memcpy(nbuf, buf, len);
      buf = nbuf;
      cap = ncap;
    }

    ssize_t n = read(fd, buf + len, cap - len);
    if (n <= 0) {
      return NULL;
    }
    len += (size_t)n;
    buf[len] = '\0';

    char *end = strstr(buf, "\r\n\r\n");
    if (end != NULL) {
      *out_len = len;
      *out_header_end = (size_t)(end - buf) + 4;
      return buf;
    }
  }
}

/* Parses the request head (request line + headers) into req. Returns 0 on
 * malformed input. */
static int orl_http_parse_head(orl_http_request_t *req, char *head, size_t header_end) {
  /* Request line: METHOD SP TARGET SP HTTP/1.x CRLF */
  char *line_end = strstr(head, "\r\n");
  if (line_end == NULL) {
    return 0;
  }
  *line_end = '\0';

  char *sp1 = strchr(head, ' ');
  if (sp1 == NULL) {
    return 0;
  }
  char *sp2 = strchr(sp1 + 1, ' ');
  if (sp2 == NULL) {
    return 0;
  }

  req->method = orl_http_strdup(head, (size_t)(sp1 - head));

  char *target = sp1 + 1;
  size_t target_len = (size_t)(sp2 - target);
  char *qmark = memchr(target, '?', target_len);
  if (qmark != NULL) {
    req->path = orl_http_strdup(target, (size_t)(qmark - target));
    req->query = orl_http_strdup(qmark + 1, target_len - (size_t)(qmark - target) - 1);
  } else {
    req->path = orl_http_strdup(target, target_len);
    req->query = orl_http_strdup("", 0);
  }

  /* Headers */
  req->header_names = (char **)GC_malloc(ORL_HTTP_MAX_HEADERS * sizeof(char *));
  req->header_values = (char **)GC_malloc(ORL_HTTP_MAX_HEADERS * sizeof(char *));

  char *cursor = line_end + 2;
  char *head_end = head + header_end - 2; /* points at the final CRLF pair */
  while (cursor < head_end && req->header_count < ORL_HTTP_MAX_HEADERS) {
    char *eol = strstr(cursor, "\r\n");
    if (eol == NULL || eol == cursor) {
      break;
    }
    char *colon = memchr(cursor, ':', (size_t)(eol - cursor));
    if (colon != NULL) {
      size_t name_len = (size_t)(colon - cursor);
      char *value = colon + 1;
      while (value < eol && (*value == ' ' || *value == '\t')) {
        value++;
      }
      req->header_names[req->header_count] = orl_http_strdup(cursor, name_len);
      req->header_values[req->header_count] = orl_http_strdup(value, (size_t)(eol - value));
      req->header_count++;
    }
    cursor = eol + 2;
  }

  return 1;
}

ORLANG_WEAK void *http_server_accept(void *server) {
  orl_http_server_t *srv = (orl_http_server_t *)server;
  if (srv == NULL) {
    return NULL;
  }

  int fd = accept(srv->fd, NULL, NULL);
  if (fd < 0) {
    return NULL;
  }

  size_t len = 0;
  size_t header_end = 0;
  char *head = orl_http_read_head(fd, &len, &header_end);
  if (head == NULL) {
    close(fd);
    return NULL;
  }

  orl_http_request_t *req = (orl_http_request_t *)GC_malloc(sizeof(orl_http_request_t));
  req->fd = fd;
  req->status = 200;
  req->res_header_names = (char **)GC_malloc(ORL_HTTP_MAX_HEADERS * sizeof(char *));
  req->res_header_values = (char **)GC_malloc(ORL_HTTP_MAX_HEADERS * sizeof(char *));

  if (!orl_http_parse_head(req, head, header_end)) {
    close(fd);
    return NULL;
  }

  /* Body: read Content-Length bytes (what we already have + the rest). */
  int64_t content_length = 0;
  for (int64_t i = 0; i < req->header_count; i++) {
    if (strcasecmp(req->header_names[i], "Content-Length") == 0) {
      content_length = atoll(req->header_values[i]);
      break;
    }
  }
  if (content_length < 0 || content_length > ORL_HTTP_MAX_BODY_BYTES) {
    close(fd);
    return NULL;
  }

  char *body = (char *)GC_malloc((size_t)content_length + 1);
  size_t have = len - header_end;
  if (have > (size_t)content_length) {
    have = (size_t)content_length;
  }
  memcpy(body, head + header_end, have);
  while (have < (size_t)content_length) {
    ssize_t n = read(fd, body + have, (size_t)content_length - have);
    if (n <= 0) {
      break;
    }
    have += (size_t)n;
  }
  body[have] = '\0';
  req->body = body;

  return req;
}

/* All request accessors are null-safe: accept returns NULL for dropped
 * or malformed connections and the stdlib checks method() == "". */

ORLANG_WEAK char *http_request_method(void *reqp) {
  return reqp != NULL ? ((orl_http_request_t *)reqp)->method : "";
}

ORLANG_WEAK char *http_request_path(void *reqp) {
  return reqp != NULL ? ((orl_http_request_t *)reqp)->path : "";
}

ORLANG_WEAK char *http_request_query(void *reqp) {
  return reqp != NULL ? ((orl_http_request_t *)reqp)->query : "";
}

ORLANG_WEAK char *http_request_body(void *reqp) {
  return reqp != NULL ? ((orl_http_request_t *)reqp)->body : "";
}

ORLANG_WEAK char *http_request_header(void *reqp, char *name) {
  orl_http_request_t *req = (orl_http_request_t *)reqp;
  if (req == NULL) {
    return "";
  }
  for (int64_t i = 0; i < req->header_count; i++) {
    if (strcasecmp(req->header_names[i], name) == 0) {
      return req->header_values[i];
    }
  }
  return "";
}

ORLANG_WEAK void http_response_status(void *reqp, int32_t code) {
  if (reqp != NULL) {
    ((orl_http_request_t *)reqp)->status = code;
  }
}

ORLANG_WEAK void http_response_header(void *reqp, char *name, char *value) {
  orl_http_request_t *req = (orl_http_request_t *)reqp;
  if (req == NULL) {
    return;
  }
  /* Replace an existing header of the same name */
  for (int64_t i = 0; i < req->res_header_count; i++) {
    if (strcasecmp(req->res_header_names[i], name) == 0) {
      req->res_header_values[i] = orl_http_strdup(value, strlen(value));
      return;
    }
  }
  if (req->res_header_count < ORL_HTTP_MAX_HEADERS) {
    req->res_header_names[req->res_header_count] = orl_http_strdup(name, strlen(name));
    req->res_header_values[req->res_header_count] = orl_http_strdup(value, strlen(value));
    req->res_header_count++;
  }
}

static const char *orl_http_status_text(int32_t code) {
  switch (code) {
  case 200: return "OK";
  case 201: return "Created";
  case 204: return "No Content";
  case 301: return "Moved Permanently";
  case 302: return "Found";
  case 304: return "Not Modified";
  case 400: return "Bad Request";
  case 401: return "Unauthorized";
  case 403: return "Forbidden";
  case 404: return "Not Found";
  case 405: return "Method Not Allowed";
  case 409: return "Conflict";
  case 500: return "Internal Server Error";
  case 503: return "Service Unavailable";
  default:  return "Status";
  }
}

static void orl_http_write_all(int fd, const char *data, size_t len) {
  size_t off = 0;
  while (off < len) {
    ssize_t n = write(fd, data + off, len - off);
    if (n <= 0) {
      return;
    }
    off += (size_t)n;
  }
}

ORLANG_WEAK void http_response_send(void *reqp, char *body) {
  orl_http_request_t *req = (orl_http_request_t *)reqp;
  if (req == NULL || req->sent) {
    return;
  }
  req->sent = 1;

  size_t body_len = body != NULL ? strlen(body) : 0;

  char head[256];
  int n = snprintf(head, sizeof(head), "HTTP/1.1 %d %s\r\n",
                   req->status, orl_http_status_text(req->status));
  orl_http_write_all(req->fd, head, (size_t)n);

  int have_content_type = 0;
  for (int64_t i = 0; i < req->res_header_count; i++) {
    if (strcasecmp(req->res_header_names[i], "Content-Type") == 0) {
      have_content_type = 1;
    }
    n = snprintf(head, sizeof(head), "%s: %s\r\n",
                 req->res_header_names[i], req->res_header_values[i]);
    orl_http_write_all(req->fd, head, (size_t)n);
  }
  if (!have_content_type) {
    const char *ct = "Content-Type: text/plain; charset=utf-8\r\n";
    orl_http_write_all(req->fd, ct, strlen(ct));
  }

  n = snprintf(head, sizeof(head), "Content-Length: %zu\r\nConnection: close\r\n\r\n", body_len);
  orl_http_write_all(req->fd, head, (size_t)n);
  orl_http_write_all(req->fd, body, body_len);

  close(req->fd);
  req->fd = -1;
}

#endif /* !_WIN32 */
