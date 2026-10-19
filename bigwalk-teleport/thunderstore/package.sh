#!/usr/bin/env bash
# Builds the Thunderstore / Gale package from a BepInEx build of the mod.
#   ./package.sh path/to/BepInEx/BigWalkTeleport.dll [output.zip]
# The zip holds manifest.json, icon.png, README.md and plugins/BigWalkTeleport.dll,
# which Gale installs into BepInEx/plugins/<author>-BigWalkTeleport/.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
dll="$1"
out="$(realpath -m "${2:-$here/../release/BigWalkTeleport-Gale.zip}")"

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/plugins"
cp "$here/manifest.json" "$here/icon.png" "$here/README.md" "$stage/"
cp "$dll" "$stage/plugins/BigWalkTeleport.dll"

rm -f "$out"
(cd "$stage" && zip -qr -X "$out" manifest.json icon.png README.md plugins)
echo "Wrote $out"
