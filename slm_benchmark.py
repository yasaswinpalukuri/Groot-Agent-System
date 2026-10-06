#!/usr/bin/env python3
"""
slm_benchmark.py — SLM inference benchmark on Pi-class hardware limits.

Measures tokens/sec, peak RAM, and CPU temperature for small language models
using Ollama's REST API. Designed to run on Groot (Lenovo M720Q, Ubuntu 22.04)
with Docker cgroup limits emulating a Raspberry Pi 4 (4 cores, 4GB RAM).

Usage:
    python3 slm_benchmark.py                    # run all benchmarks
    python3 slm_benchmark.py --model llama3.2:1b # single model

Why this matters:
    SLMs (sub-2B parameter models) are the practical choice for edge devices
    like Raspberry Pi where RAM is 4-8GB and there's no GPU. Measuring actual
    throughput under realistic constraints — not theoretical FLOPS — is what
    tells you whether a model is viable for a given workload.

Measurement methodology:
    - Uses Ollama's /api/generate with stream=True to count tokens as they arrive
    - eval_count / eval_duration from Ollama's final response gives server-side t/s
    - Wall-clock t/s computed independently as cross-check
    - RAM sampled from MCP hardware endpoint before and after each run
    - CPU temperature sampled from MCP hardware endpoint (host thermal zones)
    - 3 prompts per model, results averaged to reduce variance
    - Hardware limits: applied via Docker cgroups (--cpus=4 --memory=4g)
      NOTE: /proc/meminfo and os.cpu_count() show host values because Docker
      mounts the host's /proc. Limits are enforced by the kernel cgroup
      scheduler, not by the values reported to the process.
"""

import argparse
import json
import sys
import time
import urllib.request
from datetime import datetime
from pathlib import Path

OLLAMA_BASE = "http://localhost:11434"
MCP_BASE    = "http://localhost:8889"   # Groot MCP REST wrapper

MODELS = [
    {"name": "llama3.2:1b",   "params": "1B",  "family": "Llama 3.2"},
    {"name": "qwen2.5:0.5b",  "params": "0.5B","family": "Qwen 2.5"},
]

PROMPTS = [
    "Explain what a data pipeline is in two sentences.",
    "What is the difference between RAM and disk storage?",
    "Summarize the key steps to train a machine learning model.",
]

PI4_LIMITS = {
    "cpu_cores":  4,
    "memory_gb":  4,
    "note": "Emulated via Docker --cpus=4 --memory=4g on x86_64. "
            "Tokens/sec will be higher than real ARM Cortex-A72 (Pi 4). "
            "RAM and thermal behaviour is representative.",
}


def _get(url: str, timeout: int = 10) -> dict:
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)


def _post(url: str, body: dict, timeout: int = 120) -> dict:
    data = json.dumps(body).encode()
    req = urllib.request.Request(
        url, data=data,
        headers={"Content-Type": "application/json", "Accept": "application/json"},
        method="POST"
    )
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)


def get_mcp_snapshot() -> dict:
    """Read RAM and temperature from MCP hardware service."""
    try:
        return _get(f"{MCP_BASE}/health", timeout=5)
    except Exception:
        # Fallback: read /proc/meminfo directly
        meminfo = {}
        for line in Path("/proc/meminfo").read_text().splitlines():
            if ":" in line:
                k, v = line.split(":", 1)
                meminfo[k.strip()] = int(v.strip().split()[0])
        return {
            "ram": {
                "available_mb": meminfo.get("MemAvailable", 0) // 1024,
                "used_mb":      (meminfo.get("MemTotal", 0) - meminfo.get("MemAvailable", 0)) // 1024,
            },
            "temp": {"max_temp_c": None, "status": "unavailable"},
            "_source": "fallback",
        }


