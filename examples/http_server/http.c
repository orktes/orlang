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
