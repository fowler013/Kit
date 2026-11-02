# Private Discord attendance (disabled pending rollout)

Kit is a private Discord interface to the CampPowerUp registration service's
`/api/discord/attendance/v1` contract. CampPowerUp, not Kit or an AI model,
owns persistent attendance, family linkage, staff authorization and pickup
rosters. This implementation does not deploy or enable the feature, register
Discord commands, modify channels/roles, or contact production during tests.
There is no photo-ID collection or WhatsApp integration.

## Parent and staff experience

After a separately approved deployment and guild-scoped command registration:

```text
/attendance parent session:thanksgiving-2026 date:2026-11-23
/attendance staff session:thanksgiving-2026 date:2026-11-23
/attendance staff session:thanksgiving-2026 date:2026-11-23 camper:<private-opaque-handle>
```

Every response, error, handle, pending list and pickup name is **ephemeral**,
visible only to the acting user. The bot defers privately before contacting
the backend and edits that private response. A failed private acknowledgement
or edit never falls back to a public channel, DM, announcement, or an AI
provider. Mentions are suppressed. DMs are deliberately unsupported because
the API requires the actual interaction's approved guild and member identity.

Parents explicitly select from their approved linked campers, even when only
one is linked. Authorized display names appear as ephemeral labels when the
backend provides them. Duplicate names receive distinct numbered labels;
missing names (including older backend responses) use the explicit
`Camper name unavailable` fallback rather than exposing an opaque ID.
Component values and all backend requests continue to use opaque IDs. Multiple
campers are never silently reduced to the first. Names are sanitized for
Discord formatting and mentions before display.
Parents inspect status and can **request arrival** or **request pickup**.
A request never checks a child in or out and never authorizes release.
No links means contacting camp leadership, not self-linking.
A Verified Parent role alone grants no attendance access.

Allowlisted staff privately select pending requests or supply a handle obtained
through their private authorized staff workflow. Pending labels and status
responses use authorized display names when available; missing names use the
same generic fallback. Staff can confirm physical
arrival directly, including without a parent request. To release a child:

The backend may return an optional, nullable `display_name` string on each
`/campers` item, `/status` and `/staff-status` response, each `/pending` item,
and successful request or confirmation status objects. Kit treats absent,
`null`, empty, or formatting-only values as unavailable and never substitutes
the camper ID in a user-facing label.

1. Review fresh staff status and the current active authorized pickup roster.
2. Select the authorized pickup person privately.
3. Manually match the person physically collecting the camper to that roster.
   Resolve any uncertainty privately with the verified parent before release.
4. Explicitly attest to the manual roster check and physical release.

No photo ID is required. A request, role, roster name, or button alone is not
permission to release a child. Kit sends the selected `pickup_id` and explicit
`roster_verified:true` only from the final staff confirmation control.
The backend rechecks the active roster, staff allowlist and state version
atomically. Emergency contacts are not automatically authorized pickup people.
Link and roster administration remains outside Kit.

The normal states are `not_arrived`, `arrival_requested`, `present`,
`pickup_requested`, and `released`. Only staff confirmations set `present`
or `released`. Re-entry and corrections are intentionally unavailable;
escalate to camp leadership rather than creating an unaudited reversal.

## Configuration and trust boundary

Unset `DISCORD_ATTENDANCE_ENABLED` by default. Only exact `true` enables the
client. Invalid enabled configuration prevents bot construction with a generic
error, never printing credentials. The command handler still returns a private
unavailable response if a previously registered command reaches a disabled bot.

| Variable | Kit requirement |
| --- | --- |
| `DISCORD_ATTENDANCE_ENABLED` | Exact `true`, only after rollout approval |
| `DISCORD_ATTENDANCE_API_BASE_URL` | Registration service HTTPS origin, no credentials, path, query or fragment |
| `DISCORD_ATTENDANCE_PARENT_TOKEN` | Distinct random scoped bearer credential, at least 32 characters |
| `DISCORD_ATTENDANCE_STAFF_TOKEN` | Separate random scoped bearer credential, at least 32 characters |
| `DISCORD_ATTENDANCE_GUILD_ID` | Exact approved guild ID string |
| `DISCORD_ATTENDANCE_STAFF_IDS` | JSON array of approved staff Discord ID strings |
| `DISCORD_ATTENDANCE_SESSIONS` | Same session object as CampPowerUp |

Nonsecret session example:

```json
{
  "thanksgiving-2026": {
    "label": "Camp Power-Up Thanksgiving 2026 - November 23-24, 2026",
    "dates": ["2026-11-23", "2026-11-24"]
  }
}
```

