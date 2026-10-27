#!/usr/bin/env bash
# Builds Tenniarb.app from the gowt bundle script, then aligns the plist with the Swift app's Info.plist
# so Finder associations match. Usage: macos.sh <binary> <out-dir> <gowt-dir> <short-version> <build> <embed-js>
set -euo pipefail
bin=$1; out=$2; gowt=$3; ver=$4; build=$5; embed=$6
root=$(cd "$(dirname "$0")/../.." && pwd)
# The source .icns carries a resource fork and Finder info, which codesign refuses; cat drops them.
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
icon=$tmp/TennIco.icns
cat "$root/Tenniarb/TennIco.icns" >"$icon"
DOC_EXTS=tenn DOC_NAME="Tenniarb file" DOC_ICON=$icon DOC_UTI=tenniarb.tenn \
	bash "$gowt/tooling/darwin/mkapp.sh" "$bin" "$out" Tenniarb >/dev/null # the module cache drops the exec bit
app=$out/Tenniarb.app
plist=$app/Contents/Info.plist
pb() { /usr/libexec/PlistBuddy -c "$1" "$plist"; }
pb "Set :CFBundleIdentifier andrey.sobolev.Tenniarb"
pb "Set :CFBundleShortVersionString $ver"
pb "Set :CFBundleVersion $build"
pb "Add :CFBundleDisplayName string Tenniarb"
pb "Add :CFBundleIconFile string TennIco.icns"
pb "Add :LSApplicationCategoryType string public.app-category.productivity"
pb "Add :NSHumanReadableCopyright string 'Copyright © 2026 Andrey Sobolev'"
# Swift declares the type as public.text, mime text/plain, icon on the UTI.
pb "Set :UTExportedTypeDeclarations:0:UTTypeConformsTo:0 public.text"
pb "Add :UTExportedTypeDeclarations:0:UTTypeIconFile string TennIco.icns"
pb "Add :UTExportedTypeDeclarations:0:UTTypeTagSpecification:public.mime-type array"
pb "Add :UTExportedTypeDeclarations:0:UTTypeTagSpecification:public.mime-type: string text/plain"
pb "Add :CFBundleDocumentTypes:0:CFBundleTypeMIMETypes array"
pb "Add :CFBundleDocumentTypes:0:CFBundleTypeMIMETypes: string text/plain"
plutil -lint "$plist" >/dev/null
mkdir -p "$app/Contents/Resources"
cp "$embed" "$app/Contents/Resources/"
# Ad hoc again: the plist and Resources changed after mkapp.sh signed. No Developer ID, no notarization.
codesign --force --sign - "$app" >/dev/null 2>&1
echo "$app"
