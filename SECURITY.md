# Security

## Reporting a vulnerability

Please report security problems privately, not in a public issue: use **Report a vulnerability** on the repository's [Security tab](https://github.com/Shaitrish/37studio/security/advisories/new), or email [jahangir@shaitrish.com](mailto:jahangir@shaitrish.com). Include the version (Settings shows it), how you run it (launcher, cloud compose file or from source) and the steps to reproduce.

You'll get a reply within a few days. Fixes go out as a new release, and with your permission the advisory credits you.

## Supported versions

Only the latest release gets fixes. The launcher updates when it starts again; from source, pull `main`.

## How 37Studio is meant to be run

Some things are by design, and the README explains each one:

- **The container is the sandbox.** The launcher and the cloud file run the agent in `bypass` mode. Anyone who can use the app can have the agent run commands inside the container, read `/data` and reach the internet. `safe` mode narrows what the agent runs, but it isn't a security boundary.
- **The app trusts whoever can reach it.** The launcher only listens on `127.0.0.1`. On a server, the cloud file requires `STUDIO37_PASSWORD` and HTTPS. Share that password only with people you'd trust with a shell in the container.
- **`/data` holds everything**, including `secret.key`, which encrypts the stored keys and platform tokens. Treat backups of it like passwords.
