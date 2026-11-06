# Kit - Multi-Platform AI Bot 🤖

Kit is a Go application that connects to Discord and/or Slack. Run it continuously
as a Gateway bot, with optional AI providers and authorized CampPowerUp lookups.
For hosting, server permissions, and go-live checks, start with
**[Deploy Kit to Discord](docs/DEPLOYMENT.md)**.

## ✨ Features

- 🧠 **Google Gemini 1.5 Flash Integration**: Powered by Google's latest AI model
- 🌐 **Multi-Platform Support**: Works on both Slack and Discord simultaneously
- 🚀 **Real-time Communication**: Uses platform-native protocols for instant responses
- 💬 **Intelligent Conversations**: Context-aware responses to any question
- **Game-first Discord help**: Beginner-friendly tagging, DMs, commands, and server navigation with optional public server reference.
- **Private staff recaps**: Allowlisted channel summaries with anonymization before AI.
- 📝 **Comprehensive Logging**: Detailed event logging for monitoring
- 🔧 **Easy Configuration**: Simple environment-based setup
- 🎯 **Channel Aware**: Works in public channels, private channels, and DMs
- 🛡️ **Secure**: Token-based authentication with platforms and AI services
- 🔄 **Resilient**: Automatic fallback to basic responses if AI fails
- ⚙️ **Flexible Deployment**: Run on one platform or both simultaneously

## 🚀 Quick Start

### Prerequisites
- Go 1.25 installed (see the minimum/toolchain versions in `go.mod`)
- Slack workspace with admin permissions (for Slack integration)
- Discord server with admin permissions (for Discord integration)
- Slack app configured only if using Slack (see [SLACK_SETUP.md](docs/SLACK_SETUP.md))

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/fowler013/Kit.git
   cd Kit
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Configure environment**
   ```bash
   cp .env.example .env
   # Set DISCORD_BOT_TOKEN; leave unused Slack and AI settings empty
   ```

4. **Build and run**
   ```bash
   go build -o kit-ai-bot
   ./kit-ai-bot
   ```

## 📖 Documentation

- **[DEPLOYMENT.md](docs/DEPLOYMENT.md)** - Continuous hosting, health checks, camp connection, and go-live checklist
- **[HOW_TO_USE.md](docs/HOW_TO_USE.md)** - Complete usage guide
- **[USING_DISCORD.md](docs/USING_DISCORD.md)** - Beginner-friendly Discord interactions guide
- **[SLACK_SETUP.md](docs/SLACK_SETUP.md)** - Slack app configuration
- **[DISCORD_SETUP.md](docs/DISCORD_SETUP.md)** - Discord bot configuration
- **[DEVELOPMENT.md](DEVELOPMENT.md)** - Development guidelines
- **[PROJECT_BOARD_SETUP.md](PROJECT_BOARD_SETUP.md)** - Project management

## 🔧 Configuration

Your `.env` file should contain (configure platforms as needed):
```bash
# Slack Configuration (optional; set both or leave both empty)
SLACK_BOT_TOKEN=
SLACK_APP_TOKEN=

# Discord Configuration (optional)
DISCORD_BOT_TOKEN=your-discord-bot-token-here

# AI Configuration (optional; select an available model in your provider console)
GEMINI_API_KEY=
GEMINI_MODEL=
```

## 🌐 Platform Support

### Slack Integration
- Socket Mode for real-time responses
- Mentions and direct messages
- Slash commands support
- Rich message formatting

### Discord Integration  
- WebSocket connection for real-time responses
- Mentions and direct messages
- Built-in `!` commands in server channels without a mention
- Guild-scoped slash commands, registered automatically when a guild ID is configured
- Server and DM support
- Rich embed formatting

## 💬 Usage Examples

### Slack
```
@Kit explain quantum computing in simple terms
@Kit what are best practices for team collaboration?
@Kit help me brainstorm ideas for our project
```

### Discord
```
@Kit what's the weather like today?
@Kit can you help me understand this concept?
!status
!help
```

### Discord slash commands

Slash commands are registered for a configured Discord server, not DMs.
`/help` (or `/commands`) lists general commands and only shows staff workflows
available to the caller.

