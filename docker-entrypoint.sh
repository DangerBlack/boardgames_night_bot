#!/bin/sh
# Drop root before starting the bot. Volumes created by earlier images are
# owned by root, so fix ownership of the data directory first (idempotent),
# then re-exec as the unprivileged "bot" user.
set -e

DATA_DIR="${DB_PATH:-./archive}"

if [ "$(id -u)" = "0" ]; then
    mkdir -p "$DATA_DIR"
    chown -R bot:bot "$DATA_DIR"
    exec su-exec bot:bot "$@"
fi

exec "$@"
