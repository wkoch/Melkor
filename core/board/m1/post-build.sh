#!/bin/sh
set -eu

install -D -m 0644 "$BINARIES_DIR/bzImage" "$1/boot/bzImage"
install -d -m 0755 "$1/persist"
if [ -n "${MELKOR_RAUC_KEYRING:-}" ]; then
    awk '/^-----BEGIN CERTIFICATE-----$/,/^-----END CERTIFICATE-----$/' \
        "$MELKOR_RAUC_KEYRING" > "$1/etc/rauc/keyring.pem"
    "$HOST_DIR/bin/openssl" crl2pkcs7 -nocrl -certfile "$1/etc/rauc/keyring.pem" -out /dev/null
    grep -Fxq -- '-----BEGIN CERTIFICATE-----' "$1/etc/rauc/keyring.pem"
fi
if [ -n "${MELKOR_LAB_ROOT_HASH_FILE:-}" ]; then
    hash=$(cat "$MELKOR_LAB_ROOT_HASH_FILE")
    valid_hash=$(printf '%s\n' "$hash" | LC_ALL=C grep -Ex '\$6\$[A-Za-z0-9./]{1,16}\$[A-Za-z0-9./]{86}')
    test "$hash" = "$valid_hash"
    grep -q '^root:' "$1/etc/shadow"
    sed -i "s|^root:[^:]*:|root:$hash:|" "$1/etc/shadow"
fi
