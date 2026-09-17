#!/bin/bash

set -euo pipefail

# Paths
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOSSH="${GOSSH:-$ROOT_DIR/bin/gossh}"

# Ports
SSH_PORT=2222
SERVER_PORT=7777
CLIENT_PORT=8888

# Test SSH credentials
SSH_USER="gossh-integration-test"
SSH_PASSWORD="you-talking-to-me?"

SERVER_PID=""
CLIENT_PID=""
SSH_PID=""

cleanup() {
    echo "[TEST] Cleaning up..."

    if [ -n "$CLIENT_PID" ]; then
        kill "$CLIENT_PID" 2>/dev/null || true
    fi

    if [ -n "$SERVER_PID" ]; then
        kill "$SERVER_PID" 2>/dev/null || true
    fi

    if [ -n "$SSH_PID" ]; then
        sudo kill "$SSH_PID" 2>/dev/null || true
    fi

    sudo userdel -r "$SSH_USER" 2>/dev/null || true

    rm -rf /tmp/gossh-test-ssh

    echo "[TEST] Cleanup complete."
}

trap cleanup EXIT

echo "[TEST] Starting gossh integration test"
echo "[TEST] Binary: $GOSSH"

# Check dependencies
if [ ! -x "$GOSSH" ]; then
    echo "[FAIL] gossh binary not found or not executable:"
    echo "       $GOSSH"
    echo "       Build it first with: make build"
    exit 1
fi

for command in ssh sshd sudo; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "[FAIL] Required command not found: $command"
        exit 1
    fi
done

# sshpass is used only for the temporary integration-test SSH server.
if ! command -v sshpass >/dev/null 2>&1; then
    echo "[TEST] Installing sshpass..."

    sudo apt-get update
    sudo apt-get install -y sshpass
fi

# Prepare temporary SSH server
SSH_DIR="/tmp/gossh-test-ssh"

rm -rf "$SSH_DIR"
mkdir -p "$SSH_DIR"

echo "[TEST] Creating temporary SSH user..."

if id "$SSH_USER" >/dev/null 2>&1; then
    sudo userdel -r "$SSH_USER" 2>/dev/null || true
fi

sudo useradd \
    --create-home \
    --shell /bin/bash \
    "$SSH_USER"

echo "$SSH_USER:$SSH_PASSWORD" | sudo chpasswd

# Generate host key if necessary.
if [ ! -f /etc/ssh/ssh_host_ed25519_key ]; then
    echo "[TEST] Generating SSH host key..."
    sudo ssh-keygen \
        -t ed25519 \
        -f /etc/ssh/ssh_host_ed25519_key \
        -N ""
fi

# Configure temporary sshd
SSHD_CONFIG="$SSH_DIR/sshd_config"
SSHD_PID_FILE="$SSH_DIR/sshd.pid"
SSHD_LOG="$SSH_DIR/sshd.log"

cat > "$SSHD_CONFIG" <<EOF
Port $SSH_PORT
ListenAddress 127.0.0.1

HostKey /etc/ssh/ssh_host_ed25519_key

PidFile $SSHD_PID_FILE

PasswordAuthentication yes
KbdInteractiveAuthentication no
PubkeyAuthentication no

UsePAM yes

PermitRootLogin no
AllowUsers $SSH_USER

X11Forwarding no
AllowTcpForwarding no
PermitTunnel no

StrictModes no
LogLevel ERROR
EOF

echo "[TEST] Starting temporary sshd on port $SSH_PORT..."

sudo /usr/sbin/sshd \
    -f "$SSHD_CONFIG" \
    -E "$SSHD_LOG"

if ! sudo pgrep -f "sshd.*$SSHD_CONFIG" >/dev/null 2>&1; then
    echo "[FAIL] sshd failed to start"
    cat "$SSHD_LOG" 2>/dev/null || true
    exit 1
fi

echo "[OK] sshd started"

# Start gossh server
echo "[TEST] Starting gossh server..."

"$GOSSH" server \
    --port "$SERVER_PORT" \
    --ssh "$SSH_PORT" \
    -vvv > /tmp/gossh-server.log 2>&1 &

SERVER_PID=$!

sleep 1

if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "[FAIL] gossh server failed to start"
    cat /tmp/gossh-server.log || true
    exit 1
fi

echo "[OK] gossh server started (PID=$SERVER_PID)"

# Start gossh client
echo "[TEST] Starting gossh client..."

"$GOSSH" client \
    --connect "ws://127.0.0.1:$SERVER_PORT" \
    --port "$CLIENT_PORT" \
    -vvv > /tmp/gossh-client.log 2>&1 &

CLIENT_PID=$!

sleep 2

if ! kill -0 "$CLIENT_PID" 2>/dev/null; then
    echo "[FAIL] gossh client failed to start"
    cat /tmp/gossh-client.log || true
    exit 1
fi

echo "[OK] gossh client started (PID=$CLIENT_PID)"

# Test local gossh port
echo "[TEST] Checking gossh client port..."

if ! (echo >/dev/tcp/127.0.0.1/$CLIENT_PORT) 2>/dev/null; then
    echo "[FAIL] gossh client is not listening on port $CLIENT_PORT"
    cat /tmp/gossh-client.log || true
    exit 1
fi

echo "[OK] gossh client is listening on port $CLIENT_PORT"

# Real SSH integration test
echo "[TEST] Connecting through gossh tunnel..."

OUTPUT=$(
    sshpass -p "$SSH_PASSWORD" \
        ssh \
        -p "$CLIENT_PORT" \
        -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o PreferredAuthentications=password \
        -o PubkeyAuthentication=no \
        -o ConnectTimeout=10 \
        "$SSH_USER@127.0.0.1" \
        "echo integration-test"
)

EXPECTED="integration-test"

if [ "$OUTPUT" != "$EXPECTED" ]; then
    echo "[FAIL] Unexpected SSH output"
    echo "Expected: $EXPECTED"
    echo "Actual:   $OUTPUT"

    echo ""
    echo "--- gossh server log ---"
    cat /tmp/gossh-server.log || true

    echo ""
    echo "--- gossh client log ---"
    cat /tmp/gossh-client.log || true

    echo ""
    echo "--- sshd log ---"
    cat "$SSHD_LOG" 2>/dev/null || true

    exit 1
fi

echo "[OK] SSH command executed successfully"
echo "[OK] Received: $OUTPUT"

# Test bidirectional data
echo "[TEST] Testing stdin -> remote -> stdout..."

OUTPUT=$(
    printf 'hello-from-client\n' |
        sshpass -p "$SSH_PASSWORD" \
        ssh \
        -p "$CLIENT_PORT" \
        -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o PreferredAuthentications=password \
        -o PubkeyAuthentication=no \
        -o ConnectTimeout=10 \
        "$SSH_USER@127.0.0.1" \
        "cat"
)

EXPECTED="hello-from-client"

if [ "$OUTPUT" != "$EXPECTED" ]; then
    echo "[FAIL] Bidirectional SSH test failed"
    echo "Expected: $EXPECTED"
    echo "Actual:   $OUTPUT"
    exit 1
fi

echo "[OK] Bidirectional SSH test passed"

echo ""
echo "[OK] GOSSH INTEGRATION TEST PASSED"

