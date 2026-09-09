#!/bin/bash

set -e

REPO="ankushT369/gossh"
BINARY="gossh"
INSTALL_DIR="$HOME/.local/bin"
URL="https://github.com/$REPO/releases/latest/download/$BINARY"
TMP_FILE="$(mktemp)"

cleanup() {
    rm -f "$TMP_FILE"
}

trap cleanup EXIT

mkdir -p "$INSTALL_DIR"

echo "[-] Installing gossh"
echo

# Download in background
curl -fsSL "$URL" -o "$TMP_FILE" &
PID=$!

# Braille spinner
while kill -0 "$PID" 2>/dev/null; do
    for frame in "⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏"; do
        printf "\r[-] Downloading gossh %s" "$frame"
        sleep 0.08

        if ! kill -0 "$PID" 2>/dev/null; then
            break
        fi
    done
done

# Check whether curl succeeded
if ! wait "$PID"; then
    printf "\r[✗] Downloading gossh\n"
    echo "Failed to download gossh."
    exit 1
fi

printf "\r[✓] Downloaded gossh  \n"

# Install
mv "$TMP_FILE" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

echo "[✓] Installed gossh to $INSTALL_DIR/$BINARY"

# Check PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo
    echo "Add this to your shell configuration:"
    echo 'export PATH="$HOME/.local/bin:$PATH"'
fi

echo
echo "[✓] run: gossh version"

# #!/bin/bash
#
# set -e
#
# REPO="ankushT369/gossh"
# BINARY="gossh"
# INSTALL_DIR="$HOME/.local/bin"
#
# OS="$(uname -s)"
# ARCH="$(uname -m)"
#
# if [ "$OS" != "Linux" ]; then
#     echo "Error: unsupported OS: $OS"
#     exit 1
# fi
#
# case "$ARCH" in
#     x86_64)
#         ;;
#     aarch64|arm64)
#         ;;
#     *)
#         echo "Error: unsupported architecture: $ARCH"
#         exit 1
#         ;;
# esac
#
# mkdir -p "$INSTALL_DIR"
#
# echo "Downloading gossh..."
#
# curl -fsSL \
#     "https://github.com/$REPO/releases/latest/download/$BINARY" \
#     -o "$INSTALL_DIR/$BINARY"
#
# chmod +x "$INSTALL_DIR/$BINARY"
#
# echo "gossh installed to $INSTALL_DIR/$BINARY"
#
# if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
#     echo
#     echo "Add this to your shell configuration:"
#     echo 'export PATH="$HOME/.local/bin:$PATH"'
# fi
