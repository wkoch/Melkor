#!/bin/sh
set -eu

config="$1/etc/melkor-update"
revision=${MELKOR_CORE_REVISION:-0}
if ! printf '%s\n' "$revision" | LC_ALL=C grep -Eq '^(0|[1-9][0-9]{0,9})$' ||
    [ "$revision" -gt 4294967295 ]; then
    printf '%s\n' 'MELKOR_CORE_REVISION deve ser um inteiro de 0 a 4294967295.' >&2
    exit 1
fi
if [ "$revision" -ne 0 ] &&
    { [ -z "${MELKOR_RAUC_KEYRING:-}" ] || [ -z "${MELKOR_CHANNEL_PUBLIC_KEY:-}" ]; }; then
    printf '%s\n' 'Uma revisão publicada exige o chaveiro RAUC e a chave pública do canal.' >&2
    exit 1
fi
printf '%s\n' "$revision" > "$config/revision"
if [ -n "${MELKOR_CHANNEL_PUBLIC_KEY:-}" ]; then
    "$HOST_DIR/bin/openssl" pkey -pubin -in "$MELKOR_CHANNEL_PUBLIC_KEY" -text_pub -noout |
        LC_ALL=C grep -Fxq 'ED25519 Public-Key:'
    "$HOST_DIR/bin/openssl" pkey -pubin -in "$MELKOR_CHANNEL_PUBLIC_KEY" -pubout -out "$config/channel.pem"
else
    rm -f "$config/channel.pem"
fi
