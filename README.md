# Groot — Self-Hosted Multi-Agent AI System

> *"A self-organizing AI operating system for one person's professional and personal life."*

**Status:** 6 of 8 agents active · Running 24/7 on Lenovo M720Q (i5-8500T, 16GB RAM) · Last updated: July 19, 2026

---

## What It Is

Groot is a production-grade, self-hosted multi-agent AI system that autonomously handles research, job searching, learning, code generation, and career strategy — with human-in-the-loop approval for every irreversible action. It runs entirely on local hardware with no cloud AI dependencies, using Ollama for CPU-only inference.

**It is not a demo.** It runs continuously, processes real tasks, posts to a live Slack workspace, sends tailored PDF resumes to Telegram, searches real Canadian job listings, and has been used to generate actual job applications.

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
│         └────────┬───────┘                                     │
│                  │                                              │
│  ┌───────────────▼──────────────┐  ┌──────────────────────┐  │
│  │       Load Scheduler          │  │      ChromaDB        │  │
│  │  asyncio Priority Queue       │  │   6 Collections      │  │
│  │  RAM headroom validation      │  │   Hybrid RAG         │  │
│  │  URGENT > HIGH > MED > LOW    │  │   BM25(40)+Dense(60) │  │
│  └───────────────┬──────────────┘  └──────────────────────┘  │
│                  │                                              │
│  ┌───────────────▼──────────────────────────────────────────┐ │
│  │                      Ollama (CPU-only)                    │ │
│  ├──────────────────────────────────────────────────────────┤ │
│  │ qwen2.5:7b       5.0GB → Groot (Supervisor) + Job Search │ │
│  │ deepseek-r1:14b  9.0GB → Einstein (Researcher)           │ │
│  │ qwen2.5-coder:7b 5.0GB → Tony (Developer / Aider)        │ │
│  │ phi3:medium      8.9GB → Siva (Tutor)                    │ │
│  │ mistral:7b       4.4GB → Career Agent                    │ │
│  │ llama3.2:3b      2.0GB → Lightweight routing             │ │
│  │ nomic-embed-text  274M → RAG embeddings                  │ │
│  └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
    │ Tailscale VPN · Slack (9 channels) · Telegram Bot
