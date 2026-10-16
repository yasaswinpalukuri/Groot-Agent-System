#!/usr/bin/env python3
"""
groot_mcp_server.py — MCP server exposing Groot's hardware telemetry as tools.

Tools exposed:
  - get_ram()          → RAM stats from /proc/meminfo
  - get_cpu_temp()     → CPU temperature from /sys/class/thermal
  - get_ollama_models()→ Loaded/available models from Ollama API
  - get_system_health()→ Combined snapshot: RAM + temp + model count

Run:  python3 groot_mcp_server.py
Transport: stdio (MCP default — no port needed)

Why stdio over HTTP transport?
  This process runs on the host, not in Docker. stdio means the MCP
  client spawns this process directly and communicates over stdin/stdout.
  No port conflicts, no auth needed, process lifecycle tied to the client.
"""

import json
import urllib.request
from pathlib import Path

from mcp.server.fastmcp import FastMCP

# ── Server definition ──────────────────────────────────────────────────────────
# FastMCP is the high-level wrapper — it handles JSON-RPC framing,
# tool registration, and the stdio transport loop.
mcp = FastMCP("groot-hardware", host="0.0.0.0", port=8888)


# ── Tool 1: RAM ────────────────────────────────────────────────────────────────
@mcp.tool()
def get_ram() -> dict:
    """
    Read RAM stats from /proc/meminfo.

    Returns:
        total_mb      — total physical RAM in MB
        available_mb  — available RAM in MB (MemAvailable, not MemFree;
                        MemAvailable accounts for reclaimable cache)
        used_mb       — used RAM in MB
        used_pct      — used RAM as a percentage (0-100)
        safe_to_load  — True when ≥3 GB available (Groot's scheduler threshold)

    Why /proc/meminfo and not `free`?
        /proc/meminfo is the authoritative source — `free` reads it too.
        Parsing it directly avoids spawning a subprocess and is always available.

    Why MemAvailable and not MemFree?
        MemFree = completely unused pages.
        MemAvailable = what the kernel estimates is actually available for new
        allocations, including pages it can reclaim from buffers and cache.
        Ollama cares about MemAvailable, not MemFree.
    """
    meminfo: dict[str, int] = {}
    for line in Path("/proc/meminfo").read_text().splitlines():
        if ":" in line:
            key, val = line.split(":", 1)
            # values are in kB — strip the unit
            meminfo[key.strip()] = int(val.strip().split()[0])

    total_kb     = meminfo.get("MemTotal", 0)
    available_kb = meminfo.get("MemAvailable", 0)
    used_kb      = total_kb - available_kb

    total_mb     = total_kb     // 1024
    available_mb = available_kb // 1024
    used_mb      = used_kb      // 1024
    used_pct     = round(used_mb / total_mb * 100, 1) if total_mb else 0

    return {
        "total_mb":      total_mb,
        "available_mb":  available_mb,
        "used_mb":       used_mb,
        "used_pct":      used_pct,
        "safe_to_load":  available_mb >= 3072,   # 3 GB threshold
    }


# ── Tool 2: CPU temperature ────────────────────────────────────────────────────
@mcp.tool()
def get_cpu_temp() -> dict:
    """
    Read CPU temperature from /sys/class/thermal thermal zones.

    Returns:
        zones     — list of {zone, temp_c} for every readable zone
        max_temp_c— highest temperature across all zones
        status    — 'ok' | 'warm' | 'hot' | 'unavailable'

    Why thermal zones and not lm-sensors?
        lm-sensors requires kernel module loading (modprobe coretemp) and is
        not always present. /sys/class/thermal is a standard kernel interface
        available on any modern Linux — no extra packages needed.

    Temperature thresholds (Intel NUC / Lenovo M720Q range):
        < 60°C  → ok   (idle / light load)
        60-80°C → warm (sustained inference)
        > 80°C  → hot  (throttling risk — scheduler should back off)
    """
    base = Path("/sys/class/thermal")
    zones = []

    if not base.exists():
        return {"zones": [], "max_temp_c": None, "status": "unavailable"}

    for zone_path in sorted(base.glob("thermal_zone*/temp")):
        try:
            raw = int(zone_path.read_text().strip())
            temp_c = raw / 1000.0          # kernel stores millidegrees Celsius
            zone_name = zone_path.parent.name
            zones.append({"zone": zone_name, "temp_c": round(temp_c, 1)})
        except (ValueError, OSError):
            continue

    if not zones:
        return {"zones": [], "max_temp_c": None, "status": "unavailable"}

    max_temp = max(z["temp_c"] for z in zones)
    if max_temp < 60:
        status = "ok"
    elif max_temp < 80:
        status = "warm"
    else:
        status = "hot"

    return {
        "zones":      zones,
        "max_temp_c": max_temp,
        "status":     status,
    }


