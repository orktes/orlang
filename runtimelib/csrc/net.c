/*
 * Orlang runtime: TCP socket primitives for std/net.or.
 *
 * Connections are plain file descriptors (int32 in orlang). All IO goes
 * through the green-thread runtime's yielding wrappers, so blocking on
 * one connection lets other tasks run.
 */

#include <netdb.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>

#ifndef ORLANG_WEAK
#define ORLANG_WEAK __attribute__((weak))
#endif

void *GC_malloc(size_t size);
extern int orl_io_accept(int fd);
extern ssize_t orl_io_read(int fd, void *buf, size_t n);
extern ssize_t orl_io_write(int fd, const void *buf, size_t n);

ORLANG_WEAK int32_t tcp_listen(int32_t port) {
  signal(SIGPIPE, SIG_IGN);

  int fd = socket(AF_INET, SOCK_STREAM, 0);
  if (fd < 0) {
    return -1;
  }
  int one = 1;
  setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));

  struct sockaddr_in addr;
  memset(&addr, 0, sizeof(addr));
  addr.sin_family = AF_INET;
  addr.sin_addr.s_addr = htonl(INADDR_ANY);
  addr.sin_port = htons((uint16_t)port);

  if (bind(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0 || listen(fd, 16) != 0) {
    close(fd);
    return -1;
  }
  return fd;
}

ORLANG_WEAK int32_t tcp_accept(int32_t fd) {
  return orl_io_accept(fd);
}

ORLANG_WEAK int32_t tcp_connect(char *host, int32_t port) {
  signal(SIGPIPE, SIG_IGN);

  char portstr[16];
  snprintf(portstr, sizeof(portstr), "%d", port);

  struct addrinfo hints;
  memset(&hints, 0, sizeof(hints));
  hints.ai_family = AF_INET;
  hints.ai_socktype = SOCK_STREAM;

  struct addrinfo *res = NULL;
  if (getaddrinfo(host, portstr, &hints, &res) != 0 || res == NULL) {
    return -1;
  }

  int fd = socket(res->ai_family, res->ai_socktype, res->ai_protocol);
  if (fd < 0) {
    freeaddrinfo(res);
    return -1;
  }
  if (connect(fd, res->ai_addr, res->ai_addrlen) != 0) {
    close(fd);
    freeaddrinfo(res);
    return -1;
  }
  freeaddrinfo(res);
  return fd;
}

/* Reads up to max bytes; returns a GC string ("" on EOF or error). */
ORLANG_WEAK char *tcp_read(int32_t fd, int32_t max) {
  if (max <= 0) {
    max = 4096;
  }
  char *buf = (char *)GC_malloc((size_t)max + 1);
  ssize_t n = orl_io_read(fd, buf, (size_t)max);
  if (n <= 0) {
    return "";
  }
  buf[n] = '\0';
  return buf;
}

/* Writes the whole string; returns bytes written or -1. */
ORLANG_WEAK int32_t tcp_write(int32_t fd, char *data) {
  size_t len = strlen(data);
  size_t off = 0;
  while (off < len) {
    ssize_t n = orl_io_write(fd, data + off, len - off);
    if (n <= 0) {
      return -1;
    }
    off += (size_t)n;
  }
  return (int32_t)len;
}

ORLANG_WEAK void tcp_close(int32_t fd) {
  if (fd >= 0) {
    close(fd);
  }
}
