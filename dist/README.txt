37Studio
========

1. Install Docker Desktop (https://www.docker.com/products/docker-desktop/). Works on Windows, Intel Macs and
   Apple Silicon Macs (M1-M4).
2. Double-click "start.bat" (Windows) or "start.command" (Mac; Linux: ./start.sh). It starts Docker Desktop for you.
   Mac, first time only: right-click start.command -> Open -> Open (macOS asks because it was downloaded).
   The first start downloads about 3 GB. Every later start checks for updates first.
3. 37Studio opens in its own window at http://localhost:8080. It only accepts connections from this computer.
   To keep it in your taskbar or Dock: Chrome/Edge menu -> "Install 37Studio", or Safari -> File -> Add to Dock.
4. Settings: paste your license key and your Claude key (Anthropic API key, or a Claude subscription token from
   `claude setup-token`). Gemini and ElevenLabs keys are optional. Keys are encrypted and stay on this computer.

Inbox auto-replies and Autopilot work while 37Studio runs. After the first start it comes back by itself whenever Docker
Desktop starts (Docker Desktop -> Settings -> General -> "Start Docker Desktop when you sign in"). To answer messages at
night, keep the computer awake: Windows -> Settings -> Power -> "When plugged in, put my device to sleep after: Never";
Mac -> System Settings -> Energy (or Battery -> Options) -> "Prevent automatic sleeping when the display is off".

Your work lives in the "data" folder next to this file. Back it up; deleting it deletes your companies and videos.
To stop 37Studio: Docker Desktop -> Containers -> 37studio -> Stop.
