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
if [ -n "${MELKOR_PC_DESKTOP_LAYER:-}" ]; then
    install -d "$1/opt/melkor-desktop"
    tar -xzf "$MELKOR_PC_DESKTOP_LAYER" -C "$1/opt/melkor-desktop"
    install -D -m 0644 "$1/opt/melkor-desktop/usr/lib/firefox-esr/browser/chrome/icons/default/default32.png" \
        "$1/usr/share/melkor/firefox.png"
    install -D -m 0644 "$1/etc/xdg/melkor/firefox-policies.json" \
        "$1/opt/melkor-desktop/usr/lib/firefox-esr/distribution/policies.json"
    cat "$1/etc/xdg/weston/melkor-firefox.ini" >> "$1/etc/xdg/weston/weston.ini"
    install -D -m 0644 "$1/etc/xdg/melkor/firefox.desktop" "$1/usr/share/applications/melkor-firefox.desktop"
fi
if [ -n "${MELKOR_PC_CHROMIUM_LAYER:-}" ]; then
    install -d "$1/opt/melkor-chromium"
    tar -xzf "$MELKOR_PC_CHROMIUM_LAYER" -C "$1/opt/melkor-chromium"
    install -D -m 0644 "$1/opt/melkor-chromium/opt/brave.com/brave-origin/product_logo_32.png" \
        "$1/usr/share/melkor/brave-origin.png"
    install -D -m 0644 "$1/opt/melkor-chromium/opt/blanc/usr/share/icons/hicolor/1024x1024/apps/blanc.png" \
        "$1/usr/share/melkor/blanc.png"
    for browser in brave-origin blanc; do
        install -D -m 0644 "$1/etc/xdg/melkor/$browser.desktop" "$1/usr/share/applications/melkor-$browser.desktop"
    done
    cat "$1/etc/xdg/weston/melkor-chromium.ini" >> "$1/etc/xdg/weston/weston.ini"
fi
if [ -n "${MELKOR_PC_SESSION_LAYER:-}" ]; then
    test -n "${MELKOR_PC_DESKTOP_LAYER:-}"
    tar -xzf "$MELKOR_PC_SESSION_LAYER" -C "$1/opt/melkor-desktop"
fi
