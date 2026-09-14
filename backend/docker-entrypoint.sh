#!/bin/sh
# OpenRisk backend container entrypoint.
#
# The server fails fast at boot without an RS256 key pair (internal/config
# panics, pkg/auth panics). Keys are gitignored and generated per-install, so a
# fresh `docker compose up` had nothing to give it. This generates a pair into
# the secrets volume on first boot and reuses it afterwards — the volume is what
# keeps issued tokens valid across `docker compose down && up`.
#
# Anything already provided (RSA_PRIVATE_KEY_PATH / RSA_PRIVATE_KEY, or a key
# file mounted into the volume) is used as-is and never overwritten.
set -eu

KEY_DIR="${RSA_KEY_DIR:-/app/secrets}"
PRIV="${KEY_DIR}/private.pem"
PUB="${KEY_DIR}/public.pem"

if [ -z "${RSA_PRIVATE_KEY:-}" ] && [ -z "${RSA_PRIVATE_KEY_PATH:-}" ]; then
    mkdir -p "$KEY_DIR"
    if [ ! -s "$PRIV" ] || [ ! -s "$PUB" ]; then
        echo "entrypoint: no RS256 key pair in ${KEY_DIR} — generating one (2048-bit)"
        # Umask so the private key is never group/world readable, even briefly.
        ( umask 077 && openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$PRIV" 2>/dev/null )
        openssl rsa -in "$PRIV" -pubout -out "$PUB" 2>/dev/null
        chmod 600 "$PRIV"
        chmod 644 "$PUB"
        echo "entrypoint: key pair written to ${KEY_DIR} (persisted in the volume)"
    fi
    RSA_PRIVATE_KEY_PATH="$PRIV"
    RSA_PUBLIC_KEY_PATH="$PUB"
    export RSA_PRIVATE_KEY_PATH RSA_PUBLIC_KEY_PATH
fi

exec /usr/local/bin/openrisk "$@"
