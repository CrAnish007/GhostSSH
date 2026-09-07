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
#include "ghost.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* session */

ghost_session *ghost_session_create(void) {
    return (ghost_session *) calloc(1, sizeof(ghost_session));
}

/* Attach one endpoint to a session and take a reference on its behalf. */
void ghost_session_bind(ghost_session *session, struct mg_connection *c, int role) {
    if (session == NULL || c == NULL) return;

    if (role == WS)
        session->ws = c;
    else
        session->tcp = c;

    c->fn_data = session;
    session->refs++;
}

/* The other end of the tunnel for this connection, or NULL if it is gone. */
struct mg_connection *ghost_session_peer(ghost_session *session,
                                         struct mg_connection *c) {
    if (session == NULL) return NULL;
    if (c == session->tcp) return session->ws;
    if (c == session->ws) return session->tcp;

    return NULL;
}

/*
 * Called from MG_EV_CLOSE. Drains the peer so a half-open tunnel cannot
 * linger, then drops this endpoint's reference and frees the session once both
 * endpoints are gone.
 */
void ghost_session_close(ghost_session *session, struct mg_connection *c) {
    struct mg_connection *peer;

    if (session == NULL || c == NULL) return;

    peer = ghost_session_peer(session, c);
    if (peer != NULL) peer->is_draining = 1;

    if (c == session->tcp) session->tcp = NULL;
    if (c == session->ws) session->ws = NULL;
    c->fn_data = NULL;

    if (--session->refs <= 0) free(session);
}

/* util */

void convert_to_wss(const char *url, char *out, size_t len) {
    memset(out, '\0', len);
    const char *p = strstr(url, "://");
    if (p != NULL) {
        p += 3;
    } else {
        p = url;
    }

    size_t plen = strlen(p);
    int has_ws = (plen >= 3 && strcmp(p + plen - 3, "/ws") == 0);

    if (has_ws) {
        snprintf(out, len, "wss://%s", p);
    } else {
        snprintf(out, len, "wss://%s/ws", p);
    }
}

void create_local_url(int protocol, int port, char *out, size_t len) {
    memset(out, '\0', len);
    snprintf(out, len, "%s://localhost:%d", proto_str[protocol], port);
}

void strip_https_for_tls_host(char *out, size_t len) {
    memset(out, 0, len);

    const char *p = config.ws_url;

    if (strncmp(p, "https://", 8) == 0) {
        p += 8;  // skip "https://"
    }

    snprintf(out, len, "%s", p);
}
