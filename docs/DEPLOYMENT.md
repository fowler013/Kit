# Run Kit continuously in Discord

Kit is a long-running Gateway bot, not a Discord Activity or a web UI. Its HTTP
server only provides health checks. Discord-only operation needs one bot token;
Slack and AI providers are optional. Camp commands do not require an AI key.

## 1. Configure Discord

Follow [DISCORD_SETUP.md](DISCORD_SETUP.md) to create and invite the bot.
Enable **Message Content Intent** in the Developer Portal. Kit requests Guilds,
Guild Messages, Direct Messages, and Message Content; it does not need the
privileged Members or Presence intents.

Grant **View Channel** and **Send Messages** in the intended channels. Read Message
History is useful for viewing context but is not required by these live-message
handlers. Do not grant Administrator, Manage Roles, or Manage Channels to the
running bot. The optional `cmd/server-setup` utility is a separate administrative
operation and is not run during startup or deployment.
Its existing [server template](../config/discord-server-template.json) can be
reviewed with `go run ./cmd/server-setup -guild <SERVER_ID> -dry-run`. Review
the proposed roles/channels before a separately authorized run without
`-dry-run`; that operation needs Manage Roles/Manage Channels, which the normal
bot should not retain.

For camp data, install the bot only in your trusted camp server. Use a private
staff channel, deny ordinary members View Channel there, and list its numeric
ID in `CAMP_ALLOWED_CHANNEL_IDS`. **Camp answers are posted in the channel where
the question was asked**, so Kit only shows registration data in DMs or in the
listed channels (and threads inside them). Authorized staff who ask anywhere
else get a privacy notice instead of data, and unauthorized users get a refusal.
Kit cannot see who may read a listed channel, so only list channels that are
actually private. DMs require a user ID allowlist because a DM has no server
role context. Prefer explicit user IDs to role names.

## 2. Configure and run locally

Use Go 1.25 (at least the `go.mod` toolchain version is required).

```powershell
Copy-Item .env.example .env
# Edit .env privately; set DISCORD_BOT_TOKEN and leave Slack values empty.
go run .
```

Do not put tokens in chat, command-line arguments, Git, Docker build arguments,
or screenshots. A malformed configuration or rejected Discord/Slack login exits
nonzero instead of leaving an offline bot running silently.

Optional AI: configure an OpenAI-compatible endpoint and model, or Gemini/Claude.
Choose an available model in the provider console. Blank AI settings give basic
responses; `!status` distinguishes configured providers from basic mode. It does
not claim that an API key or model has been verified until a request succeeds.

### Docker Compose

From the repository root, with a privately configured `.env` and Docker running:

```powershell
docker compose --env-file .env -f config/docker-compose.yml up -d --build
docker compose -f config/docker-compose.yml ps
docker compose -f config/docker-compose.yml logs --tail 50 slack-ai-bot
```

Compose passes all settings at runtime, runs the bot as a non-root user with a
read-only filesystem, and restarts it unless stopped. Health is bound to
`127.0.0.1:8080` on the host; set `KIT_HOST_PORT` if that port is occupied.
The internal port stays 8080. No Redis service is needed or started by default;
conversation state is in memory and is lost on restart.

Do not run a second local copy while the hosted copy uses the same Discord token.
To stop this deployment, use
`docker compose -f config/docker-compose.yml down`.

## 3. Railway service settings

Create a service from `fowler013/Kit` only when ready to authorize hosting costs.
Select the reviewed revision containing these changes. Keep it separate from
the CampPowerUp web service: their runtimes and credentials are different.

| Setting | Value |
| --- | --- |
| Build | Root `Dockerfile` (automatically detected) |
| Start command | Leave unset; use the image entrypoint |
| Variables | `DISCORD_BOT_TOKEN`; optional settings from `.env.example` |
| Port | Railway-injected `PORT`, or explicitly set `PORT=8080` |
| Healthcheck path | `/readyz` |
| Healthcheck timeout | 150 seconds |
| Replicas | 1; no autoscaling or cron schedule |
| Serverless / sleeping | Disabled |
| Restart policy | Always (requires a paid plan) |

Railway Free/trial plans limit On Failure to 10 restarts and do not allow Always.
Do not treat that limited configuration as unattended, continuous hosting.
Keep tokens out of build-time variables/arguments. A public domain is optional
for external monitoring; no incoming Discord interaction endpoint is needed.

Railway's current documentation deprecates `railway.json`/`railway.toml` Config as
Code for new services, with a 2026-12-01 cutoff for existing services. This guide
therefore uses Dockerfile detection plus explicit service settings rather than
adding a configuration file that new services cannot use.

Railway's healthcheck is **startup-only**, not ongoing monitoring. Set up an
external readiness monitor if you need outage alerts. DiscordGo reconnects after
a lost Gateway connection; `/readyz` goes unavailable while disconnected.
Restart policy handles process exits, not merely an unhealthy HTTP endpoint.
Compose also marks health failures but does not restart an otherwise live
process solely because its probe failed.

