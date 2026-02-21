#!/usr/bin/env python3
"""
Bot Monitor for Kit AI Bot - COMPLETE EXAMPLE
==============================================
Monitors the health and status of the running Kit bot.

This is a COMPLETE WORKING EXAMPLE you can use as reference for implementing
the other scripts. Study the patterns here and apply them.

Course Connections:
- ISTA 130: Functions, classes, error handling, HTTP requests
- CYBV 326: Network requests, health monitoring, HTTP protocols
- CYBV 381: System monitoring for incident detection

Usage:
    python bot_monitor.py
    python bot_monitor.py --continuous
    python bot_monitor.py --interval 30
"""

import os
import sys
import time
import subprocess
from datetime import datetime

# Try to import requests, handle if not installed
try:
    import requests
    HAS_REQUESTS = True
except ImportError:
    HAS_REQUESTS = False
    print("⚠️  'requests' library not installed. HTTP checks disabled.")
    print("   Install with: pip install requests")


class BotMonitor:
    """
    Monitor class for Kit AI Bot.
    
    ISTA 130 Concepts: Classes, __init__, methods, attributes
    """
    
    def __init__(self, health_url: str = "http://localhost:8080/health"):
        """
        Initialize the monitor.
        
        Args:
            health_url: URL of the bot's health endpoint
        """
        self.health_url = health_url
        self.process_name = "kit-ai-bot"
        self.checks_performed = 0
        self.last_status = None
    
    def check_process_running(self) -> dict:
        """
        Check if the bot process is running.
        
        ISTA 130 Concepts: Subprocess, string parsing
        CYBV 302 Concepts: Process monitoring on Linux
        
        Returns:
            dict with keys: running (bool), pid (int or None), details (str)
        """
        result = {
            "running": False,
            "pid": None,
            "details": "Process not found"
        }
        
        try:
            # Cross-platform process check
            if sys.platform == "win32":
                # Windows: use tasklist
                output = subprocess.check_output(
                    ["tasklist", "/FI", f"IMAGENAME eq {self.process_name}.exe"],
                    text=True,
                    stderr=subprocess.DEVNULL
                )
                if self.process_name in output:
                    result["running"] = True
                    result["details"] = "Process found via tasklist"
            else:
                # Linux/Mac: use pgrep
                output = subprocess.check_output(
                    ["pgrep", "-f", self.process_name],
                    text=True,
                    stderr=subprocess.DEVNULL
                )
                pids = output.strip().split('\n')
                if pids and pids[0]:
                    result["running"] = True
                    result["pid"] = int(pids[0])
                    result["details"] = f"Process running with PID {pids[0]}"
        except subprocess.CalledProcessError:
            # Process not found - this is expected if bot isn't running
            pass
        except Exception as e:
            result["details"] = f"Error checking process: {e}"
        
        return result
    
    def check_health_endpoint(self) -> dict:
        """
        Check the bot's HTTP health endpoint.
        
        ISTA 130 Concepts: HTTP requests, JSON parsing, exception handling
        CYBV 326 Concepts: HTTP protocol, health checks, network monitoring
        
        Returns:
            dict with keys: healthy (bool), response_time_ms (float), details (str)
        """
        result = {
            "healthy": False,
            "response_time_ms": None,
            "details": "Health check not performed"
        }
        
        if not HAS_REQUESTS:
            result["details"] = "requests library not available"
            return result
        
        try:
            start_time = time.time()
            response = requests.get(self.health_url, timeout=5)
            end_time = time.time()
            
            result["response_time_ms"] = round((end_time - start_time) * 1000, 2)
            
            if response.status_code == 200:
                result["healthy"] = True
                result["details"] = f"HTTP 200 OK in {result['response_time_ms']}ms"
            else:
                result["details"] = f"HTTP {response.status_code}"
                
        except requests.exceptions.ConnectionError:
            result["details"] = "Connection refused - bot may not be running"
        except requests.exceptions.Timeout:
            result["details"] = "Request timed out after 5 seconds"
        except Exception as e:
            result["details"] = f"Error: {e}"
        
        return result
    
    def check_env_file(self) -> dict:
        """
        Check if .env file exists and has required variables.
        
        ISTA 130 Concepts: File existence check, file reading
        CYBV 302 Concepts: Configuration validation
        
        Returns:
            dict with keys: exists (bool), has_required (bool), details (str)
        """
        result = {
            "exists": False,
            "has_required": False,
            "details": ".env file not found"
        }
        
        # Look for .env in project root (two levels up from scripts/test/)
        script_dir = os.path.dirname(os.path.abspath(__file__))
        env_path = os.path.join(script_dir, "..", "..", ".env")
        
        if os.path.exists(env_path):
            result["exists"] = True
            
            # Check for at least one API key
            try:
                with open(env_path, 'r') as f:
                    content = f.read()
                    has_gemini = "GEMINI_API_KEY=" in content
                    has_slack = "SLACK_BOT_TOKEN=" in content
                    has_discord = "DISCORD_BOT_TOKEN=" in content
                    
                    if has_gemini and (has_slack or has_discord):
                        result["has_required"] = True
                        result["details"] = "Required variables found"
                    else:
                        result["details"] = "Missing some required variables"
            except Exception as e:
                result["details"] = f"Error reading .env: {e}"
        
        return result
    
    def run_all_checks(self) -> dict:
        """
        Run all health checks and compile results.
        
        ISTA 130 Concepts: Combining results, dictionaries
        
        Returns:
            dict with all check results and overall status
        """
        self.checks_performed += 1
        timestamp = datetime.now().isoformat()
        
        results = {
            "timestamp": timestamp,
            "check_number": self.checks_performed,
            "process": self.check_process_running(),
            "health_endpoint": self.check_health_endpoint(),
            "env_file": self.check_env_file(),
            "overall_healthy": False,
        }
        
        # Overall health: process running OR health endpoint responding
        results["overall_healthy"] = (
            results["process"]["running"] or 
            results["health_endpoint"]["healthy"]
        )
        
        self.last_status = results
        return results
    
    def format_report(self, results: dict) -> str:
        """
        Format check results as a readable report.
        
        ISTA 130 Concepts: String formatting, f-strings, conditionals
        
        Args:
            results: dict from run_all_checks()
            
        Returns:
            Formatted string report
        """
        lines = []
        lines.append("=" * 50)
        lines.append(f"🤖 Kit Bot Status Report")
        lines.append(f"📅 {results['timestamp']}")
        lines.append("=" * 50)
        
        # Overall status
        status_emoji = "✅" if results["overall_healthy"] else "❌"
        lines.append(f"\n{status_emoji} Overall Status: {'HEALTHY' if results['overall_healthy'] else 'UNHEALTHY'}")
        
        # Process check
        proc = results["process"]
        proc_emoji = "✅" if proc["running"] else "❌"
        lines.append(f"\n📦 Process Check:")
        lines.append(f"   {proc_emoji} {proc['details']}")
        
        # Health endpoint
        health = results["health_endpoint"]
        health_emoji = "✅" if health["healthy"] else "❌"
        lines.append(f"\n🌐 Health Endpoint:")
        lines.append(f"   {health_emoji} {health['details']}")
        
        # Environment file
        env = results["env_file"]
        env_emoji = "✅" if env["has_required"] else "⚠️"
        lines.append(f"\n📄 Environment File:")
        lines.append(f"   {env_emoji} {env['details']}")
        
        lines.append("\n" + "=" * 50)
        return "\n".join(lines)