| Command | Example | Behavior |
| --- | --- | --- |
| `/status` or `/health` | `/status` | Show Kit connection and AI-provider status. |
| `/help`, `/commands`, or `/kit help` | `/kit help` | List commands available to you, including beginner Discord help. |
| `/version` | `/version` | Show Kit's version. |
| `/myid` | `/myid` | Show your Discord user ID privately. |
| `/links` | `/links` | Show Camp Power-Up website links. |
| `/kit ask` | `/kit ask prompt:"Suggest a general camp packing list" visibility:private` | Ask Kit through the configured AI providers. |
| `/kit status` | `/kit status` | Staff-only, private operational health; no camper data or secrets. |
| `/kit recap` | `/kit recap channel:#minecraft` | Staff-only, ephemeral summary of up to 100 recent messages within 24 hours from an allowlisted channel. |
| `/kit remind create` | `/kit remind create in:30m message:"Prepare the equipment"` | Schedule a one-off reminder in the invoking private staff channel. |
| `/kit remind list` | `/kit remind list` | Privately list your active reminder IDs and UTC times in this channel. |
| `/kit remind cancel` | `/kit remind cancel id:1` | Privately cancel your reminder in this channel. |
| `/ticket new` | `/ticket new type:feedback` | Staff-only request, feedback, bug, or idea form; receipt is private. |
| `/camp` | `/camp query:stats` | Run an authorized, private Camp Power-Up query. |
| `/attendance` | `/attendance parent session:<configured-slug> date:<YYYY-MM-DD>` | Private parent/staff workflow; disabled until separately approved and enabled. |

`/kit ask` visibility defaults to **private** (ephemeral). Choose
`visibility:public` only when you intend everyone in that channel to see the
answer. Replies are deferred before provider calls, long answers are split into
Discord-safe chunks, and unsolicited mentions are suppressed. Private slash
chats use separate conversation history from public and legacy chats. Never
include camper or family private information in an AI prompt, even in a private
reply.

`/camp` is **always private** and reads registration data directly from
Camp Power-Up; that data is not sent to an AI provider. `/kit ask` prompts
recognized as camp-registration queries follow the same authorization checks
and are forced private even if `public` was selected. Supported `/camp query`
values are `help`, `stats`, `roster`, `unpaid`, and `capacity`.

### Staff access and privacy

#### Discord and server help

Kit remains a camper-friendly game guide first. Ask `@Kit how do I tag someone?`,
`!kit how do I send a message to a friend?`, or `/kit ask` for beginner Discord help.
Optional `KIT_SERVER_GUIDE` accepts inline text or a UTF-8 file path, loaded once
at startup with a 4,000-character limit; only its presence is logged. Copy and
customize [the public guide template](server_guide.example.md). Missing file paths,
invalid UTF-8, and oversized guides fail startup rather than silently losing context.

For navigation questions **in the configured guild only**, Kit sends the guide
and up to 40 public text/announcement/forum channel names, truncated topics and
public category names to the AI provider. Visibility is calculated from the
`@everyone` base role and its channel overwrites; role/member-only channels are
excluded. Gateway state is used first, REST if unavailable, cached for five
minutes (permission changes can take up to that long to appear). DMs, other
guilds, Slack and ordinary game questions get no server reference. The keyword
heuristic is intentionally broad; ambiguous words like "command" may match.
Keep the guide public, factual and current. Kit cannot verify unlisted buttons
or invent server features.

#### Staff channel recaps

`/kit recap channel:<channel>` requires `DISCORD_ATTENDANCE_STAFF_IDS`,
the configured `DISCORD_GUILD_ID` (or its documented fallback), and an exact
channel ID in comma-separated `KIT_RECAP_CHANNEL_IDS`. Empty means disabled.
Only text/announcement channels in that guild are accepted, not threads/forums.
Grant Kit **View Channel** and **Read Message History** on every allowlisted
channel. Recaps do not require the source to be staff-private: the operator must
explicitly review which channels are appropriate to summarize.

Kit inspects at most 100 recent messages from the last 24 hours, excludes
bots/webhooks/system messages, orders eligible messages oldest-first, and
labels authors consistently as `Camper 1`, `Camper 2`, etc. Known author and
mentioned-user names/nicknames, Discord mentions, links, emails, phone-like
numbers and attachment contents are removed/replaced before AI. Each message
is capped near 300 characters and total transcript at 20,000 characters.
This is best-effort redaction, **not a guarantee that arbitrary prose is
anonymous**; unknown names and indirect identifying details may remain.
Do not allowlist channels containing sensitive personal, health or family data.

