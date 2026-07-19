# Groot — Self-Hosted Multi-Agent AI System

> *"A self-organizing AI operating system for one person's professional and personal life."*

**Status:** Demo stack active · 5 of 8 agents · Running 24/7 on Lenovo M720Q (i5-8500T, 16GB RAM)

---

## What It Is

Groot is a production-grade, self-hosted multi-agent AI system that autonomously handles research, job searching, learning, code generation, and content creation — with human-in-the-loop approval for every irreversible action. It runs entirely on local hardware with no cloud AI dependencies, using Ollama for CPU-only inference.

The system is not a demo or tutorial project. It runs continuously, processes real tasks, posts to a live Slack workspace, and has been used to generate real job applications, write code, and conduct research.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        GROOT OS (M720Q)                         │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │     n8n     │  │  FastAPI    │  │  Dashboard  │           │
│  │   :5678     │  │   :8000     │  │   :3000     │           │
│  │  Workflows  │  │  LangGraph  │  │  React+Vite │           │
│  └──────┬──────┘  └──────┬──────┘  └─────────────┘           │
│         │                │                                      │
│         └────────┬───────┘                                     │
│                  │                                              │
│  ┌───────────────▼──────────────┐  ┌──────────────────────┐  │
│  │       Load Scheduler          │  │      ChromaDB        │  │
│  │  asyncio Priority Queue       │  │   6 Collections      │  │
│  │  RAM headroom validation      │  │   Hybrid RAG         │  │
│  │  URGENT > HIGH > MED > LOW    │  │   BM25 + Dense 40/60 │  │
│  └───────────────┬──────────────┘  └──────────────────────┘  │
│                  │                                              │
│  ┌───────────────▼──────────────────────────────────────────┐ │
│  │                      Ollama (CPU)                         │ │
│  ├──────────────────────────────────────────────────────────┤ │
│  │ qwen2.5:7b      5.0GB  → Groot (Supervisor) + Job Search │ │
│  │ deepseek-r1:14b 9.0GB  → Einstein (Researcher)           │ │
│  │ qwen2.5-coder:7b 5.0GB → Tony (Developer)                │ │
│  │ phi3:medium     8.9GB  → Siva (Tutor)                    │ │
│  │ mistral:7b      4.4GB  → Content (Production)            │ │
│  │ llama3.2:3b     2.0GB  → Lightweight routing             │ │
│  │ nomic-embed-text 274MB → RAG embeddings                  │ │
│  └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
         │ Tailscale VPN · Slack (9 channels) · Telegram
    ┌────┴──────────────────────┐
    │  Mac / Windows / Anywhere │
    │  SSH · Browser · Mobile   │
    └───────────────────────────┘
