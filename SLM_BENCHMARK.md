# SLM Inference Benchmark — Pi-Class Hardware Limits

**Author:** Yasaswin Palukuri  
**Date:** October 6, 2026  
**Host:** Lenovo M720Q · Intel Core i5-8400T · Ubuntu 22.04 · x86_64  
**Ollama:** 0.31.2  
**Repo:** [Groot-Agent-System](https://github.com/yasaswinpalukuri/Groot-Agent-System)

---

## Why This Benchmark

Small language models (sub-2B parameters) are the practical choice for edge devices with constrained RAM and no GPU — Raspberry Pi 4/5, Jetson Nano, and similar hardware. Before deploying an SLM to an edge node, you need to know three things:

1. **Throughput** — tokens per second at realistic output lengths
2. **Memory footprint** — how much RAM the model actually occupies at runtime
3. **Thermal behaviour** — does sustained inference push the chip into throttling range?

This benchmark answers all three for two models under Raspberry Pi 4–class constraints.

---

## Hardware Configuration

| Parameter | Value |
|---|---|
| Host machine | Lenovo M720Q (Groot OS) |
| CPU | Intel Core i5-8400T (6 cores, 1.7–3.3 GHz) |
| RAM | 16 GB DDR4 |
| OS | Ubuntu 22.04 LTS |
| Inference engine | Ollama 0.31.2 (CPU-only, no GPU) |
| Emulated target | Raspberry Pi 4 (4-core ARM Cortex-A72, 4 GB RAM) |

### Emulation Method

Docker cgroup limits applied: `--cpus=4 --memory=4g`

**Important caveat:** `/proc/meminfo` and `os.cpu_count()` report host values inside a Docker container — the limits are enforced by the kernel's cgroup scheduler, not reflected in process-visible system calls. Tokens/sec on this x86_64 machine will be **3–5× higher** than on real ARM Cortex-A72 silicon. The RAM delta and thermal readings are representative of load behaviour, not ARM-specific values.

A correction factor of ~0.25–0.35× should be applied when estimating real Pi 4 throughput.

---

## Models Tested

| Model | Parameters | Quantization | Disk Size | Family |
|---|---|---|---|---|
| llama3.2:1b | 1B | Q4_K_M | 1.3 GB | Meta Llama 3.2 |
| qwen2.5:0.5b | 0.5B | Q4_K_M | 0.4 GB | Alibaba Qwen 2.5 |

---

## Benchmark Design

**Prompts (3 per model, averaged):**
1. "Explain what a data pipeline is in two sentences."
2. "What is the difference between RAM and disk storage?"
3. "Summarize the key steps to train a machine learning model."

**Settings:** `num_predict=150`, `temperature=0.1` (near-deterministic for reproducibility)

**Measurements:**
- **Server t/s** — `eval_count / (eval_duration / 1e9)` from Ollama's response body. Excludes HTTP overhead. Most accurate.
- **Wall t/s** — total output tokens / wall-clock elapsed. Conservative estimate including network overhead.
- **RAM delta** — host RAM used before model load minus after model unload, via MCP hardware tools reading `/proc/meminfo`
- **Peak temp** — host CPU thermal zones via MCP hardware tools reading `/sys/class/thermal`

**Warm-up:** One inference pass before timing to load the model into RAM.

---

## Results

### Raw Results

#### llama3.2:1b

| Prompt | Output tokens | Server t/s | Wall t/s | Elapsed | RAM used (host) | CPU temp |
|---|---|---|---|---|---|---|
| Data pipeline | 92 | 21.5 | 19.3 | 4.8s | 14,615 MB | 61°C |
| RAM vs disk | 150 | 20.6 | 19.4 | 7.7s | 14,616 MB | 65°C |
| ML training steps | 150 | 21.8 | 20.4 | 7.4s | 14,610 MB | 67°C |

#### qwen2.5:0.5b

| Prompt | Output tokens | Server t/s | Wall t/s | Elapsed | RAM used (host) | CPU temp |
|---|---|---|---|---|---|---|
| Data pipeline | 38 | 54.1 | 36.9 | 1.0s | 13,628 MB | 64°C |
| RAM vs disk | 150 | 54.6 | 48.9 | 3.1s | 13,637 MB | 66°C |
| ML training steps | 150 | 56.0 | 49.8 | 3.0s | 13,640 MB | 68°C |

### Summary

| Model | Params | Server t/s (mean) | Wall t/s (mean) | Peak temp | Estimated Pi 4 t/s* |
|---|---|---|---|---|---|
| llama3.2:1b | 1B | **21.3** | 19.7 | 67°C | ~5–7 t/s |
| qwen2.5:0.5b | 0.5B | **54.9** | 45.2 | 68°C | ~14–19 t/s |

*Estimated Pi 4 throughput applies a 0.25–0.35× correction factor for ARM Cortex-A72 vs Intel x86_64.

---

## Analysis

### Throughput

qwen2.5:0.5b generates tokens **2.6× faster** than llama3.2:1b despite half the parameter count. This makes it the clear choice for latency-sensitive edge applications — chatbots, real-time classification, interactive assistants. At an estimated 14–19 t/s on a real Pi 4, it can produce a coherent 100-token response in 5–7 seconds, which is acceptable for most applications.

llama3.2:1b at an estimated 5–7 t/s on real Pi 4 hardware is better suited for batch processing where latency is not critical — nightly summarization, offline document analysis, background classification.

### Thermal behaviour

Both models sustained inference without exceeding 68°C on the M720Q, which has active cooling. A Raspberry Pi 4 with a heatsink and fan should remain in the 65–75°C range under sustained load — within safe operating limits (throttling begins at 80°C on the Pi's Broadcom BCM2711). Passive cooling only (no fan) is not recommended for sustained inference workloads.

### Memory

qwen2.5:0.5b's 0.4 GB disk size means it fits comfortably within the Pi 4's 4 GB RAM alongside an OS and application stack. llama3.2:1b at 1.3 GB is also viable but leaves less headroom for the rest of the software.

### Wall t/s vs Server t/s gap

The gap between server and wall t/s (qwen2.5:0.5b: 54.9 vs 45.2) reflects HTTP overhead — the connection setup, request parsing, and response buffering in Ollama's REST layer. This gap is larger for fast models because inference itself is shorter relative to fixed overhead. For production edge deployment, using Ollama's streaming API reduces this gap by returning tokens as they're generated.

---

## Recommendations for Edge Deployment

| Use case | Recommended model | Reason |
|---|---|---|
| Real-time chat / assistant | qwen2.5:0.5b | 2.6× faster, fits in 4 GB with headroom |
| Document summarization (batch) | llama3.2:1b | Better reasoning quality, latency acceptable |
| Classification / routing | qwen2.5:0.5b | Sub-second responses for short outputs |
| Always-on background agent | qwen2.5:0.5b | Lower RAM footprint, more room for other processes |

---

## Measurement Infrastructure

Hardware telemetry (RAM and temperature) was collected via a custom **MCP (Model Context Protocol) server** built for Groot OS. The server exposes four tools — `get_ram()`, `get_cpu_temp()`, `get_ollama_models()`, `get_system_health()` — over stdio (for Claude Desktop) and REST (for Docker containers). This is the same infrastructure Groot's scheduler uses to decide whether to load a model before dispatching an inference task.

Source: [`groot_mcp_server.py`](groot_mcp_server.py), [`groot_mcp_rest.py`](groot_mcp_rest.py)

---

## Reproducing This Benchmark

```bash
# Install Ollama and pull models
curl -fsSL https://ollama.ai/install.sh | sh
ollama pull llama3.2:1b
ollama pull qwen2.5:0.5b

# Run benchmark
python3 slm_benchmark.py

# Single model
python3 slm_benchmark.py --model qwen2.5:0.5b
```

Results are saved to `slm_benchmark_results.json`.

---

## Files

| File | Description |
|---|---|
| `slm_benchmark.py` | Benchmark script |
| `slm_benchmark_results.json` | Raw results (JSON) |
| `groot_mcp_server.py` | MCP server — hardware telemetry tools |
| `groot_mcp_rest.py` | REST wrapper for Docker container access |