**Anonymized text is sent to the configured AI provider(s)** using the normal
provider fallback order. Review provider retention and your community's notice
and consent requirements before enabling. The neutral summary flags possible
safety/conduct concerns generically for staff follow-up, without quotes or
individual attribution; it is a limited sample, not a moderation verdict.
No eligible messages means no AI call. Recaps bypass Kit's session storage,
do not persist transcripts or summaries in Kit, and never log message content.
Discord/provider retention still applies. All results and refusals are ephemeral
to the requester, with mentions suppressed. Mentions/DMs/`!kit`/`/kit ask`
cannot fetch recaps; use the staff slash command.

#### Staff tickets

`/ticket new type:<request|feedback|bug|idea>` opens a form with a required title
(100 characters) and details (1,500 characters). **No medical, family, or camper
personal info. Not for emergencies or safety reports.** Use the camp's established
safety/emergency reporting process instead. Phase 1 is approved staff only:
`DISCORD_ATTENDANCE_STAFF_IDS` and the configured guild are checked on command,
submission, and every button. Parents and campers cannot submit or operate tickets.

`KIT_TICKETS_CHANNEL_ID` is optional; unset disables intake with a private reply.
When enabled, that exact destination must pass the same live, fail-closed
staff-only permission check described below for reminders: explicit `@everyone`
View Channel denial and no unrecognized member/role visibility grants.
The ticket variable explicitly approves the destination; it need not also be in
`CAMP_ALLOWED_CHANNEL_IDS`. Only guild text channels are supported, not forums or
private per-ticket channels. Server administrators retain inherent visibility.
Kit needs View Channel, Send Messages, Embed Links, Read Message History,
Create Public Threads, and Manage Threads in that staff-only destination.
**Administrator bypasses channel overwrites, including for apps/bots.** The
privacy check does not exclude administrators or administrator-enabled apps
(such as Appy, MEE6, or Sapphire). An operator must separately review the actual
staff/admin/app audience and permissions before enabling intake; do not treat
passing the overwrite check as proof that no third-party app can read tickets.

Each ticket is a team-visible message with a random ID, type, submitter mention
(no ping), UTC creation time, and Open status, plus a discussion thread. Staff can
**Claim**, **Resolve** (archives the thread), or **Reopen** (unarchives it).
Kit edits the original message and sends only an ephemeral receipt with its jump
link; it never DMs, forwards tickets to AI, or posts to another destination.
All posts/edits suppress mentions; `@everyone` and `@here` are neutralized.
Logs contain ticket IDs and outcomes, not titles or details.

**Message-backed status and controls survive restarts; the per-user limit of five
submission attempts per ten minutes resets.** Forms expire after ten minutes,
are replaced when the same staff member opens another form, and must be reopened
after a restart. Each form can attempt delivery only once. Uncertain API outcomes
are reported privately without automatic retries; check the channel before
resubmitting. A thread failure leaves the ticket intact with an explicit warning.
Thread archival failures also report partial success and need operator attention.
Stale controls are rejected using message-backed revisions and fresh message reads.
Run **one active Kit instance**: Discord message edits have no cross-instance
compare-and-swap protection. Deleting a ticket message deletes its stored state.
Changing the destination disables controls on tickets left in the old channel.

`/kit status` and `/kit remind` require an approved ID in
`DISCORD_ATTENDANCE_STAFF_IDS` and the configured guild; all replies, denials,
and confirmations are ephemeral. These operational commands do not grant
Camp Power-Up data access. `/kit status` reports process uptime, commit (Railway
`RAILWAY_GIT_COMMIT_SHA`, build `-ldflags "-X main.buildCommit=<git SHA>"`, or Go
VCS metadata), configured AI providers with last-request health, attendance
enablement, and the startup slash-registration result. AI health is **unknown
until used**, not a paid test request. CampPowerUp is probed without credentials
at `/healthz` with a two-second timeout; HTTP 404 means reachable but no health
endpoint. No registration endpoints or response bodies are included.

Reminders accept Go durations such as `30m`, `2h`, or `1h30m`, from one second
through `168h` (seven days; use `168h`, not `7d`). Text is limited to 1,000
characters, with at most five active reminders per user across channels.
**Reminders are in memory and lost on restart or deployment.** They never DM,
never change channels, and suppress all mentions, including `@everyone`, roles,
and user pings. Do not put camper, family, or pickup data in reminder text:
delivery is a channel message, not an ephemeral reply.

