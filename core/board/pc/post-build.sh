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
if [ "${MELKOR_PC_MINIMAL:-0}" = 1 ]; then
    sed -i '/^waits-for = seatd$/d' "$1/etc/dinit.d/boot"
    printf '%s\n' 'Melkor Linux PC. Use iwctl para configurar o Wi-Fi.' > "$1/etc/motd"
    rm -f "$1/etc/dinit.d/seatd" "$1/usr/libexec/melkor-desktop-session"
    for launcher in desktop terminal session-exec browser firefox brave-origin blanc; do
        rm -f "$1/usr/bin/melkor-$launcher"
    done
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
    install -D -m 0644 "$1/etc/xdg/melkor/mimeapps.list" "$1/usr/share/applications/mimeapps.list"
    install -D -m 0644 "$1/etc/xdg/melkor/audio-pulse.conf" \
        "$1/opt/melkor-desktop/usr/share/pipewire/pipewire-pulse.conf.d/90-melkor.conf"
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
    test "${MELKOR_PC_MINIMAL:-0}" != 1
    runtime=/opt/melkor-desktop
    tar -xzf "$MELKOR_PC_SESSION_LAYER" -C "$1$runtime"
    for binary in sbin/greetd sbin/unix_chkpwd bin/cage bin/noctalia-greeter bin/umbriel; do
        test -x "$1$runtime/usr/$binary"
    done
    # greetd reexecuta o próprio ELF. PAM executa unix_chkpwd pelo caminho de fábrica.
    # O Cage também executa o cliente Noctalia diretamente.
    for binary in sbin/greetd sbin/unix_chkpwd bin/cage bin/noctalia-greeter; do
        "$HOST_DIR/bin/patchelf" --set-interpreter "$runtime/lib/ld-musl-x86_64.so.1" \
            --set-rpath "$runtime/lib:$runtime/usr/lib" "$1$runtime/usr/$binary"
    done
    chmod 0700 "$1$runtime/usr/sbin/unix_chkpwd"
    install -d "$1/usr/sbin"
    ln -sf "$runtime/usr/sbin/unix_chkpwd" "$1/usr/sbin/unix_chkpwd"
    login="$(dirname "$0")/graphical-login"
    for file in etc/dinit.d/graphical-login etc/greetd/config.toml etc/pam.d/greetd etc/pam.d/greetd-greeter \
        usr/share/wayland-sessions/melkor-umbriel.desktop var/lib/noctalia-greeter/greeter.toml; do
        install -D -m 0644 "$login/$file" "$1/$file"
    done
    install -D -m 0755 "$login/usr/libexec/melkor-greeter" "$1/usr/libexec/melkor-greeter"
    sed -i 's/tty1 linux/tty2 linux/' "$1/etc/dinit.d/console-local"
    printf '%s\n' 'waits-for = graphical-login' >> "$1/etc/dinit.d/boot"
    printf '%s\n' 'Melkor Linux PC. Use Ctrl+Alt+F1 para voltar ao login gráfico.' > "$1/etc/motd"
fi
