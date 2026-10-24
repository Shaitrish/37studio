# Builds for Intel/AMD PCs (amd64) and Apple Silicon Macs (arm64); release.sh pushes both under one tag.

# ---- web UI (plain files, so built once on the build machine)
FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- API (pure Go SQLite, so no cgo: cross-compiled natively instead of emulated)
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS api
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /37studio ./cmd/37studio

# ---- runtime: ffmpeg + Chrome libs for HyperFrames rendering + the Claude Code CLI
FROM node:22-bookworm AS base
ARG HF_VERSION=0.8.186
RUN apt-get update && apt-get install -y --no-install-recommends \
      ffmpeg python3 python3-venv ca-certificates fonts-noto-core fonts-noto-color-emoji \
      libnss3 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libxkbcommon0 libxcomposite1 \
      libxdamage1 libxfixes3 libxrandr2 libgbm1 libasound2 libpango-1.0-0 libcairo2 libxshmfence1 \
    && rm -rf /var/lib/apt/lists/*
# Free local transcription for `37studio transcribe` (faster-whisper; models download to ~/.cache on first use), and
# yt-dlp for importing videos from YouTube and Google Drive links (unpinned: YouTube changes often, each release takes the newest).
RUN python3 -m venv /opt/asr && /opt/asr/bin/pip install --no-cache-dir faster-whisper==1.2.1 "yt-dlp[default,curl-cffi]"
ENV STUDIO37_ASR_PYTHON=/opt/asr/bin/python
# The agent runtimes are pinned too: they are the most privileged processes here, and a pin makes builds repeatable.
# To bump: try the new versions with `docker compose up --build`, run a project and the Assistant, then change these.
ARG CLAUDE_CODE_VERSION=2.1.309
ARG OPENCODE_VERSION=1.19.4
ARG CODEX_VERSION=0.164.0
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION} opencode-ai@${OPENCODE_VERSION} @openai/codex@${CODEX_VERSION} hyperframes@${HF_VERSION}
COPY --from=api /37studio /usr/local/bin/37studio
COPY --from=web /web/dist /app/web
RUN mkdir -p /data /home/node/.claude /opt/browser && chown -R node:node /data /home/node/.claude /opt/browser
# Claude Code refuses bypass mode as root, so the agent runs as the unprivileged node user.
USER node
ENV HF_VERSION=${HF_VERSION}

# Chrome for Testing, HyperFrames' own browser, has no linux-arm64 build, and Debian's chromium crashes at startup there.
# So arm64 gets Playwright's pinned arm64 headless shell, the same as HyperFrames' Dockerfile.render.
FROM base AS browser-amd64
RUN npx -y hyperframes@${HF_VERSION} browser ensure

FROM base AS browser-arm64
ARG PLAYWRIGHT_VERSION=1.61.1
RUN PLAYWRIGHT_BROWSERS_PATH=/opt/browser npx -y playwright-core@${PLAYWRIGHT_VERSION} install chromium-headless-shell \
 && ln -s "$(find /opt/browser -name headless_shell -type f | head -1)" /opt/browser/headless_shell \
 && /opt/browser/headless_shell --version
ENV HYPERFRAMES_BROWSER_PATH=/opt/browser/headless_shell

FROM browser-${TARGETARCH}
RUN npx -y hyperframes@${HF_VERSION} skills update || echo "skills install skipped; hyperframes init retries per project"
ENV STUDIO37_DATA=/data STUDIO37_WEB=/app/web STUDIO37_ADDR=:8080
WORKDIR /app
EXPOSE 8080 3100-3149
CMD ["37studio"]
