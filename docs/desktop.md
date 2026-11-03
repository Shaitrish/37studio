# Desktop app (preview)

37Studio as a normal app in its own window, without Docker. It is the same app as the Docker version: the same
brands, projects, agent and publishing, served to a native window instead of a browser tab.

**Status: preview, Linux and Windows.** From 0.9.0 on, each
[release](https://github.com/Shaitrish/37studio/releases) has a .deb for Ubuntu 24.04+ and Debian 13+ and a plain
program for other Linux, for Intel/AMD (`amd64`) and ARM (`arm64`) computers, and a .zip for Windows 10 and 11 on
Intel/AMD. A Mac build and automatic setup of the video tools come later; until then, the
[ready-made Docker app](../README.md#on-your-computer-the-ready-made-app) is the way to run 37Studio on a Mac.

## Install from a release (Ubuntu and Debian)

1. On the [latest release](https://github.com/Shaitrish/37studio/releases/latest), download
   `37studio_<version>_amd64.deb` (or `_arm64.deb` on an ARM computer; `dpkg --print-architecture` says which).
2. Install it, which also installs ffmpeg and Python if they're missing:

   ```bash
   sudo apt install ./37studio_*_amd64.deb
   ```

3. Install Claude Code, which no Linux distribution packages: `sudo npm install -g @anthropic-ai/claude-code`
   (needs Node.js 22 or newer; see *What you need* below).
4. Open **37Studio** from your app menu. Overview → **Finish setup** lists anything still missing.

To update, install the next release's .deb the same way: Settings → About says when one is out. To remove the app,
`sudo apt remove 37studio`; your work stays in `~/.local/share/37studio`.

**Other Linux** (Fedora, Arch, …): download `37studio-<version>-linux-amd64.tar.gz`, unpack it and run `./37studio`.
It needs GTK 4 and WebKitGTK 6.0 (Fedora: `sudo dnf install gtk4 webkitgtk6.0`) and the tools under *What you need*.
Keep the program's name `37studio`. Each release's `.sha256` file has the checksums: `sha256sum -c *.sha256`.

## Install from a release (Windows)

Windows 10 or 11 on an Intel or AMD computer. The window is drawn by WebView2, which both already have.

1. On the [latest release](https://github.com/Shaitrish/37studio/releases/latest), download
   `37studio-<version>-windows-amd64.zip`.
2. Right-click it → **Properties** → tick **Unblock** → **OK**, then unpack it. Windows marks downloaded files, and
   refuses to run what it marked.
3. In the unpacked folder, right-click `install-windows.ps1` → **Run with PowerShell**. It copies the app to
   `%LOCALAPPDATA%\Programs\37Studio` and adds it to the Start menu: no admin rights, nothing for other users.
   From a terminal instead: `powershell -ExecutionPolicy Bypass -File .\install-windows.ps1`
4. Install the tools the agent uses, which Windows comes with none of (see *What you need*):

   ```powershell
   winget install Git.Git Gyan.FFmpeg OpenJS.NodeJS.LTS Python.Python.3.13
   npm install -g @anthropic-ai/claude-code
   ```

   Open a new terminal afterwards, so it picks up the changed PATH. Git is there for its bash: Claude Code runs
   the agent's commands through it on Windows, so without Git the agent fails on its first step.
5. Press the Windows key, type **37Studio** and open it. Overview → **Finish setup** lists anything still missing,
   with the command to install it and a **Check again** button.

You can also just run `37studio.exe` from the unpacked folder, without installing it. Keep the program's name
`37studio.exe`: the agent runs `37studio gen|clip|transcribe` from the folder it sits in.

To update, unpack the next release and run `install-windows.ps1` again with 37Studio closed; Settings → About says
when one is out. To remove the app, `.\install-windows.ps1 -Uninstall`; your work stays in `%AppData%\37Studio`.

Each release's `.sha256` file has the checksum to compare against:

```powershell
(Get-FileHash .\37studio-0.9.0-windows-amd64.zip -Algorithm SHA256).Hash.ToLower()
```

**SmartScreen.** The program isn't signed yet, so the first run may show *Windows protected your PC*: click **More
info** → **Run anyway**. Signing comes with the first release out of preview.

## Build it yourself

For changes that aren't released yet, or to work on the app.

### What you need

- **On Linux**, Ubuntu 24.04 or newer, Debian 13 or Fedora 40, or another Linux with GTK 4 and WebKitGTK 6.0.
  **On Windows**, Windows 10 or 11 on an Intel or AMD computer; the WebView2 runtime that draws the window is
  already there.
- [Go 1.26](https://go.dev/dl/) or newer to build the app, and [Node.js 22](https://nodejs.org) or newer to build
  the interface
- The tools the agent uses. The Docker image ships them; the desktop app uses the ones on this computer:

  | Tool | What for |
  | --- | --- |
  | Claude Code | the editor agent, the Assistant, captions and replies |
  | ffmpeg | cutting, probing and encoding video |
  | Node.js 22 or newer | rendering, previews and the timeline editor (HyperFrames runs through `npx`; its Chrome downloads by itself on the first render) |
  | Python 3 with venv | transcription and YouTube or Drive imports (faster-whisper and yt-dlp install themselves on first use, about 200 MB) |
  | Noto fonts | Bangla, other non-Latin scripts and emoji in rendered videos |
  | Git for Windows (Windows only) | the shell Claude Code runs the agent's commands through |

  On Ubuntu or Debian, all of them at once:

  ```bash
  sudo apt install ffmpeg nodejs npm python3 python3-venv fonts-noto-core fonts-noto-color-emoji
  sudo npm install -g @anthropic-ai/claude-code
  ```

  Ubuntu before 26.04 and Debian before 13 ship an older Node.js: install version 22 or newer from
  [nodejs.org](https://nodejs.org) instead, and leave `nodejs npm` out of the first line.

  On Windows, all of them at once:

  ```powershell
  winget install Git.Git Gyan.FFmpeg OpenJS.NodeJS.LTS Python.Python.3.13
  npm install -g @anthropic-ai/claude-code
  ```

  Open a new terminal afterwards, so it picks up the changed PATH. There is no Noto line: Windows 10 and 11 already
  have Bangla (Nirmala UI) and emoji (Segoe UI Emoji).

  The app's Overview page lists anything still missing under **Finish setup**, with the command to install it and a
  **Check again** button.

### Install on Linux

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

### Install on Windows

Nothing to install first: the app builds with Go and Node alone.

```powershell
git clone https://github.com/Shaitrish/37studio.git
cd 37studio
powershell -ExecutionPolicy Bypass -File .\desktop\install-windows.ps1
```

The first run takes a few minutes: it builds the interface and the app, then copies it to
`%LOCALAPPDATA%\Programs\37Studio` and adds it to the Start menu. Press the Windows key, type **37Studio** and open
it; to keep it handy, right-click it there and choose **Pin to Start** or **Pin to taskbar**.

To build without installing, `.\desktop\build.ps1` leaves the program at `desktop\bin\37studio.exe`. To build the
release zip the way CI does, `.\desktop\package-windows.ps1`.

Then follow [First steps](../README.md#first-steps): add your Claude key in Settings → Keys and add a brand.

### Update

```bash
cd 37studio
git pull
./desktop/install-linux.sh
```

On Windows, with 37Studio closed:

```powershell
cd 37studio
git pull
powershell -ExecutionPolicy Bypass -File .\desktop\install-windows.ps1
```

### Uninstall

```bash
./desktop/install-linux.sh --uninstall
```

```powershell
powershell -ExecutionPolicy Bypass -File .\desktop\install-windows.ps1 -Uninstall
```

This removes the app and its menu entry. Your brands, projects and videos stay where they are (below); delete that
folder too to remove everything.

## Where your work is kept

Everything lives in one folder: the database, `secret.key` (which decrypts your keys), brand files, footage and
videos. Back it up, or use Settings → Backup.

| | |
| --- | --- |
| Linux | `~/.local/share/37studio` |
| Windows | `%AppData%\37Studio` |

**Using your Docker data.** The desktop app starts empty. To open the work you made in the Docker version, stop
Docker first, so two copies don't use one database at the same time, then point the app at that `data` folder:

```bash
docker compose down                              # in the launcher's folder
STUDIO37_DATA=/path/to/37Studio/data ~/.local/lib/37studio/37studio
```

```powershell
docker compose down                              # in the launcher's folder
$env:STUDIO37_DATA = "C:\path\to\37Studio\data"; & "$env:LOCALAPPDATA\Programs\37Studio\37studio.exe"
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
this computer: install them (see *What you need*), then click **Check again** under Finish setup on the Overview page.
If a tool still shows as missing, it's installed somewhere the app doesn't look (for example `~/.local/bin` before
you've logged out and back in): log out and in once, or start the app from a terminal. On Windows, close and reopen
37Studio after a `winget install` — a running program keeps the PATH it started with.

**Windows: `running scripts is disabled on this system`.** PowerShell won't run downloaded scripts by default. Run
it as `powershell -ExecutionPolicy Bypass -File .\install-windows.ps1`, which allows that one script and changes
nothing else.

**Windows: *Windows protected your PC*, or the app won't start after downloading.** The program isn't signed yet.
Click **More info** → **Run anyway**, and tick **Unblock** in the .zip's Properties before unpacking it.

**Windows: the agent stops on its first command, or every Bash step fails.** Git for Windows isn't installed.
Claude Code runs the agent's commands through its bash, and looks for it where Git installs itself rather than on
PATH, so having `git` elsewhere doesn't help: `winget install Git.Git`, then reopen 37Studio.

**Windows: nothing happens when the agent runs `37studio gen`.** The program has to keep the name `37studio.exe`
and stay in the folder the app runs from: the agent finds it beside the app.

**To see what it's doing**, start it from a terminal: `~/.local/lib/37studio/37studio`.
