# Python Guidelines for Kit

This document defines Python conventions for the Kit project. Follow these patterns for consistency across all Python scripts.

## Quick Reference

```python
#!/usr/bin/env python3
"""
Script Name - Brief Description
===============================
Longer description of what this script does.

Course Connections:
- ISTA 130: Relevant concepts
- CYBV XXX: Relevant concepts

Usage:
    python script_name.py
    python script_name.py --flag
"""

import os
import sys
# ... rest of imports

def main():
    """Main entry point."""
    pass

if __name__ == "__main__":
    main()
```

## File Structure

Every Python script should have:

1. **Shebang**: `#!/usr/bin/env python3`
2. **Module docstring**: Description, course connections, usage examples
3. **Imports**: Standard library → third-party → local (separated by blank lines)
4. **Constants**: UPPER_SNAKE_CASE at module level
5. **Functions/Classes**: With docstrings explaining purpose
6. **Main guard**: `if __name__ == "__main__":`

## Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Files | snake_case | `log_analyzer.py` |
| Functions | snake_case | `def parse_log_line():` |
| Variables | snake_case | `log_lines = []` |
| Constants | UPPER_SNAKE_CASE | `MAX_RETRIES = 3` |
| Classes | PascalCase | `class BotMonitor:` |

## Docstrings

Use this format for functions (connects to ISTA 130 documentation practices):

```python
def function_name(param1: str, param2: int = 10) -> dict:
    """
    Brief description of what the function does.
    
    Course Connections:
    - ISTA 130: Relevant concept (e.g., "File I/O")
    - CYBV 381: Relevant concept (e.g., "Log parsing")
    
    Args:
        param1: Description of first parameter
        param2: Description with default noted
        
    Returns:
        Description of return value
        
    Example:
        >>> result = function_name("test", 5)
        >>> print(result)
        {'status': 'ok'}
    """
```

## Type Hints

Use type hints for function signatures (Python 3.9+ style):

```python
# Basic types
def process(text: str, count: int) -> bool:

# Collections
def get_items() -> list[str]:
def get_mapping() -> dict[str, int]:

# Optional values
def find(name: str) -> str | None:

# Complex types
from typing import Callable
def apply(func: Callable[[str], str], data: str) -> str:
```

## Error Handling

Pattern from ISTA 130 and CYBV 302 (security-conscious error handling):

```python
def safe_file_read(filepath: str) -> str | None:
    """Read file with proper error handling."""
    try:
        with open(filepath, 'r') as f:
            return f.read()
    except FileNotFoundError:
        print(f"⚠️  File not found: {filepath}")
        return None
    except PermissionError:
        print(f"❌ Permission denied: {filepath}")
        return None
    except Exception as e:
        # Log unexpected errors but don't expose details
        print(f"❌ Error reading file: {type(e).__name__}")
        return None
```

## Logging Style

Match Kit's Go logging emoji conventions:

```python
# Use these emoji prefixes consistently
print("🔵 Initializing...")      # Init/startup
print("✅ Operation successful")  # Success
print("❌ Operation failed")      # Error
print("⚠️  Warning message")      # Warning
print("📥 Received data")         # Inbound
print("📤 Sending data")          # Outbound
print("🧠 AI processing")         # AI operations
print("📊 Analysis complete")     # Analysis/reports
```

For more complex scripts, use the `logging` module:

```python
import logging

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s %(levelname)s %(message)s'
)
logger = logging.getLogger(__name__)

logger.info("✅ Bot started")
logger.error("❌ Connection failed")
```

## Project Paths

Always use `os.path` for cross-platform compatibility:

```python
import os

# Get script's directory
script_dir = os.path.dirname(os.path.abspath(__file__))

# Build paths relative to script location
project_root = os.path.join(script_dir, "..", "..")
env_path = os.path.join(project_root, ".env")
log_path = os.path.join(project_root, "bot.log")

# Check existence before using
if os.path.exists(env_path):
    # ...
```

## Course Integration Comments

When implementing features, add comments linking to coursework:

```python
def analyze_network_log(log_line: str) -> dict:
    """
    Parse network-related log entries.
    
    CYBV 326 Connection:
    This function applies concepts from Week 3 (Transport Layer)
    - TCP/UDP port identification
    - Connection state tracking
    See: Scince/CYBV 326/Lectures/Week3_Transport.md
    """
    # Implementation using networking knowledge...
```

## Dependencies

Keep dependencies minimal. For scripts in Kit:

```python
# Standard library - always available
import os, sys, re, json, time
from datetime import datetime
from collections import Counter

# Optional third-party - check before using
try:
    import requests
    HAS_REQUESTS = True
except ImportError:
    HAS_REQUESTS = False
    
if HAS_REQUESTS:
    # Use requests...
else:
    print("⚠️  Install requests: pip install requests")
```

Document any required packages in `scripts/README.md`.

## Testing

For simple scripts, include a basic self-test:

```python
def _self_test():
    """Basic self-test for development."""
    print("🧪 Running self-test...")
    
    # Test key functions
    assert parse_line("test") is not None
    assert validate_config({}) == False
    
    print("✅ All tests passed")

if __name__ == "__main__":
    if "--test" in sys.argv:
        _self_test()
    else:
        main()
```

## Reference Examples

| Concept | Reference File | Course Link |
|---------|---------------|-------------|
| Classes & OOP | `scripts/test/bot_monitor.py` | ISTA 130 |
| File I/O | `scripts/setup/env_validator.py` | ISTA 130 |
| String parsing | `scripts/debug/log_analyzer.py` | ISTA 130, CYBV 381 |
| HTTP requests | `scripts/test/bot_monitor.py` | CYBV 326 |

## Scince Repository Resources

Your Scince repo has Python foundations to reference:

- `Scince/Scince/Computer_Science/Python_Programming/` - Core Python concepts
- `Scince/Scince/Computer_Science/Applyd/src/python_foundations/` - Applied examples
- `Scince/ISTA 130/` - Course materials and exercises

When stuck on implementation, check these resources first.
