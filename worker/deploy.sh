#!/usr/bin/env bash
# Deploys the website with the desktop app's files on it, so a Download button downloads at once instead of opening
# GitHub. release.sh runs it after publishing the release; run it again by hand if that step stopped.
#   Usage: worker/deploy.sh <version>      (e.g. 0.12.0; the GitHub release v<version> must exist)
# The files stay out of git (.gitignore); /download/windows, /download/linux and the rest are short, fixed links to
# them, written to public/_redirects for this version. Cloudflare serves a file up to 25 MiB; each is about 15 MB.
set -euo pipefail
cd "$(dirname "$0")"
V=${1:?usage: worker/deploy.sh <version>}
REPO=${REPO:-Shaitrish/37studio}

# Publishing the release starts .github/workflows/desktop.yml, which builds the files and attaches them (~15 min).
run=""
for _ in $(seq 60); do
  run=$(gh run list -R "$REPO" --workflow desktop.yml --event release --branch "v$V" --limit 1 --json databaseId -q '.[0].databaseId')
  [ -n "$run" ] && break
  sleep 5
done
[ -n "$run" ] || { echo "No desktop build started for v$V. Start it from GitHub Actions, then run worker/deploy.sh $V" >&2; exit 1; }
echo "Waiting for the desktop build (run $run) to attach its files to v$V…"
gh run watch -R "$REPO" "$run" --exit-status >/dev/null || { echo "The desktop build failed: see gh run view $run --log-failed. The website is not deployed." >&2; exit 1; }

rm -rf public/download
gh release download "v$V" -R "$REPO" -D public/download -p '37studio*'
need() { [ -e "public/download/$1" ] || { echo "v$V has no $1; the website is not deployed." >&2; exit 1; }; }
for f in "37studio-$V-windows-amd64-setup.exe" "37studio_${V}_amd64.deb"; do need "$f"; done
cat > public/_redirects <<EOF
/download/windows       /download/37studio-$V-windows-amd64-setup.exe 302
/download/windows-zip   /download/37studio-$V-windows-amd64.zip 302
/download/linux         /download/37studio_${V}_amd64.deb 302
/download/linux-arm     /download/37studio_${V}_arm64.deb 302
/download/linux-tar     /download/37studio-$V-linux-amd64.tar.gz 302
/download/linux-arm-tar /download/37studio-$V-linux-arm64.tar.gz 302
EOF
npx -y wrangler deploy
