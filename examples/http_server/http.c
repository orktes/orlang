#include <microhttpd.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* Forward declaration - implemented in orlang */
extern int handle_request(const char *method, const char *url, void *connection);

/* ── HTTP helpers ──────────────────────────────────────────────── */

int http_send(void *conn, int status, const char *content_type, const char *body) {
    struct MHD_Response *response = MHD_create_response_from_buffer(
        strlen(body), (void *)body, MHD_RESPMEM_MUST_COPY);
    if (!response)
        return 0;
    MHD_add_response_header(response, "Content-Type", content_type);
    MHD_add_response_header(response, "Access-Control-Allow-Origin", "*");
    enum MHD_Result ret = MHD_queue_response(
        (struct MHD_Connection *)conn, (unsigned int)status, response);
    MHD_destroy_response(response);
    return ret == MHD_YES ? 1 : 0;
}

static enum MHD_Result mhd_handler(
    void *cls,
    struct MHD_Connection *connection,
    const char *url,
    const char *method,
    const char *version,
    const char *upload_data,
    size_t *upload_data_size,
    void **con_cls)
{
    (void)cls; (void)version; (void)upload_data; (void)upload_data_size;
    if (*con_cls == NULL) {
        *con_cls = (void *)1;
        return MHD_YES;
    }
    return handle_request(method, url, (void *)connection) ? MHD_YES : MHD_NO;
}

void *http_listen(int port) {
    struct MHD_Daemon *d = MHD_start_daemon(
        MHD_USE_INTERNAL_POLLING_THREAD,
        (uint16_t)port,
        NULL, NULL,
        &mhd_handler, NULL,
        MHD_OPTION_END);
    if (!d) {
        fprintf(stderr, "Failed to start HTTP server on port %d\n", port);
        exit(1);
    }
    return d;
}

void http_close(void *daemon) {
    MHD_stop_daemon((struct MHD_Daemon *)daemon);
}

/* ── String helpers ────────────────────────────────────────────── */

int str_eq(const char *a, const char *b) {
    return strcmp(a, b) == 0 ? 1 : 0;
}

int str_starts_with(const char *s, const char *prefix) {
    return strncmp(s, prefix, strlen(prefix)) == 0 ? 1 : 0;
}

/* ── JSON formatting ───────────────────────────────────────────── */

static char json_buf[4096];

const char *json_ok(const char *message) {
    snprintf(json_buf, sizeof(json_buf),
             "{\"status\": \"ok\", \"message\": \"%s\"}", message);
    return json_buf;
}

const char *json_error(const char *message) {
    snprintf(json_buf, sizeof(json_buf),
             "{\"error\": \"%s\"}", message);
    return json_buf;
}

const char *json_int(const char *key, int value) {
    snprintf(json_buf, sizeof(json_buf),
             "{\"%s\": %d}", key, value);
    return json_buf;
}

const char *json_item(int id, const char *title, int done) {
    snprintf(json_buf, sizeof(json_buf),
             "{\"id\": %d, \"title\": \"%s\", \"done\": %s}",
             id, title, done ? "true" : "false");
    return json_buf;
}

/* ── Todo storage (simple fixed-size array) ────────────────────── */

#define MAX_TODOS 64

typedef struct {
    int    id;
    char   title[256];
    int    done;
    int    active;
} Todo;

static Todo todos[MAX_TODOS];
static int  next_id = 1;

int todo_add(const char *title) {
    for (int i = 0; i < MAX_TODOS; i++) {
        if (!todos[i].active) {
            todos[i].id = next_id++;
            strncpy(todos[i].title, title, 255);
            todos[i].title[255] = '\0';
            todos[i].done = 0;
            todos[i].active = 1;
            return todos[i].id;
        }
    }
    return -1;
}

int todo_toggle(int id) {
    for (int i = 0; i < MAX_TODOS; i++) {
        if (todos[i].active && todos[i].id == id) {
            todos[i].done = !todos[i].done;
            return 1;
        }
    }
    return 0;
}

int todo_delete(int id) {
    for (int i = 0; i < MAX_TODOS; i++) {
        if (todos[i].active && todos[i].id == id) {
            todos[i].active = 0;
            return 1;
        }
    }
    return 0;
}

const char *todo_get(int id) {
    for (int i = 0; i < MAX_TODOS; i++) {
        if (todos[i].active && todos[i].id == id) {
            return json_item(todos[i].id, todos[i].title, todos[i].done);
        }
    }
    return "";
}

/* Returns a JSON array of all active todos */
static char list_buf[16384];

const char *todo_list(void) {
    int offset = 0;
    offset += snprintf(list_buf + offset, sizeof(list_buf) - offset, "[");
    int first = 1;
    for (int i = 0; i < MAX_TODOS; i++) {
        if (!todos[i].active) continue;
        if (!first)
            offset += snprintf(list_buf + offset, sizeof(list_buf) - offset, ", ");
        offset += snprintf(list_buf + offset, sizeof(list_buf) - offset,
            "{\"id\": %d, \"title\": \"%s\", \"done\": %s}",
            todos[i].id, todos[i].title, todos[i].done ? "true" : "false");
        first = 0;
    }
    snprintf(list_buf + offset, sizeof(list_buf) - offset, "]");
    return list_buf;
}

int todo_count(void) {
    int count = 0;
    for (int i = 0; i < MAX_TODOS; i++) {
        if (todos[i].active) count++;
    }
    return count;
}

/* ── URL path parsing ──────────────────────────────────────────── */

/* Extract integer from URL path after a prefix, e.g. "/todos/42" -> 42 */
int url_param_int(const char *url, const char *prefix) {
    size_t plen = strlen(prefix);
    if (strncmp(url, prefix, plen) != 0)
        return -1;
    return atoi(url + plen);
}

/* Extract string suffix after a prefix, e.g. "/todos/add/buy milk" -> "buy milk" */
const char *url_suffix(const char *url, const char *prefix) {
    size_t plen = strlen(prefix);
    if (strncmp(url, prefix, plen) != 0)
        return "";
    return url + plen;
}
