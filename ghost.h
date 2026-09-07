/*
 * Copyright 2024-2026 Ankush Mondal
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
#ifndef GHOST_H
#define GHOST_H

#include "mongoose.h"

#include <signal.h>

/* default ports */
#define SSH_PORT 22

#define HTTP_PORT 7777
#define TCP_PORT 8888

/* protocol */
#define TCP 0
#define UDP 1
#define HTTP 2
#define WS 3
#define LIMIT 4

/* mode */
#define SERVER 0
#define CLIENT 1

/* length */
#define MAX_LEN 1024

typedef struct ghost_config {
    /* server side config */
    int http_server_port;
    int sshd_port;


    /* client side config */
    int tcp_server_port;
    char ws_url[MAX_LEN];
} ghost_config;

/*
 * Per-connection state. A tunnelled session always pairs one TCP endpoint
 * with one WebSocket endpoint, so both endpoints keep a pointer to the same
 * ghost_session in their fn_data. Nothing about a session is global, which
 * lets several clients be tunnelled at the same time without one client's
 * progress (for example, its WebSocket coming up) affecting another's.
 *
 * refs counts the endpoints still alive; the session is freed once the last
 * one reports MG_EV_CLOSE.
 */
typedef struct ghost_session {
    struct mg_connection *tcp;  /* local listener (client mode) or sshd (server mode) */
    struct mg_connection *ws;   /* WebSocket peer */
    int upgrade_done;           /* set once this session's WebSocket can take payload */
    int refs;
} ghost_session;

extern const char* proto_str[LIMIT];
extern volatile sig_atomic_t s_signo;

extern ghost_config config; /* startup configuration, immutable once parsed */

/* API */
void ghost_http_handler(struct mg_connection *http, int ev, void *ev_data);
void ghost_ws_handler(struct mg_connection *ws, int ev, void *ev_data);
void ghost_tcp_handler(struct mg_connection *tcp, int ev, void *ev_data);
void ghost_tls_handshake(struct mg_connection *tcp, ghost_session *session);

/* session */
ghost_session *ghost_session_create(void);
void ghost_session_bind(ghost_session *session, struct mg_connection *c, int role);
struct mg_connection *ghost_session_peer(ghost_session *session,
                                         struct mg_connection *c);
void ghost_session_close(ghost_session *session, struct mg_connection *c);

/* util */
void convert_to_wss(const char *url, char *out, size_t len);
void create_local_url(int protocol, int port, char *out, size_t len);
void strip_https_for_tls_host(char *out, size_t len);

#endif // GHOST_H
