"""
groot_mcp_rest.py — Self-contained REST wrapper exposing hardware telemetry.
Runs as a Docker service with /proc and /sys mounted read-only from host.

Routes:
    GET /health  → combined RAM + temp + Ollama snapshot
    GET /ram     → RAM stats from /proc/meminfo
    GET /temp    → CPU temp from /sys/class/thermal
    GET /ollama  → Ollama model list

Why Docker service instead of host process?
    Container-to-container networking is reliable — no UFW or iptables
    DOCKER-USER issues. agent_service reaches this via http://mcp-hardware:8889
    on the Docker bridge. /proc and /sys are mounted read-only so the
    container reads real host hardware metrics without privileged mode.
"""

import json
import sys
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path


def get_ram() -> dict:
    meminfo = {}
    for line in Path("/proc/meminfo").read_text().splitlines():
        if ":" in line:
            key, val = line.split(":", 1)
            meminfo[key.strip()] = int(val.strip().split()[0])
    total_kb     = meminfo.get("MemTotal", 0)
    available_kb = meminfo.get("MemAvailable", 0)
    used_kb      = total_kb - available_kb
    total_mb     = total_kb     // 1024
    available_mb = available_kb // 1024
    used_mb      = used_kb      // 1024
    used_pct     = round(used_mb / total_mb * 100, 1) if total_mb else 0
    return {
        "total_mb":     total_mb,
        "available_mb": available_mb,
        "used_mb":      used_mb,
        "used_pct":     used_pct,
        "safe_to_load": available_mb >= 3072,
    }


def get_cpu_temp() -> dict:
    base  = Path("/sys/class/thermal")
    zones = []
    if not base.exists():
        return {"zones": [], "max_temp_c": None, "status": "unavailable"}
    for zone_path in sorted(base.glob("thermal_zone*/temp")):
        try:
            temp_c = int(zone_path.read_text().strip()) / 1000.0
            zones.append({"zone": zone_path.parent.name, "temp_c": round(temp_c, 1)})
        except (ValueError, OSError):
            continue
    if not zones:
        return {"zones": [], "max_temp_c": None, "status": "unavailable"}
    max_temp = max(z["temp_c"] for z in zones)
    status   = "ok" if max_temp < 60 else "warm" if max_temp < 80 else "hot"
    return {"zones": zones, "max_temp_c": max_temp, "status": status}


def get_ollama_models() -> dict:
    try:
        with urllib.request.urlopen(
            "http://host.docker.internal:11434/api/tags", timeout=5
        ) as r:
            tags = json.load(r)
        available = [
            {"name": m["name"], "size_gb": round(m.get("size", 0) / 1e9, 2)}
            for m in tags.get("models", [])
        ]
        try:
            with urllib.request.urlopen(
                "http://host.docker.internal:11434/api/ps", timeout=5
            ) as r:
                ps = json.load(r)
            loaded = [
                {"name": m["name"], "size_gb": round(m.get("size", 0) / 1e9, 2)}
                for m in ps.get("models", [])
            ]
        except Exception:
            loaded = []
        return {
            "available":       available,
            "loaded":          loaded,
            "total_available": len(available),
            "total_loaded":    len(loaded),
        }
    except Exception as e:
        return {"error": str(e), "available": [], "loaded": [],
                "total_available": 0, "total_loaded": 0}


def get_system_health() -> dict:
    ram    = get_ram()
    temp   = get_cpu_temp()
    ollama = get_ollama_models()
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
    return {"ram": ram, "temp": temp, "ollama": ollama, "verdict": verdict}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def _json(self, data: dict, status: int = 200):
        body = json.dumps(data).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", len(body))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        routes = {
            "/health": get_system_health,
            "/ram":    get_ram,
            "/temp":   get_cpu_temp,
            "/ollama": get_ollama_models,
        }
        fn = routes.get(self.path)
        if fn:
            self._json(fn())
        else:
            self._json({"error": "not found", "routes": list(routes)}, 404)


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8889
    print(f"MCP hardware REST on 0.0.0.0:{port}", flush=True)
    HTTPServer(("0.0.0.0", port), Handler).serve_forever()
