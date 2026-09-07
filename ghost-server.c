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

#include "mongoose.h"
#include "ghost.h"

void ghost_http_handler(struct mg_connection *http, int ev, void *ev_data) {
    ghost_session *session = (ghost_session *) http->fn_data;

    if (ev == MG_EV_HTTP_MSG) {
        struct mg_http_message *hm = (struct mg_http_message *) ev_data;
        if (mg_match(hm->uri, mg_str("/ws"), NULL)) {
            mg_ws_upgrade(http, hm, NULL);   // Upgrade HTTP to WebSocket
            MG_INFO(("Upgraded to WebSocket"));
        }
    } else if (ev == MG_EV_WS_OPEN) {
        char url[MAX_LEN];
        struct mg_connection *tcp;

        MG_INFO(("WebSocket connection is successfully established"));

        /* every tunnelled client gets its own session and its own sshd link */
        session = ghost_session_create();
        if (session == NULL) {
            MG_ERROR(("Cannot allocate session, dropping connection"));
            http->is_closing = 1;
            return;
        }

        ghost_session_bind(session, http, WS);

        create_local_url(TCP, config.sshd_port, url, sizeof(url));
        tcp = mg_connect(http->mgr, url, ghost_tcp_handler, NULL);

        if (tcp) {
            ghost_session_bind(session, tcp, TCP);
        } else {
            MG_ERROR(("Cannot connect to sshd on %s", url));
            http->is_closing = 1;
        }
    } else if (ev == MG_EV_WS_MSG) {
        struct mg_ws_message *wm = (struct mg_ws_message *) ev_data;
        struct mg_connection *tcp = ghost_session_peer(session, http);

        MG_DEBUG(("Data from websocket in SERVER mode: %.*s", wm->data.len, wm->data.buf));
        if (tcp) {
            mg_send(tcp, wm->data.buf, wm->data.len);
        }
    } else if (ev == MG_EV_CLOSE) {
        ghost_session_close(session, http);

        if (s_signo) {
            MG_INFO(("Shutting down server..."));
            return;
        }
        
        MG_INFO(("Client disconnected"));
    }
}