# ── Tool 3: Ollama models ──────────────────────────────────────────────────────
@mcp.tool()
def get_ollama_models() -> dict:
    """
    Query Ollama's REST API for available and currently loaded models.

    Returns:
        available — list of {name, size_gb} for all pulled models
        loaded    — list of {name, size_gb} for models currently in VRAM/RAM
        total_available — count of available models
        total_loaded    — count of loaded models

    Why two lists?
        Available = models on disk (can be loaded in ~seconds).
        Loaded = models in memory right now (respond in milliseconds).
        Groot's scheduler checks loaded to avoid the cold-start penalty.

    Why HTTP and not subprocess `ollama list`?
        The Ollama REST API is stable and returns structured JSON.
        subprocess output format can change between Ollama versions.
    """
    def _get(path: str) -> dict:
        url = f"http://localhost:11434{path}"
        req = urllib.request.Request(url, headers={"Accept": "application/json"})
        with urllib.request.urlopen(req, timeout=5) as r:
            return json.load(r)

    # Available models (pulled to disk)
    try:
        tags = _get("/api/tags")
        available = [
            {
                "name":    m["name"],
                "size_gb": round(m.get("size", 0) / 1e9, 2),
            }
            for m in tags.get("models", [])
        ]
    except Exception as e:
        return {"error": str(e), "available": [], "loaded": [], "total_available": 0, "total_loaded": 0}

    # Currently loaded models (in memory — Ollama /api/ps endpoint)
    try:
        ps = _get("/api/ps")
        loaded = [
            {
                "name":    m["name"],
                "size_gb": round(m.get("size", 0) / 1e9, 2),
            }
            for m in ps.get("models", [])
        ]
    except Exception:
        loaded = []   # /api/ps may not exist on older Ollama versions

    return {
        "available":       available,
        "loaded":          loaded,
        "total_available": len(available),
        "total_loaded":    len(loaded),
    }


# ── Tool 4: Combined health snapshot ──────────────────────────────────────────
@mcp.tool()
def get_system_health() -> dict:
    """
    Single call that returns RAM, CPU temperature, and Ollama model state.

    Why a combined tool?
        When the supervisor agent checks whether it's safe to dispatch a task,
        it needs all three signals together. Calling three tools separately
        means three round-trips and three LLM tool-call tokens. One combined
        call is cheaper and atomic — the three readings are taken close together.

    Returns:
        ram     — same as get_ram()
        temp    — same as get_cpu_temp()
        ollama  — same as get_ollama_models()
        verdict — 'ready' | 'warm' | 'memory_low' | 'hot_and_low'
    """
    ram    = get_ram()
    temp   = get_cpu_temp()
    ollama = get_ollama_models()

    # Derive a single actionable verdict for the scheduler
    mem_ok  = ram["safe_to_load"]
    temp_ok = temp["status"] in ("ok", "warm", "unavailable")

    if mem_ok and temp_ok:
        verdict = "ready"
    elif mem_ok and not temp_ok:
        verdict = "hot"
    elif not mem_ok and temp_ok:
        verdict = "memory_low"
    else:
        verdict = "hot_and_low"

    return {
        "ram":     ram,
        "temp":    temp,
        "ollama":  ollama,
        "verdict": verdict,
    }


# ── Entry point ────────────────────────────────────────────────────────────────
if __name__ == "__main__":
    import sys as _sys
    if "--http" in _sys.argv:
        # SSE/HTTP transport on port 8888 — lets Docker containers reach
        # this server over http://172.18.0.1:8888 (Docker bridge gateway)
        # Port 8888 is free on Groot (8000=agent_service, 8001=chromadb)
        mcp.run(transport="sse")
    else:
        # stdio transport — default for direct subprocess clients
        mcp.run()