Use only one active bot process. Rolling deployments can temporarily overlap
old and new processes even with one replica, causing duplicate replies/alerts.
Stop the previous bot before starting a replacement when duplicates must be
avoided; this entails a brief maintenance window. Kit has no distributed leader
election or exactly-once notification storage.

The GitHub **Publish Container Image** workflow publishes to GHCR only. It does
not deploy to Railway or claim that a bot is running.

## 4. Enable CampPowerUp only after verifying the backend

The previously documented
`https://camppowerup-production.up.railway.app` returned Railway
`404 Application not found` for `/`, `/admin/login`, and `/admin/export-json`
when checked on 2026-11-01. **Do not assume this is the current deployment URL.**
Get the actual registration-service URL from the owner/Railway dashboard.

Before configuring Kit:

- Verify the root page and `/admin/login` return the expected registration app.
- Verify an unauthenticated request to `/admin/export-json` is refused or
  redirected to login. The selected CampPowerUp source currently defines that
  route without `@require_admin_auth`; protect it before enabling live camp data.
- In an authorized private session, verify form login (`username`, `password`)
  sets a session cookie and authenticated export returns JSON with a
  `registrations` array. Do not paste exports or camper records into chat/logs.

Then set these runtime variables privately:

`CAMP_ADMIN_USERNAME` and `CAMP_ADMIN_PASSWORD` must match that CampPowerUp
deployment's `ADMIN_USERNAME` and `ADMIN_PASSWORD`, not Discord credentials.
Use the operator-configured values rather than assuming repository defaults.

```dotenv
CAMP_API_BASE_URL=https://your-verified-registration-service
CAMP_ADMIN_USERNAME=
CAMP_ADMIN_PASSWORD=
CAMP_ALLOWED_DISCORD_IDS=
# Optional, trusted server role name:
CAMP_ALLOWED_ROLE=
# Numeric IDs of private staff channels allowed to show camp data. DMs always
# work; if empty, camp data is only answered in DMs:
CAMP_ALLOWED_CHANNEL_IDS=
# Optional:
CAMP_CAPACITY=30
CAMP_POLL_MINUTES=5
CAMP_ALERTS_CHANNEL=
CAMP_STATUS_CHANNEL=
```

Use `!myid` for staff user IDs. In Discord, enable Developer Mode and use
right-click > Copy Channel ID for `CAMP_ALLOWED_CHANNEL_IDS`. Prefer numeric
channel IDs for alerts; duplicate
channel names are rejected rather than choosing an arbitrary server. Check
channel visibility with a non-staff account before using roster or payment
commands. Kit composes camp answers directly, not through AI providers.

The client retries login once for expired sessions/401/403, refuses cross-origin
redirects, and reports invalid JSON or HTTP failures rather than inventing zero
registrations. Website monitoring treats 404 as down. Alert baselines are
in-memory: startup establishes a baseline, not a replay of missed registrations.

## 5. Go-live acceptance checks

These are operator checks, not a claim that this checkout has been deployed:

- Confirm exactly one bot process is running and Discord shows it online.
- In the intended channel, send `!help`, `!status`, `!myid`, and `!links`
  **without mentioning the bot**; then try a mention and a DM.
- Confirm ordinary unmentioned conversation and other bots' commands are ignored.
- If AI is configured, ask one non-sensitive question and check the provider's
  response; if not configured, expect basic mode.
- With camp enabled, authorized staff can run `!camp stats` in a listed staff
  channel or a DM. In any other channel they get a privacy notice and no data;
  unauthorized users receive a refusal. Exercise roster/payment commands only in
  a private channel.
- Query `/healthz` (HTTP listener alive) and `/readyz` (all configured chat
  gateways ready). `/readyz` must be 503 before connection or during disconnect.
- Run `/slack-ai-bot --health-check` inside the container: exit 0 means ready;
  exit 1 means unavailable. This command probes the existing process and never
  logs into Discord/Slack itself.
- Restart the service, confirm one reconnect and resumed replies, and check that
  shutdown cancels monitoring without sending spurious website-down alerts.

Local automated checks use synthetic Discord Gateway/REST and CampPowerUp
servers; they do not authenticate to live accounts or read actual registrations:

```powershell
go test -race -count=1 -timeout 120s ./...
go vet ./...
go build -o kit-ai-bot.exe .
```

References: [Dockerfiles](https://docs.railway.com/builds/dockerfiles),
[healthchecks](https://docs.railway.com/deployments/healthchecks),
[restart policy](https://docs.railway.com/deployments/restart-policy),
[Serverless](https://docs.railway.com/deployments/serverless),
[Config as Code](https://docs.railway.com/config-as-code).
