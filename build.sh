#!/usr/bin/env bash
# Builds rayo into bin/.   build.sh          the Go tool only
#                          build.sh --agent  also the device agent (needs a JDK and the Android SDK)
#                          build.sh --all    for Windows, Linux and macOS
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if [ "${1:-}" = --agent ]; then
  sdk="${ANDROID_HOME:-${LOCALAPPDATA:-$HOME}/Android/Sdk}"
  command -v cygpath >/dev/null && sdk="$(cygpath -m "$sdk")"
  platform="$(ls -d "$sdk"/platforms/android-* | sort -V | tail -n 1)"
  tools="$(ls -d "$sdk"/build-tools/* | grep -v rc | sort -V | tail -n 1)"
  rm -rf internal/agent/out && mkdir -p internal/agent/out
  javac -nowarn --release 8 -cp "$platform/android.jar" -d internal/agent/out internal/agent/java/rayo/*.java 2>&1 | grep -v '^warning\|^Note' || true
  [ -f internal/agent/out/rayo/Agent.class ] || { echo "error: the agent didn't compile" >&2; exit 1; }
  java -cp "$tools/lib/d8.jar" com.android.tools.r8.D8 --release --min-api 26 --lib "$platform/android.jar" \
    --output internal/agent/agent.jar internal/agent/out/rayo/*.class
  rm -rf internal/agent/out
fi

export CGO_ENABLED=0
flags=(-trimpath -ldflags "-s -w")
mkdir -p bin
if [ "${1:-}" = --all ]; then
  for target in windows/amd64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
    os="${target%/*}" arch="${target#*/}" ext=""
    [ "$os" = windows ] && ext=.exe
    GOOS=$os GOARCH=$arch go build "${flags[@]}" -o "bin/rayo-$os-$arch$ext" ./cmd/rayo
  done
else
  go build "${flags[@]}" -o "bin/rayo$(go env GOEXE)" ./cmd/rayo
fi
ls -l bin
