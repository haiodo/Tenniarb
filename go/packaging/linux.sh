#!/usr/bin/env bash
# Stages the Linux tarball. Usage: linux.sh <binary> <stage-dir> <gowt-dir> <embed-js>
set -euo pipefail
bin=$1; stage=$2; gowt=$3; embed=$4
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
id=io.github.haiodo.Tenniarb
rm -rf "$stage"; mkdir -p "$stage"
cp "$bin" "$stage/tenniarb"
cp "$embed" "$stage/"
cp "$root/Tenniarb/Assets.xcassets/AppIcon.appiconset/Icon-v3.png" "$stage/tenniarb.png"
cp "$here/install.sh" "$stage/"
# Exec is rewritten by install.sh to the install location.
# Run from our module: gowt in the read-only module cache is not a module go run can build in.
(cd "$root/go" && go run github.com/haiodo/gowt/tooling/assoc -kind desktop -id $id -name Tenniarb -exe /opt/tenniarb/tenniarb \
	-mime application/x-tenniarb -icon tenniarb -wmclass Tenniarb -categories 'Office;Graphics;') >"$stage/$id.desktop"
(cd "$root/go" && go run github.com/haiodo/gowt/tooling/assoc -kind mime -mime application/x-tenniarb -name 'Tenniarb file' -ext tenn) >"$stage/tenniarb-mime.xml"