def benchmark_model(model_name: str, prompts: list[str]) -> dict:
    """
    Run inference benchmark for one model across all prompts.

    Returns aggregated stats: mean/min/max tokens/sec, peak RAM delta, temp.
    """
    print(f"\n{'='*60}")
    print(f"  Model: {model_name}")
    print(f"{'='*60}")

    # Warm up — load the model into memory without timing it
    print("  Warming up (loading model)...", end=" ", flush=True)
    try:
        _post(f"{OLLAMA_BASE}/api/generate", {
            "model": model_name,
            "prompt": "Hello",
            "stream": False,
            "options": {"num_predict": 5}
        }, timeout=120)
        print("done")
    except Exception as e:
        print(f"FAILED: {e}")
        return {"model": model_name, "error": str(e)}

    results = []
    snap_before = get_mcp_snapshot()
    ram_before  = snap_before["ram"]["used_mb"]

    for i, prompt in enumerate(prompts, 1):
        print(f"\n  Prompt {i}/{len(prompts)}: {prompt[:50]}...")

        wall_start = time.perf_counter()
        try:
            resp = _post(f"{OLLAMA_BASE}/api/generate", {
                "model":   model_name,
                "prompt":  prompt,
                "stream":  False,
                "options": {
                    "num_predict": 150,   # cap output length for consistent measurement
                    "temperature": 0.1,   # near-deterministic for reproducibility
                }
            }, timeout=180)
        except Exception as e:
            print(f"    ERROR: {e}")
            continue

        wall_elapsed = time.perf_counter() - wall_start

        # Ollama reports eval_count (output tokens) and eval_duration (nanoseconds)
        eval_count    = resp.get("eval_count", 0)
        eval_duration = resp.get("eval_duration", 1)   # nanoseconds
        prompt_tokens = resp.get("prompt_eval_count", 0)

        # Server-side tokens/sec (most accurate — excludes network overhead)
        server_tps = eval_count / (eval_duration / 1e9) if eval_duration > 0 else 0

        # Wall-clock tokens/sec (includes HTTP overhead — conservative estimate)
        wall_tps = eval_count / wall_elapsed if wall_elapsed > 0 else 0

        snap_mid  = get_mcp_snapshot()
        ram_used  = snap_mid["ram"]["used_mb"]
        temp      = snap_mid["temp"].get("max_temp_c")

        result = {
            "prompt_tokens":  prompt_tokens,
            "output_tokens":  eval_count,
            "wall_elapsed_s": round(wall_elapsed, 2),
            "server_tps":     round(server_tps, 2),
            "wall_tps":       round(wall_tps, 2),
            "ram_used_mb":    ram_used,
            "temp_c":         temp,
        }
        results.append(result)

        print(f"    Output tokens : {eval_count}")
        print(f"    Server t/s    : {server_tps:.1f}")
        print(f"    Wall t/s      : {wall_tps:.1f}")
        print(f"    Elapsed       : {wall_elapsed:.1f}s")
        print(f"    RAM used      : {ram_used} MB")
        if temp:
            print(f"    CPU temp      : {temp}°C")

    # Unload model from RAM after benchmarking
    try:
        _post(f"{OLLAMA_BASE}/api/generate", {
            "model": model_name, "prompt": "", "keep_alive": 0
        }, timeout=10)
    except Exception:
        pass

    snap_after   = get_mcp_snapshot()
    ram_after    = snap_after["ram"]["used_mb"]
    ram_delta_mb = ram_before - ram_after   # how much RAM the model occupied

    if not results:
        return {"model": model_name, "error": "all prompts failed"}

    server_tps_vals = [r["server_tps"] for r in results]
    wall_tps_vals   = [r["wall_tps"]   for r in results]
    temps           = [r["temp_c"] for r in results if r["temp_c"] is not None]

    return {
        "model":              model_name,
        "prompts_run":        len(results),
        "server_tps_mean":    round(sum(server_tps_vals) / len(server_tps_vals), 1),
        "server_tps_min":     round(min(server_tps_vals), 1),
        "server_tps_max":     round(max(server_tps_vals), 1),
        "wall_tps_mean":      round(sum(wall_tps_vals) / len(wall_tps_vals), 1),
        "peak_ram_delta_mb":  ram_delta_mb,
        "peak_temp_c":        max(temps) if temps else None,
        "raw":                results,
    }


def run_all(models: list[dict], prompts: list[str]) -> dict:
    """Run benchmarks for all models and return full report."""
    print("\nGroot SLM Benchmark — Pi-class Hardware Emulation")
    print(f"Date       : {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"Host       : Lenovo M720Q · Ubuntu 22.04 · x86_64")
    print(f"Limits     : {PI4_LIMITS['cpu_cores']} vCPU · {PI4_LIMITS['memory_gb']}GB RAM (Docker cgroups)")
    print(f"Ollama     : {_get(f'{OLLAMA_BASE}/api/version').get('version', 'unknown')}")

    snap = get_mcp_snapshot()
    print(f"RAM avail  : {snap['ram']['available_mb']} MB")
    print(f"CPU temp   : {snap['temp'].get('max_temp_c', 'N/A')}°C")

    results = []
    for model_info in models:
        result = benchmark_model(model_info["name"], prompts)
        result["params"]  = model_info["params"]
        result["family"]  = model_info["family"]
        results.append(result)

    return {
        "benchmark_date": datetime.now().isoformat(),
        "host":           "Lenovo M720Q · Ubuntu 22.04 · x86_64",
        "pi4_limits":     PI4_LIMITS,
        "ollama_version": _get(f"{OLLAMA_BASE}/api/version").get("version", "unknown"),
        "models":         results,
    }


def print_summary(report: dict):
    print(f"\n{'='*60}")
    print("  SUMMARY")
    print(f"{'='*60}")
    print(f"{'Model':<22} {'Params':<8} {'t/s (server)':<14} {'t/s (wall)':<12} {'RAM Δ MB':<10} {'Temp °C'}")
    print("-" * 80)
    for m in report["models"]:
        if "error" in m:
            print(f"{m['model']:<22} ERROR: {m['error']}")
            continue
        temp = f"{m['peak_temp_c']:.1f}" if m["peak_temp_c"] else "N/A"
        print(
            f"{m['model']:<22} {m['params']:<8} "
            f"{m['server_tps_mean']:<14.1f} {m['wall_tps_mean']:<12.1f} "
            f"{m['peak_ram_delta_mb']:<10} {temp}"
        )
    print(f"\nNote: {PI4_LIMITS['note']}")


def main():
    parser = argparse.ArgumentParser(description="SLM benchmark — Pi-class limits")
    parser.add_argument("--model",  help="Single model to benchmark (default: all)")
    parser.add_argument("--output", default="slm_benchmark_results.json",
                        help="Output JSON file (default: slm_benchmark_results.json)")
    args = parser.parse_args()

    models = MODELS
    if args.model:
        models = [m for m in MODELS if m["name"] == args.model]
        if not models:
            models = [{"name": args.model, "params": "?", "family": "custom"}]

    report = run_all(models, PROMPTS)
    print_summary(report)

    output_path = Path(args.output)
    output_path.write_text(json.dumps(report, indent=2))
    print(f"\nFull results saved to: {output_path.resolve()}")


if __name__ == "__main__":
    main()
