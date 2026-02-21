# Changelog

All notable changes to Kit and relevant updates from Scince are documented here.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

### Kit Changes
- *(Add upcoming changes here)*

### Scince Integration
- *(Note relevant Scince updates that affect Kit)*

---

## [2026-02-21] - Project Reorganization

### Kit Changes

#### Added
- `docs/development/GO_GUIDELINES.md` - Go coding conventions
- `docs/development/PYTHON_GUIDELINES.md` - Python coding conventions
- `scripts/test/bot_monitor.py` - Complete health monitoring script
- `scripts/setup/env_validator.py` - Environment validation template
- `scripts/debug/log_analyzer.py` - Log analysis template
- `scripts/README.md` - Scripts documentation

#### Changed
- Reorganized project structure:
  - Documentation moved to `docs/` and `docs/development/`
  - Shell scripts moved to `scripts/setup/`, `scripts/debug/`, `scripts/test/`
  - Config files moved to `config/`
- Updated `copilot-instructions.md` with new structure and guidelines references
- Updated `README.md` project structure section

### Scince Integration
- Added Kit cross-reference in Scince's `copilot-instructions.md`
- Python scripts designed to align with ISTA 130 coursework patterns
- Course connections documented in guidelines (CYBV 326, 381, 302, APCV 360)

---

## [2026-02-XX] - Initial Release

### Kit Changes
- Multi-platform AI bot (Slack + Discord)
- Google Gemini AI integration (primary)
- Anthropic Claude AI integration (fallback)
- Slack Socket Mode for real-time responses
- Discord WebSocket integration
- `/kit` slash commands for Slack
- `!` prefix commands for Discord

---

## Scince Course Reference

Kit development connects to these Scince courses:

| Course | Relevance to Kit |
|--------|-----------------|
| CYBV 326 | Network protocols, WebSocket connections |
| CYBV 381 | Incident logging, response workflows |
| CYBV 302 | Security hardening, Linux deployment |
| APCV 360 | Database patterns (Redis), SQL concepts |
| ISTA 130 | Python scripting foundations |

### How to Track Scince Updates

When Scince has updates relevant to Kit:

1. Check `Scince/CYBV 326/` for networking concepts applicable to bot connections
2. Check `Scince/CYBV 381/` for logging/monitoring patterns
3. Check `Scince/Scince/Computer_Science/Applyd/` for Python patterns to port
4. Add notes to `[Unreleased]` section above

---

## Version Format

Kit uses date-based versioning: `[YYYY-MM-DD]`

For code versions, use semantic tags: `v1.0.0`, `v1.1.0`, etc.