The destination must be explicitly listed in `CAMP_ALLOWED_CHANNEL_IDS` or be
`KIT_OPS_CHANNEL_ID`, and must pass a live permissions check both at creation
and delivery. Only guild text channels are accepted (not threads). The channel
must explicitly deny **View Channel** to `@everyone`; any overwrite allowing
visibility must be for an allowlisted staff member, Kit itself, or the exact
staff role configured by `CAMP_ALLOWED_ROLE` (case-insensitive name match).
Unrecognized role/member grants, inaccessible permissions, and public channels
fail closed. Server administrators retain Discord's inherent visibility.
If permissions become unsafe or delivery fails, the reminder is dropped with
a generic operator log rather than retried or redirected. Shutdown cancels
pending timers and waits for bounded in-flight delivery.

Optional `KIT_OPS_CHANNEL_ID` enables one concise, mention-suppressed startup
health line after Discord Ready and command registration. Unset means **no
startup post**. The destination must pass the same fresh staff-only permissions
check as reminders. Reconnects do not repeat it; invalid IDs, unknown/public
audiences, or send failures are logged without stopping the bot. Set this only to a private staff operations
channel after separately approving the production variable change.

Discord's default slash-command permission is **Use Application Commands**;
Administrator is not required for ordinary command use. This platform
permission is not staff authorization. `/camp` additionally requires all of:

- The caller is listed as a Discord ID string in the JSON array
  `DISCORD_ATTENDANCE_STAFF_IDS`.
- The caller has the existing Camp Power-Up access granted by
  `CAMP_ALLOWED_DISCORD_IDS` or `CAMP_ALLOWED_ROLE`.
- The request is in the configured guild and an allowed staff channel from
  `CAMP_ALLOWED_CHANNEL_IDS`. Threads inside an allowed channel are accepted;
  other server channels are denied. A private slash reply does not bypass this
  authorization.

Keep staff channels private and grant access only to the intended staff. Do not
share roster, payment, attendance, camper, family, or pickup details in public
channels, AI prompts, screenshots, logs, or test data. `/attendance` replies,
including errors, are ephemeral, but the workflow remains unavailable unless
its separate backend rollout is approved and `DISCORD_ATTENDANCE_ENABLED` is
set to `true`; merely seeing the command does not enable it. The staff workflow
requires an allowlisted staff ID in the configured guild, with authorization
also enforced by the attendance backend. Parents can only select approved
linked campers. See [private attendance](docs/DISCORD_ATTENDANCE.md).

### Safe testing

For a local documentation or code change, use automated tests such as
`go test ./...`; Discord command tests use synthetic identities and mocked
responses, not a live server or Camp Power-Up account. Do not use real camper
or family data in test prompts or fixtures. Do not test real arrival, pickup,
check-in, or release workflows, enable attendance, register commands in a live
guild, or deploy the bot as part of a safe documentation check. Existing `!`
commands, `!kit <question>`, @mentions, and DMs remain supported.

Configure these variables locally or in Railway:

| Variable | Behavior |
| --- | --- |
| `DISCORD_GUILD_ID` | Target server ID; falls back to `DISCORD_ATTENDANCE_GUILD_ID` if empty |
| `DISCORD_APPLICATION_ID` | Optional application ID; defaults to the logged-in bot's ID |
| `DISCORD_REGISTER_COMMANDS` | Empty defaults to on when a guild ID is set; `false` skips registration without disabling handlers |
| `DISCORD_ATTENDANCE_STAFF_IDS` | JSON array of approved staff ID strings; gates `/kit status`, `/kit remind`, `/ticket`, and `/camp` even with attendance disabled |
| `CAMP_ALLOWED_DISCORD_IDS` | Comma-separated Discord IDs authorized for Camp Power-Up queries |
| `CAMP_ALLOWED_ROLE` | Optional Discord role name (case-insensitive match) that grants Camp Power-Up access |
| `CAMP_ALLOWED_CHANNEL_IDS` | Comma-separated allowed staff channel IDs; empty restricts server queries to DMs |
| `KIT_OPS_CHANNEL_ID` | Optional private staff operations channel ID; enables one startup health summary and can be a reminder destination; unset disables startup posts |
| `KIT_TICKETS_CHANNEL_ID` | Optional verified staff-only text destination for staff tickets; unset disables intake; message state persists across restarts, rate limits reset |

For the camp server, the guild is `1537008248048328726` and the application is
`1537124728131817603`. Startup performs one **guild-scoped bulk overwrite** of
the entire suite, including `/attendance`, and never registers global commands.
This is idempotent, but removes any other application commands in that guild;
set `DISCORD_REGISTER_COMMANDS=false` if commands are managed externally.
Registration failures are logged without stopping the bot; restart after fixing
configuration to retry. No command registration or deployment is performed by tests.

