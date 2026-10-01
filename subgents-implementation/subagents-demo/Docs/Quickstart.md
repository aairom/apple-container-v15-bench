# Quickstart Guide

> Get the Sub-Agents Demo running in under 5 minutes.

---

## Prerequisites

| Requirement | Check |
|---|---|
| Python 3.10+ | `python3 --version` |
| Ollama (default LLM) | `ollama --version` → [install](https://ollama.com) if missing |
| llama3 model | `ollama list` → run `ollama pull llama3` if not listed |

> **Alternative backends:** llama.cpp (no network, fully offline) and OpenAI are also
> supported — see [LLM Backends](#llm-backends) below.

---

## 1 – Clone / enter the project

```bash
cd subagents-demo
```

---

## 2 – Configure environment

```bash
cp .env.example .env
```

Open `.env` and verify the defaults match your setup:

```ini
LLM_BACKEND=ollama
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama3
TARGET_CODEBASE_DIR=input
OUTPUT_DIR=output
LOG_LEVEL=INFO
```

### LLM Backends

| `LLM_BACKEND` | What to set |
|---|---|
| `ollama` *(default)* | `OLLAMA_BASE_URL` + `OLLAMA_MODEL` |
| `llamacpp` | `LLAMACPP_MODEL_PATH=/path/to/model.gguf` (absolute path) |
| `openai` | `OPENAI_API_KEY=sk-…` (and optionally `OPENAI_MODEL`) |

> **Using OpenAI instead?**  
> Set `LLM_BACKEND=openai` and add `OPENAI_API_KEY=sk-…` to `.env`.  
> Uncomment `openai>=1.30.0` in `requirements.txt` and reinstall.

> **Using llama.cpp?**
> Set `LLM_BACKEND=llamacpp` and set `LLAMACPP_MODEL_PATH` to your GGUF file.
> Then uncomment `llama-cpp-python>=0.2.90` in `requirements.txt` and reinstall,
> or install manually with GPU flags (see README).

---

## 3 – Run (one command)

```bash
./scripts/run.sh
```

This script automatically:
1. Creates `venv/` and installs `requirements.txt`
2. Creates `output/` directory
3. Launches the orchestrator

---

## 4 – Watch the output

You will see the orchestrator register both sub-agents, assign tasks, execute them,
and print a summary table:

```
  ✓  Sub-agent registered: [code-explorer]    (type=explore)
  ✓  Sub-agent registered: [documentation-writer] (type=general)

  ✓  Task assigned → [code-explorer]
  ⏳  Executing [code-explorer] …
  ✓  [code-explorer] completed in 4.2s  – tools used: ['directory_scan', 'ast_parser', 'llm_summarise']
     Files analysed: 5
     ── Explorer LLM summary ──
     This codebase implements a simple inventory management system …
     … (see report for full text)

  ✓  Task assigned → [documentation-writer] (fork_context=True, 5 module(s) forwarded)
  ⏳  Executing [documentation-writer] …
  ✓  [documentation-writer] completed in 6.1s  – tools used: ['payload_reader', 'llm_document', 'markdown_assembler']
     Modules documented: 5

  ✓ SUCCESS  [code-explorer]          4.2s  tools=['directory_scan', 'ast_parser', 'llm_summarise']
  ✓ SUCCESS  [documentation-writer]  6.1s  tools=['payload_reader', 'llm_document', 'markdown_assembler']

  Total time   : 10.3s
  All succeeded: True
  Report file  : output/subagents_report_20250601_143022.md
```

---

## 5 – Read the report

```bash
cat output/subagents_report_*.md
```

The report contains:
- **Project Overview** – the explorer sub-agent's LLM summary of the codebase
- **Module Reference** – per-module API docs written by the documentation-writer sub-agent

---

## 6 – Launch the Web UI (optional)

A Streamlit browser interface is available as an alternative to the CLI:

```bash
./scripts/ui.sh
```

Open **http://localhost:8501** in your browser.

The UI provides six tabs:

| Tab | What you can do |
|---|---|
| 🏠 Overview | Understand the architecture and agent roles |
| ⚙️ Configure | Switch LLM backend, set paths and model names |
| ▶️ Run Pipeline | Execute the pipeline and watch live console output |
| 🔍 Sub-Agent Results | Inspect per-agent summaries and tools used |
| 📄 Final Report | View and download the assembled Markdown report |
| 📖 About Bob Subagents | Read inline Bob documentation |

> The UI port defaults to **8501**. Override with `UI_PORT=<port>` in `.env`.  
> Port 5000 is blocked (reserved for macOS AirDrop).

---

## Running Tests

```bash
source venv/bin/activate
pytest tests/ -v
```

Tests mock the LLM — no Ollama, API key, or GGUF file is needed.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `Connection refused` to `localhost:11434` | Ollama is not running | `ollama serve` in a separate terminal |
| `Model not found` | llama3 not pulled | `ollama pull llama3` |
| `OPENAI_API_KEY is not set` | Using OpenAI without key | Add `OPENAI_API_KEY=sk-…` to `.env` |
| `LLAMACPP_MODEL_PATH is not set` | Using llama.cpp without a model path | Set `LLAMACPP_MODEL_PATH=/path/to/model.gguf` in `.env` |
| `ModuleNotFoundError: llama_cpp` | llama-cpp-python not installed | `pip install "llama-cpp-python>=0.2.90"` |
| Empty `output/` directory | Run crashed before completion | Check `logs/subagents.log` |
| `ModuleNotFoundError: requests` | venv not activated | `source venv/bin/activate` |
| `ModuleNotFoundError: streamlit` | venv not activated or UI deps missing | `source venv/bin/activate && pip install streamlit` |
| UI not starting / blank page | Port conflict or Streamlit error | Set `UI_PORT=8502` in `.env` and re-run `./scripts/ui.sh` |

---

## Background Mode

```bash
./scripts/start.sh          # starts orchestrator + UI detached; prints URL on console
```

Console output:

```
  ┌──────────────────────────────────────────────────────────┐
  │  Sub-Agents Demo is running                              │
  └──────────────────────────────────────────────────────────┘

  ➜  Web UI:  http://localhost:8501

  Orchestrator log : …/logs/subagents.log
  UI log           : …/logs/ui.log
```

```bash
tail -f logs/subagents.log  # follow orchestrator output
tail -f logs/ui.log         # follow UI output
./scripts/stop.sh           # graceful shutdown of both processes
```
