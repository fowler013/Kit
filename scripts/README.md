# Kit Scripts

Utility scripts for managing, debugging, and testing the Kit AI Bot.

## Structure

```
scripts/
├── setup/              # Setup and configuration
│   ├── *.sh            # Bash setup scripts
│   └── env_validator.py    # [TODO] Validate .env configuration
├── debug/              # Debugging and diagnostics
│   ├── *.sh            # Bash debug scripts
│   └── log_analyzer.py     # [TODO] Analyze bot.log files
├── test/               # Testing and monitoring
│   ├── *.sh            # Bash test scripts
│   └── bot_monitor.py      # [COMPLETE] Monitor bot health
├── bot-service.sh      # Service management
└── restart-bot.sh      # Quick restart
```

## Python Scripts

### Learning Path

These Python scripts are designed as learning exercises connecting to coursework:

| Script | Status | Course Connections |
|--------|--------|-------------------|
| `bot_monitor.py` | ✅ Complete | ISTA 130, CYBV 326 |
| `env_validator.py` | 📝 Template | ISTA 130, CYBV 302 |
| `log_analyzer.py` | 📝 Template | ISTA 130, CYBV 381 |

### Using the Templates

1. **Study `bot_monitor.py` first** - It's a complete working example
2. **Fill in the TODO functions** - Use patterns from the example
3. **Reference your Scince notes** - `python_foundations` has relevant code

### Running Python Scripts

```bash
# From Kit root directory:
python scripts/test/bot_monitor.py
python scripts/test/bot_monitor.py --continuous --interval 30

# After implementing:
python scripts/setup/env_validator.py
python scripts/debug/log_analyzer.py --errors-only
```

### Dependencies

```bash
# Optional but recommended for bot_monitor.py HTTP checks:
pip install requests
```

## Bash Scripts Reference

### Setup Scripts (`setup/`)
- `setup-integrations.sh` - Configure Slack/Discord integrations
- `setup-labels.sh` - Set up GitHub labels
- `verify-setup.sh` - Verify configuration is correct

### Debug Scripts (`debug/`)
- `debug-slack.sh` - Debug Slack connection issues
- `diagnose-app.sh` - General diagnostics
- `monitor-events.sh` - Watch incoming events

### Test Scripts (`test/`)
- `test-bot.sh` - Basic bot functionality test
- `comprehensive-test.sh` - Full test suite