Install the bot with the `bot` and `applications.commands` scopes. Definitions
disable DMs (guild commands are not offered in DMs) and require **Use Application
Commands**, not Administrator or Send Messages. User-ID staff allowlists cannot
be represented by Discord's default member permission bitset, so runtime
authorization remains mandatory. Keep private staff channels and attendance's
read-only parent-channel permissions unchanged. Legacy text/DM camp behavior
retains its existing authorization rules.

```bash
SLACK_BOT_TOKEN=xoxb-your-bot-token-here
SLACK_APP_TOKEN=xapp-your-app-level-token-here
GEMINI_API_KEY=your-app-level-token-here
GEMINI_MODEL=gemini-1.5-flash
```

## 💬 Usage

### In Slack Channels
```
@Kit explain quantum computing in simple terms
@Kit what are best practices for team collaboration?
@Kit help me brainstorm ideas for our project
```

### Direct Messages
```
Hello Kit! How can you help our team?
What's the best way to learn programming?
Can you review this code snippet for me?
```

## 🧩 Platform Skills and Structure

The app is intentionally split into platform adapters and shared service logic so Discord and Slack can evolve independently without duplicating the AI routing layer.

- `Slack skill`: event routing, mentions, direct messages, slash commands
- `Discord skill`: DM handling, mentions, server commands, bot status triggers
- `Shared AI boundary`: provider orchestration, session memory, fallback logic
- `MCP-ready request layer`: a thin adapter boundary for future external tool ingestion

## 🏗️ Project Structure

```
Kit/
├── main.go                 # Bootstraps Slack/Discord and app lifecycle
├── service.go              # Shared AI router, chat request boundary, session memory
├── gemini.go               # Google Gemini AI client
├── claude.go               # Anthropic Claude AI client
├── discord.go              # Discord adapter and message handling
├── go.mod                  # Go dependencies
├── Dockerfile              # Container build
├── docs/                   # Documentation
│   ├── USING_DISCORD.md    # Beginner-friendly Discord usage guide
│   ├── SLACK_SETUP.md      # Slack app configuration
│   ├── DISCORD_SETUP.md    # Discord bot configuration
│   ├── HOW_TO_USE.md       # Usage guide
│   └── development/        # Dev docs (DEVELOPMENT.md, etc.)
├── scripts/                # Shell scripts
│   ├── setup/              # Setup & configuration scripts
│   ├── debug/              # Debugging scripts
│   └── test/               # Test scripts
├── config/                 # Configuration files
│   ├── docker-compose.yml  # Docker orchestration
│   └── redis.conf          # Redis configuration
├── .env.example            # Example environment configuration
└── README.md               # Project overview and usage
```

## � Monitoring

Check if the bot is running:
```bash
ps aux | grep slack-ai-bot
```

View live logs:
```bash
tail -f bot.log
```

## �️ Development

### Making Changes
1. Edit `main.go`
2. Test: `go test ./...`
3. Build: `go build -o slack-ai-bot`
4. Deploy: `./slack-ai-bot`

### Adding Features
- Modify event handlers in `main.go`
- Add AI integrations (OpenAI, Claude, etc.)
- Implement custom commands
- Extend logging and monitoring

## � Troubleshooting

**Bot not responding?**
1. Check process: `ps aux | grep slack-ai-bot`
2. Check logs: `tail -f bot.log`
3. Verify tokens in `.env`
4. Ensure bot is invited to channel

**Build issues?**
1. Run: `go mod tidy`
2. Ensure only one `main.go` exists
3. Check Go version: `go version`

## 🎯 Status

Deployment status depends on your configured credentials and hosting service.
This repository does not establish that a live bot is online. Follow the
[deployment acceptance checks](docs/DEPLOYMENT.md#5-go-live-acceptance-checks).
`/healthz` reports HTTP liveness; `/readyz` reports chat-gateway readiness.

## 📞 Support

For issues and questions:
1. Check [HOW_TO_USE.md](HOW_TO_USE.md)
2. Review logs: `./test-bot.sh`
3. Verify Slack app configuration

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## � License

This project is licensed under the MIT License - see the LICENSE file for details.

---

**🎉 Congratulations! Your Kit AI Bot is ready to assist your team!** 

For detailed usage instructions, see [HOW_TO_USE.md](HOW_TO_USE.md).
