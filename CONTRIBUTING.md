# Contributing to 37Studio

Thanks for helping. Bug reports, ideas and pull requests are all welcome.

## Reporting bugs and asking for features

Open an [issue](https://github.com/Shaitrish/37studio/issues). For a bug, include your OS, how you run 37Studio (the launcher or from source), the version (Settings → Updates), what you did, what you expected, and what happened. A screenshot or the server log (`docker compose logs`) helps a lot. Leave out API keys, tokens and customer data.

Found a security problem, for example a way to read another company's files or a leaked token? Don't open a public issue. Email jahangir@shaitrish.com instead.

## Running it from source

See [Local development](README.md#local-development) in the README. In short: Go 1.26+, Node 22+, ffmpeg and the Claude Code CLI, then `./dev.sh`. Or run the whole thing in Docker with `docker compose up --build`.

Before you open a pull request:

```bash
gofmt -l .           # prints nothing when formatting is clean
go vet ./... && go test ./...
cd web && npm run build   # type-checks and builds the UI
```

## Pull requests

- **Small and focused.** One change per pull request is much easier to review. For anything big (a new platform, a new page, a schema change), open an issue first so we can agree on the shape.
- **Match the code around you.** The backend is the Go standard library plus SQLite, with no framework; the UI is React with plain CSS and Phosphor icons. Keep it that way unless there's a strong reason: don't add a dependency for something a few lines can do.
- **Keep it local-first.** 37Studio runs on people's own computers with their own keys. Changes shouldn't send their data anywhere new, add accounts or telemetry, or need a hosted server.
- **Test what can break.** Logic with branches (parsers, schedulers, cost maths, anything touching tokens or files) gets a small test next to it, like the existing `*_test.go` files.
- **Write for the person using it.** UI text and error messages should say what happened and what to do next, in plain words.
- **Update the README** when you change how something is set up or configured.

By sending a pull request you agree that your contribution is licensed under the project's [MIT License](LICENSE).
