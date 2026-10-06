"""
mcp_hardware.py — MCP client wrapper for Groot's hardware telemetry server.

Connects to groot_mcp_server.py running on the host at port 8888 via SSE.
Docker containers reach it via the bridge gateway (172.18.0.1:8888).
Host processes reach it via localhost:8888.
"""

import json
import os
import urllib.request

# Model sizes in GB — parameter count ≠ disk size
# These are the actual sizes reported by Ollama on Groot
MODEL_SIZES_GB = {
    "qwen2.5:7b":         4.7,
    "qwen2.5-coder:7b":   4.7,
    "deepseek-r1:14b":    9.0,
    "phi3:medium":        7.9,
    "llama3.2:3b":        2.0,
    "mistral:7b-instruct":4.4,
    "llava:7b":           4.7,
    "nomic-embed-text":   0.3,
}

# Host when called from Docker vs host
_IN_DOCKER = os.path.exists("/.dockerenv")
# From inside Docker: use service name (container DNS)
# From host: use localhost
MCP_HOST   = "mcp-hardware" if _IN_DOCKER else "localhost"
MCP_BASE   = f"http://{MCP_HOST}:8889"


def _call_tool_http(tool_name: str, args: dict = {}) -> dict:
    """
    Call hardware telemetry via the REST wrapper on port 8889.

    Routes:
        get_system_health → GET /health
        get_ram           → GET /ram
        get_cpu_temp      → GET /temp
        get_ollama_models → GET /ollama

    Why a REST wrapper instead of the MCP SSE protocol directly?
        MCP SSE requires a 2-step handshake: SSE connect to get a
        session_id, then POST to /messages/?session_id=...
        A plain REST GET is one round-trip and simpler for container calls.
        The MCP server (port 8888) still runs for Claude Desktop / stdio use.
    """
    route_map = {
        "get_system_health": "/health",
        "get_ram":           "/ram",
        "get_cpu_temp":      "/temp",
        "get_ollama_models": "/ollama",
    }
    path = route_map.get(tool_name, "/health")
    try:
        req = urllib.request.Request(
            f"{MCP_BASE}{path}",
            headers={"Accept": "application/json"},
        )
        with urllib.request.urlopen(req, timeout=5) as r:
            return json.load(r)
    except Exception as e:
        return _fallback_health(str(e))


def _fallback_health(reason: str = "MCP unavailable") -> dict:
    """
    Direct /proc/meminfo read when MCP server is unreachable.
    Preserves the same return shape as get_system_health().
    """
    try:
        meminfo = {}
        with open("/proc/meminfo") as f:
            for line in f:
                if ":" in line:
                    key, val = line.split(":", 1)
                    meminfo[key.strip()] = int(val.strip().split()[0])
        available_mb = meminfo.get("MemAvailable", 0) // 1024
        total_mb     = meminfo.get("MemTotal", 0)     // 1024
        return {
            "verdict": "ready" if available_mb >= 3072 else "memory_low",
            "ram": {
                "available_mb": available_mb,
                "total_mb":     total_mb,
                "safe_to_load": available_mb >= 3072,
            },
            "temp":   {"status": "unavailable", "max_temp_c": None},
            "ollama": {"total_available": 0, "total_loaded": 0},
            "_source": "fallback",
            "_reason": reason,
        }
    except Exception:
        return {"verdict": "ready", "ram": {"safe_to_load": True}, "_source": "error"}


def get_system_health() -> dict:
    return _call_tool_http("get_system_health")


def get_ram() -> dict:
    return _call_tool_http("get_ram")


def get_cpu_temp() -> dict:
    return _call_tool_http("get_cpu_temp")


def get_ollama_models() -> dict:
    return _call_tool_http("get_ollama_models")


def is_safe_to_load(health: dict, model_name: str = "", model_size_gb: float = 7.0) -> bool:
    """
    Should the scheduler dispatch this task now?

    Prefers model_name lookup (exact GB) over the generic model_size_gb param.
    Adds 3GB safety buffer on top of the model size.
    Blocks on 'hot' CPU temperature — sustained inference at >80°C risks throttling.
    """
    # Resolve actual model size from name if available
    if model_name:
        for key, size in MODEL_SIZES_GB.items():
            if key in model_name or model_name in key:
                model_size_gb = size
                break

    ram  = health.get("ram", {})
    temp = health.get("temp", {})

    available_mb = ram.get("available_mb", 0)
    required_mb  = int(model_size_gb * 1024) + 3072   # model + 3GB buffer
    temp_status  = temp.get("status", "ok")

    if available_mb < required_mb:
        return False
    if temp_status == "hot":
        return False
    return True
