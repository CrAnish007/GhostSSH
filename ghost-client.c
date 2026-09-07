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

void ghost_ws_handler(struct mg_connection *ws, int ev, void *ev_data) {
    ghost_session *session = (ghost_session *) ws->fn_data;

    if (ev == MG_EV_WS_OPEN) {
        struct mg_connection *tcp = ghost_session_peer(session, ws);

        MG_INFO(("WebSocket connection is successfully established"));
        if (session) session->upgrade_done = 1;

        /* flush whatever the SSH client sent while the tunnel was coming up */
        if (tcp && !tcp->is_closing && tcp->recv.len > 0) {
            mg_ws_send(ws, tcp->recv.buf, tcp->recv.len, WEBSOCKET_OP_BINARY);
            mg_iobuf_del(&tcp->recv, 0, tcp->recv.len);
        }
    } else if (ev == MG_EV_WS_MSG) {
        struct mg_ws_message* wm = (struct mg_ws_message *)ev_data;
        struct mg_connection* tcp = ghost_session_peer(session, ws);
        MG_DEBUG(("Data from websocket in CLIENT mode: %.*s", wm->data.len, wm->data.buf));

        if (wm && tcp) {
            mg_send(tcp, wm->data.buf, wm->data.len);
        }
    } else if (ev == MG_EV_CLOSE) {
        ghost_session_close(session, ws);
    }
}

void ghost_tls_handshake(struct mg_connection *tcp, ghost_session *session) {
    /* configure TLS options */
    char tls_host[MAX_LEN];
    char url[MAX_LEN];

    strip_https_for_tls_host(tls_host, sizeof(tls_host));

    struct mg_tls_opts tls_opts = {
        .name = mg_str(tls_host),
        .ca = mg_str(""),  // Use system CA certificates
        .cert = mg_str(""),
        .key = mg_str(""),
    };

    convert_to_wss(config.ws_url, url, sizeof(url));
    struct mg_connection *ws = mg_ws_connect(tcp->mgr,
            url,
            ghost_ws_handler, NULL, NULL);

    if (ws) {
        /* Initialize TLS with SNI */
        mg_tls_init(ws, &tls_opts);
        ghost_session_bind(session, ws, WS);
        MG_INFO(("WebSocket connection initiated with TLS/SNI"));
    } else {
        MG_INFO(("Failed to create WebSocket connection"));
        tcp->is_closing = 1;
    }
}
