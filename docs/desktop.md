# Desktop app (preview)

37Studio as a normal app in its own window, without Docker. It is the same app as the Docker version: the same
brands, projects, agent and publishing, served to a native window instead of a browser tab.

**Status: preview, Linux only, built from source.** There is no download yet. Windows and Mac builds, installers
(.exe, .dmg, .deb, AppImage) and automatic setup of the video tools come later. Until then, the
[ready-made Docker app](../README.md#on-your-computer-the-ready-made-app) is the way to run 37Studio for everyday work.

## What you need

- Ubuntu 24.04 or newer, Debian 13 or Fedora 40, or another Linux with GTK 4 and WebKitGTK 6.0
- [Go 1.26](https://go.dev/dl/) or newer and [Node.js 22](https://nodejs.org) or newer
- The tools the agent uses, installed on this computer as when [running from source](../CONTRIBUTING.md#running-it-from-source):
  [Claude Code](https://docs.claude.com/en/docs/claude-code) signed in (or a key in Settings), `ffmpeg`, and
  `python3` with `venv` for transcription and YouTube imports

## Install

1. Install the GTK and WebKit headers once (this asks for your password):

   ```bash
   # Ubuntu and Debian
   sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev pkg-config
   # Fedora
   sudo dnf install gtk4-devel webkitgtk6.0-devel pkgconf-pkg-config
   ```

2. Get the code and install the app into your app menu (no password needed; it installs for your user only):

   ```bash
   git clone https://github.com/Shaitrish/37studio.git && cd 37studio
   ./desktop/install-linux.sh
   ```

   The first run takes a few minutes: it builds the interface and the app.

3. Press the Super (Windows) key, type **37Studio** and open it. To keep it in the dock, right-click its icon there
   and choose **Pin to Dash**.

Then follow [First steps](../README.md#first-steps): add your Claude key in Settings → Keys and add a brand.

## Update

```bash
cd 37studio
git pull
./desktop/install-linux.sh
```

## Uninstall

```bash
./desktop/install-linux.sh --uninstall
```

This removes the app and its menu entry. Your brands, projects and videos stay in `~/.local/share/37studio`; delete
that folder too to remove everything.

## Where your work is kept

Everything lives in `~/.local/share/37studio`: the database, `secret.key` (which decrypts your keys), brand files,
footage and videos. Back up that folder, or use Settings → Backup.

**Using your Docker data.** The desktop app starts empty. To open the work you made in the Docker version, stop
Docker first, so two copies don't use one database at the same time, then point the app at that `data` folder:

```bash
docker compose down                              # in the launcher's folder
STUDIO37_DATA=/path/to/37Studio/data ~/.local/lib/37studio/37studio
```

## How it differs from the Docker version

- **The agent asks before running commands outside a short list.** In Docker the container keeps the agent away from
  the rest of your computer, so it may run any command there. On the desktop there is no container, so the agent uses
  the `safe` list of commands. Some tasks may stop and say a command isn't allowed.
- **Links, the timeline editor and social sign-ins open in your normal browser.** After a sign-in, the browser tab
  says to go back, and 37Studio shows the account picker by itself. Downloads and backups open a Save dialog.
- **Port 8080.** The app uses `http://localhost:8080` when it's free, the address you register with each platform
  ([Social publishing](../README.md#social-publishing)). If the Docker version is running, it holds 8080 and the
  desktop app picks another port, so social sign-ins won't work until you stop Docker.
- **Closing the window quits the app.** Autopilot, scheduled posts and auto-replies only run while it's open.
- **One copy at a time.** Opening it again brings the open window to the front.

## Troubleshooting

**`Package gtk4 was not found` or `webkitgtk-6.0 was not found` while building.** Step 1 wasn't done, or your Linux is
too old to have WebKitGTK 6.0.

**`go: command not found`.** Install Go, or add it to your PATH: `export PATH=$HOME/.local/go/bin:$PATH` if it's in
`~/.local/go`.

**It doesn't appear in the app menu.** Log out and back in once, or run
`update-desktop-database ~/.local/share/applications`.

**The window opens but the agent says Claude or ffmpeg is missing.** The desktop preview uses the tools installed on
this computer: install them (see *What you need*) and open the app again.

**To see what it's doing**, start it from a terminal: `~/.local/lib/37studio/37studio`.