┌───┴───────────────────────┐
│  Mac / Windows / Anywhere  │
│  SSH · Browser · Mobile    │
└────────────────────────────┘
```

---

## Agent Roster

| Agent | Name | Model | Role | Status |
|-------|------|-------|------|--------|
| Supervisor | Groot | qwen2.5:7b | Task routing, orchestration | ✅ Active |
| Researcher | Einstein | deepseek-r1:14b | Deep research, chain-of-thought | ✅ Active |
| Developer | Tony | qwen2.5-coder:7b | Code generation, git commits via Aider | ✅ Active |
| Tutor | Siva | phi3:medium | Learning, explanations | ✅ Active |
| Job Search | — | qwen2.5:7b | Job scraping, scoring, PDF resume pipeline | ✅ Active |
| Career | — | mistral:7b-instruct | LinkedIn outreach, career strategy | ✅ Active |
| Content | — | mistral:7b-instruct | Content creation | ⏳ 32GB PC |
| Finance | — | llama3.2:3b | Financial tracking | ⏳ 32GB PC |

---

## Live Capabilities

### Job Search Pipeline (fully working)
1. Search Adzuna Canada API — 20 real job listings per query
2. Click **Interested** in the dashboard
3. Einstein tailors resume using recruiter-grade prompt (ATS optimization, action verbs, quantified achievements, red flags)
4. PDF resume generated on Groot via reportlab
5. Telegram delivers: job card (company/role/salary/score/link) + PDF resume
6. Job saved to ChromaDB applied_jobs collection with status tracking

### Chat Interface
- Stream conversations with any agent via dashboard
- Each agent maintains separate conversation history per session
- Agent-specific greetings and personalities (Groot, Einstein, Tony, Siva)
- SSE streaming for real-time token delivery

### Developer Agent (Tony)
- Aider CLI wired to qwen2.5-coder:7b via Ollama
- Writes code, runs linting (ruff/flake8), auto-commits to git
- Workspace: ~/code/tony/ with git remote on GitHub
- First production output: Adzuna Canada job scraper (job_scraper.py)

### Notifications
- **Slack:** 9 channels — #groot #approvals #activity-feed #researcher #job-search #developer #tutor #system-health #errors
- **Telegram:** Job alerts, PDF resumes, system alerts, approval requests

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Agent Framework | LangGraph, LangChain |
| API | FastAPI 0.115.9 (Python 3.11) |
| Inference | Ollama (CPU-only, 7 models, 27GB total) |
| Vector Store | ChromaDB 1.0.0 (6 collections) |
| RAG | Hybrid: BM25 (rank_bm25) + Dense vectors, 40/60 weighted merge |
| PDF Generation | reportlab |
| Orchestration | n8n (workflow automation) |
| Dashboard | React + Vite + Tailwind CSS |
| Code Agent | Aider + qwen2.5-coder:7b via Ollama |
| Job Data | Adzuna Canada API |
| Containerization | Docker Engine + Docker Compose |
| Notifications | Slack (9 channels) + Telegram Bot API |
| VPN | Tailscale (groot = 100.112.171.54) |
| OS | Ubuntu 22.04.5 LTS (bare metal, LUKS encrypted) |
| Hardware | Lenovo M720Q (i5-8500T, 16GB DDR4, 235GB SSD) |

---

## Key Technical Decisions

### Load Scheduler
Custom asyncio priority queue (URGENT/HIGH/MEDIUM/LOW). Before every Ollama dispatch, reads /proc/meminfo and validates: available_RAM - model_size >= 3GB. If not, re-queues and retries after 30 seconds. Max one large model (>=8GB) loaded simultaneously on 16GB hardware.

### Hybrid RAG (BM25 + Dense, 40/60)
Two-stage retrieval: BM25 keyword search (40%) merged with ChromaDB dense vector search (60%). BM25 index built on first collection access and cached in memory. Scores normalized to [0,1] before weighted merge. RBAC enforced at query time via agent_access metadata. Every retrieval logged to /var/log/groot/rag_audit.jsonl.

### Dynamic Context Window
Per-agent and per-priority context sizing: Groot=2048, Job Search=4096, Siva=8192, Tony=12288, Einstein=16384. URGENT tasks get 2x context up to 16384 max.

### Resume Tailoring Pipeline
Senior recruiter prompt: ATS optimization, action verb enforcement, achievement quantification, 1-page constraint, red flag identification. Uses qwen2.5:7b for speed (30-60s on i5-8500T). PDF generated via reportlab, delivered via Telegram Bot API.

---

## ChromaDB Collections (6)

| Collection | Agent Access | Purpose |
|------------|-------------|---------|
| learning_notes | groot, einstein, siva | Study notes, architecture docs, resume |
| research_reports | groot, einstein | Research outputs |
| job_descriptions | groot, job_search, einstein | Job listings, career strategy |
| applied_jobs | groot, job_search | Application tracking, status |
| documentation_cache | groot, tony, einstein | Technical documentation |
| code_patterns | groot, tony | Code templates, patterns |

---

## Services & Ports

| Service | Port | Access |
|---------|------|--------|
| FastAPI (agent_service) | 8000 | LAN + Tailscale |
| n8n workflows | 5678 | LAN + Tailscale |
| React Dashboard | 3000 | LAN + Tailscale |
| ChromaDB | 8001 | Internal Docker only |
| Ollama | 11434 | Host only (Docker via 172.18.0.1) |

---

## Infrastructure

- **OS:** Ubuntu 22.04.5 LTS, bare metal, LUKS full-disk encryption
- **Access:** Tailscale VPN — works from anywhere, zero port forwarding
- **Boot:** groot.service systemd auto-starts all Docker containers on boot
- **Monitoring:** Watchdog cron every 5 min, Slack + Telegram alerts on failure
- **Backup:** rsync to USB pendrive every 12 hours, 7-day rotation at /mnt/backup
- **DNS:** Avahi mDNS (groot.local) for local network access

---

## Evaluation (Ragas)

Evaluated using Ragas 0.1.21 with qwen2.5:7b as judge model and 
nomic-embed-text for embeddings. Run on live ChromaDB collections.

| Metric | Score | Notes |
|--------|-------|-------|
| Faithfulness | 1.000 | Answers stay within retrieved context — no hallucination |
| Answer Relevancy | 0.676 | Answers address questions; 7B model verbosity affects score |
| Overall | 0.838 | |

Target on 32GB production hardware with reranker: 0.85+

---

## What's Coming (Production 32GB PC)

- Full 8-agent stack (+ Content, Finance agents)
- Nginx reverse proxy + Cloudflare Tunnel (HTTPS)
- Ragas evaluation pipeline with weekly eval scores
- guardrails-ai for prompt injection prevention and PII redaction
- Slack interactive approval buttons (requires permanent public URL)
- LinkedIn outreach automation via Career agent
- Real-time agent-to-agent message passing via LangGraph

---

## Author

**Yasaswin Palukuri** — Data Engineer · AI/ML Engineer · LLMOps Engineer

- 📍 Toronto, Canada
- 🔗 LinkedIn: https://www.linkedin.com/in/yasaswin-palukuri/
- 🐙 GitHub: https://github.com/yasaswinpalukuri
- 📧 palukuriyasaswin@gmail.com

> Open to: Data Engineer · ML Engineer · LLMOps Engineer roles
> Remote / Toronto / Vancouver / India
