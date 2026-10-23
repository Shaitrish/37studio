#!/usr/bin/env bash
# Publish a release to everyone: build and push the Docker image, bundle the launcher, update version.json, deploy the
# Worker on studio.shaitrish.com. Bump `Version` in internal/update/update.go first.   Usage: ./release.sh "What changed"
set -euo pipefail
cd "$(dirname "$0")"
IMAGE=${IMAGE:-ghcr.io/shaitrish/37studio}
V=$(sed -n 's/^const Version = "\(.*\)"/\1/p' internal/update/update.go)
# Both Intel PCs and Apple Silicon Macs: arm64 is built under QEMU emulation and needs a docker-container builder.
[ -e /proc/sys/fs/binfmt_misc/qemu-aarch64 ] || docker run --privileged --rm tonistiigi/binfmt --install arm64
docker buildx inspect studio37 >/dev/null 2>&1 || docker buildx create --name studio37 --driver docker-container
docker buildx build --builder studio37 --platform linux/amd64,linux/arm64 -t "$IMAGE:$V" -t "$IMAGE:latest" --push .
rm -f worker/public/37studio.zip
python3 -c "import shutil; shutil.make_archive('worker/public/37studio', 'zip', 'dist')"
python3 - "$V" "${1:-}" <<'PY'
import json, sys
json.dump({"version": sys.argv[1], "notes": sys.argv[2], "url": "https://studio.shaitrish.com/"}, open("worker/public/version.json", "w"))
PY
(cd worker && npx -y wrangler deploy)
git tag -a "v$V" -m "37Studio $V${1:+: $1}" && git push origin "v$V"
echo "Released 37Studio $V as $IMAGE:$V"
