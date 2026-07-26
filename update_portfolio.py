#!/usr/bin/env python3
"""
Tony's daily portfolio updater.
Calculates Groot's uptime and updates index.html, then commits and pushes.
Run daily at midnight EST via n8n.
"""
import subprocess
import re
import os
from datetime import datetime
from pathlib import Path

PORTFOLIO_DIR = Path("/home/groot/portfolio")
INDEX_HTML    = PORTFOLIO_DIR / "index.html"

def get_uptime_hours() -> int:
    """Get project uptime in hours since Groot project started Jan 2026."""
    start = datetime(2026, 2, 2)  # Groot project start — planning + learning phase
    delta = datetime.now() - start
    return int(delta.total_seconds() / 3600)

def update_html(hours: int) -> bool:
    """Update uptime values in index.html."""
    content = INDEX_HTML.read_text()
    original = content

    # Replace all patterns
    content = re.sub(r'Uptime: \d+ hours', f'Uptime: {hours} hours', content)
    content = re.sub(r'uptime \d+h and counting', f'uptime {hours}h and counting', content)
    content = re.sub(r'\d+<span style="font-size:0\.55em;">hrs</span>', f'{hours}<span style="font-size:0.55em;">hrs</span>', content)

    if content == original:
        print(f"No changes needed — already showing {hours} hours")
        return False

    INDEX_HTML.write_text(content)
    print(f"Updated portfolio — Groot uptime: {hours} hours")
    return True

def git_commit_push(hours: int) -> bool:
    """Commit and push the updated portfolio."""
    try:
        os.chdir(PORTFOLIO_DIR)

        # Check if there are changes
        result = subprocess.run(
            ["git", "diff", "--quiet", "index.html"],
            capture_output=True
        )
        if result.returncode == 0:
            print("No git changes to commit")
            return False

        # Stage, commit, push
        subprocess.run(["git", "add", "index.html"], check=True)
        subprocess.run([
            "git", "commit", "-m",
            f"chore: update Groot uptime to {hours}h [{datetime.now().strftime('%Y-%m-%d')}]"
        ], check=True)
        subprocess.run(["git", "push", "origin", "main"], check=True)
        print(f"Pushed portfolio update — {hours}h uptime")
        return True
    except subprocess.CalledProcessError as e:
        print(f"Git error: {e}")
        return False

def main():
    print(f"Tony portfolio updater — {datetime.now().isoformat()}")
    hours   = get_uptime_hours()
    changed = update_html(hours)
    if changed:
        pushed = git_commit_push(hours)
        if pushed:
            print("Portfolio live at https://yasaswinpalukuri.github.io")
    print(f"Done. Groot uptime: {hours} hours")

if __name__ == "__main__":
    main()