def main():
    """
    Main entry point for bot monitor.
    
    This demonstrates:
    - Argument parsing (basic)
    - Class instantiation
    - Continuous monitoring loop
    """
    # Basic argument handling
    continuous = "--continuous" in sys.argv or "-c" in sys.argv
    
    # Get interval if specified
    interval = 60  # default: check every 60 seconds
    for i, arg in enumerate(sys.argv):
        if arg in ("--interval", "-i") and i + 1 < len(sys.argv):
            try:
                interval = int(sys.argv[i + 1])
            except ValueError:
                print(f"⚠️  Invalid interval, using default: {interval}s")
    
    # Create monitor instance
    monitor = BotMonitor()
    
    if continuous:
        print(f"🔄 Starting continuous monitoring (interval: {interval}s)")
        print("   Press Ctrl+C to stop\n")
        
        try:
            while True:
                results = monitor.run_all_checks()
                print(monitor.format_report(results))
                print(f"\n⏳ Next check in {interval} seconds...\n")
                time.sleep(interval)
        except KeyboardInterrupt:
            print("\n\n👋 Monitoring stopped by user")
    else:
        # Single check
        results = monitor.run_all_checks()
        print(monitor.format_report(results))
        
        # Exit with appropriate code
        sys.exit(0 if results["overall_healthy"] else 1)


if __name__ == "__main__":
    main()
