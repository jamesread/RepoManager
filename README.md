# RepoManager

<div align="center">
  <img alt="project logo" src="frontend/public/logo.svg" width="128" />
  <h1>RepoManager</h1>

  Browse and manage Git repositories under a directory, from a terminal UI and a web UI.

  [![Maturity Badge](https://img.shields.io/badge/maturity-beta-yellow.svg)](#)
  [![Discord](https://img.shields.io/discord/846737624960860180?label=Discord%20Server)](https://discord.gg/jhYWWpNJ3v)

</div>

## Getting started

```bash
make setup
make dev-service    # web UI + Connect API (:8080)
make dev-frontend   # Vite dev server (:3000)
```

Or use the terminal UI only:

```bash
make service
./plist /path/to/repos
```

Default web login on first start: **admin** / **admin** (change immediately in production).

## Architecture

This project follows [jwr-soa-2.0](https://github.com/jamesread/RepoManager): Connect RPC (`protocol/`), SQLite IAM (`armature-iam`), koanf configuration, PicoCrank Vue SPA, and Prometheus metrics at `/metrics`.
