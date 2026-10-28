#!/bin/sh
# Per-user install: ./install.sh [prefix], default ~/.local/opt/tenniarb. Needs GTK 3 at run time.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:-$HOME/.local/opt/tenniarb}
share=${XDG_DATA_HOME:-$HOME/.local/share}
id=io.github.haiodo.Tenniarb
mkdir -p "$dest" "$share/applications" "$share/mime/packages" "$share/icons/hicolor/512x512/apps"
cp "$here/tenniarb" "$here/tenniarb-embed.min.js" "$dest/"
sed "s|^Exec=.*|Exec=$dest/tenniarb %U|" "$here/$id.desktop" >"$share/applications/$id.desktop"
cp "$here/tenniarb-mime.xml" "$share/mime/packages/tenniarb.xml"
cp "$here/tenniarb.png" "$share/icons/hicolor/512x512/apps/tenniarb.png"
update-mime-database "$share/mime" 2>/dev/null || true
update-desktop-database "$share/applications" 2>/dev/null || true
xdg-mime default "$id.desktop" x-scheme-handler/tenniarb 2>/dev/null || true
echo "installed to $dest"