Camp runs November 23-24, 2026, 10am-3pm Central, grades K-8. Either configured
date may be read. Kit and the backend permit writes only on today's configured
date in `America/Chicago`, with no 10am cutoff. Use new session configuration
for future camps; do not broaden the dates to work around production gates.

Service credentials permit asserting users in their scope: protect them in a
secret manager and restrict backend network access where possible. Kit derives
`X-Discord-Guild-ID` and `X-Discord-User-ID` only from authenticated Gateway
interactions, never command options, messages, roles or LLM arguments.
The local staff list narrows access; it cannot grant access absent the backend
allowlist. Parent requests always use the parent token, never the staff token.
These settings are separate from legacy `CAMP_*` registration/AI settings.
No attendance payload, pickup name, handle, token, or transport detail is logged.
Cross-origin and same-origin HTTP redirects are refused.

## Concurrency, failures and private controls

Controls are random, actor/guild-bound, expire after ten minutes, and are
single-use. Lists paginate beyond Discord's 25-entry limit. Each page re-fetches
the authoritative list; selection and actions recheck backend access. Staff
confirmations fetch fresh status and require the version displayed before the
attestation to remain unchanged; release also refreshes the active roster.
A stale version or revoked selection requires reopening and reevaluation,
not blind confirmation.

POST `Idempotency-Key` is the actual Discord interaction ID. Kit does not
automatically retry writes or generate replacement keys. Any future operator
replay must preserve the identical actor, endpoint, body and interaction key.
Backend replay results may be historical; obtain fresh status before the next
confirmation. A transport failure, malformed success or unrecognized server
failure is an **unknown outcome**, never success. Refresh status and reconcile
with leadership before submitting another write. Known contract errors are
mapped to generic private instructions, not raw server bodies.

Kit stores only short-lived control state in memory. Restarting invalidates
controls but does not lose attendance, which persists in CampPowerUp. Keep
one active Kit process; multi-replica delivery/rollout must be reviewed before
changing that operational limit.

## Rollout gates (not performed by this change)

Keep the feature off until **all** of these are separately approved and verified:

1. CampPowerUp's backend change is merged, its final synthetic persistence
   hardening passes, and the registration service is deployed over HTTPS with
   the persistent existing SQLite registration database pinned correctly.
   Verify volume persistence, secure admin signing secret, schema and backups
   per that repository's `docs/development/DISCORD_ATTENDANCE.md`; the old
   admin roll-call page is not pickup/release authorization.
2. Leadership approves exact staff accounts and independently verifies family
   Discord ownership through trusted registration/contact channels. Create
   and revoke links and pickup rosters through the private backend admin
   session/CSRF workflow, not Kit or a parent's assertion.
3. Configure distinct scoped credentials and identical session/guild/allowlist
   settings privately in both services. Do not copy legacy admin credentials.
4. Separately register the guild-scoped `/attendance` command defined by
   `attendanceCommand()` in `discord_attendance.go`. Its `parent` and `staff`
   subcommands require string `session` and `date`; only `staff` accepts an
   optional string `camper`. The definition disables DMs and uses
   `Use Application Commands` permission, not `Send Messages` or Administrator.
   `go run . --print-attendance-command` prints the exact registration JSON
   locally, without loading `.env`, starting the bot or making network calls.
   Submit that definition to the approved guild only in the separately
   authorized operator registration step. Registration is deliberately not
   automatic at startup.
5. Test using separate synthetic linked-parent, unrelated-parent and staff
   accounts in the actual deployment, including revocation, expired controls,
   backend outages, stale state, and pickup revocation. Verify every response
   is private, including errors and names, before enabling real use.
6. Preserve parent-first read-only channel overwrites. Allow only the intended
   users **View Channel** and **Use Application Commands** where appropriate;
   do not enable public **Send Messages**, change posting restrictions, or
   broaden legacy staff-roster access just to make slash commands available.
   Privately verify Discord client behavior in that exact read-only channel.

Command registration, Discord permissions, cloud variables, deployment,
production-data checks and feature enablement are separate operator actions.
No production readiness is claimed by mock tests alone.

## Local synthetic checks

No real Discord account or CampPowerUp data is needed. Tests use synthetic
HTTP servers and a fake Discord REST transport; no bot connection is opened.

```powershell
$env:GOCACHE = 'D:\DevCache\go-build'
$env:GOTMPDIR = 'D:\DevCache\gotmp'
go test -run TestAttendance ./...
go test ./...
go vet ./...
go test -race ./...
```

The suite covers scope/actor headers, explicit multi-camper selection,
request-versus-confirmation semantics, fresh state and pickup attestation,
revocation, controls, paging, configuration/date gates, failures without
payload leakage, no blind write retry, redirects, and ephemeral-only replies.