```

---

## Agent Roster

| Agent | Name | Model | Role | Status |
|-------|------|-------|------|--------|
| Supervisor | Groot | qwen2.5:7b | Task routing, orchestration | ✅ Active |
| Researcher | Einstein | deepseek-r1:14b | Deep research, chain-of-thought | ✅ Active |
| Developer | Tony | qwen2.5-coder:7b | Code generation, git, PRs via Aider | ✅ Active |
| Tutor | Siva | phi3:medium | Learning, explanations | ✅ Active |
| Job Search | — | qwen2.5:7b | Job scraping, scoring, applications | ✅ Active |
| Career | — | mistral:7b | Career strategy | ⏳ Production |
| Content | — | mistral:7b | Content creation | ⏳ Production |
| Finance | — | llama3.2:3b | Financial tracking | ⏳ Production |

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Agent Framework | LangGraph, LangChain |
| API | FastAPI (Python 3.11) |
| Inference | Ollama (CPU-only, 7 models) |
| Vector Store | ChromaDB 1.0.0 (6 collections) |
| RAG | Hybrid: BM25 (rank_bm25) + Dense vectors, 40/60 weighted merge |
| Orchestration | n8n (workflow automation) |
| Dashboard | React + Vite + Tailwind CSS |
| Containerization | Docker Engine + Docker Compose |
| Notifications | Slack (9 channels) + Telegram |
| Code Agent | Aider (qwen2.5-coder:7b via Ollama) |
| Job Data | Adzuna Canada API |
| VPN | Tailscale (global access, no port forwarding) |
| OS | Ubuntu 22.04.5 LTS (bare metal, LUKS encrypted) |
| Hardware | Lenovo M720Q (i5-8500T, 16GB DDR4, 235GB SSD) |

---

## Key Technical Decisions

### Load Scheduler
Custom `asyncio` priority queue with four levels (URGENT/HIGH/MEDIUM/LOW). Before every Ollama dispatch, validates available RAM via `/proc/meminfo`. If `available_RAM - model_size < 3GB`, re-queues the task and retries after 30 seconds. Prevents OOM on 16GB hardware running multiple services simultaneously. Max one large model (≥8GB) loaded at a time.

### Hybrid RAG
Two-stage retrieval combining BM25 keyword search (40% weight) with ChromaDB dense vector search (60% weight). BM25 index built on first collection access and cached in memory. Scores normalized to [0,1] before weighted merge. Every retrieval logged to `/var/log/groot/rag_audit.jsonl` with agent, collection, query, and hit count. RBAC enforced at query time via `agent_access` metadata on each collection.

### Dynamic Context Window
Context size set per agent and per priority: Groot=2048, Job Search=4096, Siva=8192, Tony=12288, Einstein=16384. URGENT priority tasks get 2× context up to 16384 max. Eliminates the performance overhead of a globally-fixed large context window.

### Human-in-the-Loop
Every irreversible action (job application, git merge, financial transaction) routes through an approval workflow. Slack posts interactive cards with Approve/Reject buttons. 4-hour escalation to Telegram if no response. n8n handles the workflow orchestration.

---

## RAG Collections

| Collection | Access | Purpose |
|------------|--------|---------|
| learning_notes | groot, einstein, siva | Study notes, architecture docs, resume |
| research_reports | groot, einstein | Research outputs |
| job_descriptions | groot, job_search, einstein | Job listings, strategy |
| applied_jobs | groot, job_search | Application tracking |
| documentation_cache | groot, tony, einstein | Technical docs |
| code_patterns | groot, tony | Code templates, patterns |

---

## Services

| Service | Port | Access |
|---------|------|--------|
| FastAPI (agent_service) | 8000 | LAN + Tailscale |
| n8n workflows | 5678 | LAN + Tailscale |
| React Dashboard | 3000 | LAN + Tailscale |
| ChromaDB | 8001 | Internal only |
| Ollama | 11434 | Host only |

---

## Infrastructure

- **OS:** Ubuntu 22.04.5 LTS, bare metal, LUKS full-disk encryption
- **Access:** Tailscale VPN (`groot` = `100.112.171.54`) — works from anywhere, no port forwarding
- **Boot:** `groot.service` systemd unit auto-starts all Docker containers on boot
- **Monitoring:** Watchdog cron every 5 minutes, Slack + Telegram alerts on failure
- **Backup:** rsync to mounted USB pendrive every 12 hours, 7-day rotation
- **DNS:** Avahi mDNS (`groot.local`) for local network access

---

## Repository Structure

```
Groot-Agent-System/
├── job_scraper.py          ← Adzuna Canada API job search (Tony wrote this)
└── README.md
```

> Full agent service, dashboard, and infrastructure code lives on the M720Q at `~/agents/`. Selective components pushed here as portfolio artifacts.

---

## Current Capabilities

- ✅ Real job search via Adzuna Canada API (20 results, filterable by role/location)
- ✅ Streaming chat with any of 4 agents via dashboard
- ✅ Einstein (deepseek-r1:14b) research tasks via `/run` endpoint
- ✅ Tony writes and commits code via Aider + qwen2.5-coder:7b
- ✅ Slack notifications across 9 channels
- ✅ Telegram alerts and file delivery
- ✅ Hybrid RAG retrieval across 6 ChromaDB collections
- ✅ Live dashboard with real-time RAM/queue monitoring via WebSocket
- 🔄 Resume tailoring pipeline (in progress)
- 🔄 Automated job application with approval flow (in progress)
- ⏳ Ragas evaluation pipeline
- ⏳ Full 8-agent stack (requires 32GB production hardware)

---

## Author

**Yasaswin Palukuri** — Data Engineer · AI/ML Engineer · LLMOps Engineer

- 📍 Toronto, Canada
- 🔗 [LinkedIn](https://www.linkedin.com/in/yasaswin-palukuri/)
- 🐙 [GitHub](https://github.com/yasaswinpalukuri)
- 📧 palukuriyasaswin@gmail.com

> Open to Data Engineer · ML Engineer · LLMOps roles (Remote / Toronto / Vancouver / India)
