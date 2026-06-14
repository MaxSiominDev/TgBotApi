# Self-hosted Telegram Bot API

Your own [Telegram Bot API](https://github.com/tdlib/telegram-bot-api) server in Docker, behind
Caddy, with health checks and a small metrics page. Push to `main`, it deploys to the VPS.

## Why

The public API at `api.telegram.org` caps downloads at 20 MB and uploads at 50 MB. Run the
server yourself in `--local` mode and that jumps to 2 GB, with files served straight off local
disk. If your bot moves big files, that's the point.

The cost is running and patching a C++ server that faces the internet. The rest of this repo
is here to make that less annoying.

## What runs

- `telegram-bot-api`: the server itself, local mode, on `:8081`
- `caddy`: TLS for your domain(s) plus a token gate in front of the API
- `healthcheck`: small Go service that sends a real test message and tells you if it worked
- `netdata` + `monitor-auth`: CPU/RAM/disk once a second, behind a login page

## Deploy

Two workflows:

- **Build Binary** (manual): compiles tdlib's server for amd64 and arm64 and attaches the
  binaries to a release. Run it once, then again when you want a newer upstream build. It's
  slow, which is why it's not tied to pushes.
- **Deploy** (on push to `main`): detects the VPS arch, builds and pushes the images, runs the
  Ansible playbook (Docker + firewall), ships the configs, writes the `.env` files from
  secrets, and brings the stack up. Then it polls the health endpoints until they pass and
  pings you on Telegram.

Secrets it expects:

| Secret | For |
| --- | --- |
| `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY` | SSH access |
| `VPS_DOMAINS` | domain(s), space-separated |
| `API_TOKEN` | the `X-Api-Token` you send to reach the API |
| `TELEGRAM_API_ID`, `TELEGRAM_API_HASH` | from my.telegram.org |
| `TEST_BOT_TOKEN`, `TEST_CHAT_ID` | bot and chat for the health check |
| `MONITOR_USER`, `MONITOR_PASSWORD` | metrics page login |
| `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID` | deploy notifications |

## Use

Every API call needs the token:

```sh
curl https://your-domain.com/bot<BOT_TOKEN>/getMe -H "X-Api-Token: <API_TOKEN>"
```

Wrong token or none, you get a `403`. Point your bot library's API root at
`https://your-domain.com` and have it send the header.

Health: `GET /health/containers` (no auth), `/health/telegram` and `/health/send-message`
(both need the token).

Metrics: open `https://your-domain.com:19999`, log in, and you get CPU/RAM/disk with history.
