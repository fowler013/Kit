#!/usr/bin/env python3
"""
Environment Validator for Kit AI Bot
=====================================
Validates that all required environment variables are set before running the bot.

Course Connections:
- ISTA 130: File I/O, string operations, functions, conditionals
- CYBV 302: Configuration validation, security best practices

Usage:
    python env_validator.py
    python env_validator.py --env-file ../.env
"""

import os
import sys

# Required environment variables for Kit to function
REQUIRED_VARS = [
    "GEMINI_API_KEY",      # Primary AI backend
]

# Optional but recommended variables
OPTIONAL_VARS = [
    "SLACK_BOT_TOKEN",     # For Slack integration
    "SLACK_APP_TOKEN",     # For Slack Socket Mode
    "DISCORD_BOT_TOKEN",   # For Discord integration
    "CLAUDE_API_KEY",      # Fallback AI backend
    "GEMINI_MODEL",        # Model selection
    "CLAUDE_MODEL",        # Model selection
]


def load_env_file(filepath: str) -> dict:
    """
    Load environment variables from a .env file.
    
    ISTA 130 Concepts: File I/O, string parsing, dictionaries
    
    TODO: Implement this function
    - Open the file at `filepath`
    - Read each line
    - Skip empty lines and comments (lines starting with #)
    - Parse KEY=VALUE pairs
    - Return a dictionary of {key: value}
    
    Hint: Use str.split('=', 1) to split on first '=' only
    """
    env_vars = {}
    
    # TODO: Your implementation here
    # Example from ISTA 130:
    # with open(filepath, 'r') as f:
    #     for line in f:
    #         # Process each line...
    
    return env_vars


def validate_required(env_vars: dict) -> list:
    """
    Check which required variables are missing.
    
    ISTA 130 Concepts: Lists, conditionals, iteration
    
    TODO: Implement this function
    - Check each variable in REQUIRED_VARS
    - If not in env_vars or empty, add to missing list
    - Return list of missing variable names
    """
    missing = []
    
    # TODO: Your implementation here
    # Hint: for var in REQUIRED_VARS:
    #           if var not in env_vars or env_vars[var] == "":
    #               ...
    
    return missing


def check_optional(env_vars: dict) -> list:
    """
    Check which optional variables are not set.
    
    ISTA 130 Concepts: Same as validate_required
    
    TODO: Implement this function
    - Similar to validate_required but for OPTIONAL_VARS
    """
    not_set = []
    
    # TODO: Your implementation here
    
    return not_set


def validate_token_format(token: str, prefix: str) -> bool:
    """
    Validate that a token starts with expected prefix.
    
    ISTA 130 Concepts: String methods (startswith)
    CYBV 302 Concepts: Input validation for security
    
    TODO: Implement this function
    - Return True if token starts with prefix
    - Return False otherwise
    
    Example: validate_token_format("xoxb-123", "xoxb-") -> True
    """
    # TODO: Your implementation here
    pass


def print_report(missing: list, not_set: list) -> None:
    """
    Print a formatted validation report.
    
    ISTA 130 Concepts: String formatting, conditionals
    
    TODO: Implement this function
    - Print ✅ if no missing required vars, else ❌ with list
    - Print ⚠️ for optional vars that aren't set
    """
    # TODO: Your implementation here
    # Hint: Use f-strings for formatting
    # print(f"✅ All {len(REQUIRED_VARS)} required variables set!")
    pass


def main():
    """Main entry point for the validator."""
    # Default path to .env file (relative to this script's location)
    script_dir = os.path.dirname(os.path.abspath(__file__))
    default_env_path = os.path.join(script_dir, "..", "..", ".env")
    
    # TODO: Add argument parsing to accept --env-file flag
    # For now, use default path
    env_path = default_env_path
    
    print(f"🔍 Validating environment from: {env_path}")
    print("-" * 50)
    
    # Load and validate
    env_vars = load_env_file(env_path)
    missing = validate_required(env_vars)
    not_set = check_optional(env_vars)
    
    print_report(missing, not_set)
    
    # Exit with error code if required vars missing
    if missing:
        sys.exit(1)
    sys.exit(0)


if __name__ == "__main__":
    main()
