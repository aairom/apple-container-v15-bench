# Sub-Agents Demo

> A fully functional reference implementation of the **IBM Bob Sub-Agents** feature.  
> Reference documentation: <https://bob.ibm.com/docs/ide/features/subagents>

[![Python 3.10+](https://img.shields.io/badge/python-3.10%2B-blue)](https://www.python.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## What the Demo Does

This application demonstrates every stage of the Bob sub-agent lifecycle using a
concrete, runnable Python implementation:

1. An **Orchestrator** (parent agent) is initialised with a target directory of
   Python source files (`input/`).
2. It **spawns** two independent sub-agents — each with its own isolated context — and
   **delegates** a non-overlapping task to each.
3. Each sub-agent calls the configured **LLM backend** (Ollama, llama.cpp, or OpenAI),
   and its full response is printed to the console in a clearly labelled block.
4. The orchestrator **collects** each sub-agent's result summary and **aggregates**
   them into a single Markdown report written to `output/`.

---

## Agents at a Glance

| Agent | Type | Responsibility |
|---|---|---|
| **`code-explorer`** | `explore` | Read-only scan: discovers Python files, extracts AST metadata (classes, functions, imports, docstrings), asks the LLM for a natural-language summary. |
| **`documentation-writer`** | `general` | Receives explorer findings via `fork_context`, calls the LLM to write Markdown API docs for each module, assembles a complete project document. |
| **Orchestrator** | — | Registers both agents, assigns tasks, sequences execution, handles errors, prints LLM output to console, writes the unified report. |

---

## Project Structure

```
subagents-demo/
├── input/                              # Sample codebase that sub-agents analyse
│   ├── data_processor.py
│   ├── inventory.py
│   ├── order_processor.py
│   ├── reporting.py
│   └── utils.py
├── src/                                # Application source
│   ├── models.py                       # DTOs (SubAgentTask, SubAgentResult …)
│   ├── utils.py                        # Logging, LLM backend factory, output helpers
│   ├── code_explorer_agent.py          # Sub-Agent 1 (explore type)
│   ├── documentation_writer_agent.py   # Sub-Agent 2 (general type)
│   ├── orchestrator.py                 # Parent / orchestrator agent
│   └── ui.py                           # Streamlit web UI
├── tests/
│   └── test_subagents.py               # 31 unit tests (LLM fully mocked)
├── scripts/
│   ├── run.sh                          # Run CLI interactively (foreground)
│   ├── start.sh                        # Start CLI in detached/background mode
│   ├── stop.sh                         # Gracefully stop background process
│   ├── ui.sh                           # Launch Streamlit web UI
│   └── cleanup.sh                      # Remove all generated artefacts and caches
├── Docs/
│   ├── Architecture.md                 # Mermaid architecture + workflow diagrams
│   └── Quickstart.md                   # Step-by-step setup guide
├── output/                             # Timestamped Markdown reports (at runtime)
├── .env.example                        # Template for all environment variables
├── requirements.txt
└── README.md
```

---

## LLM Backends

Switch between inference backends using the `LLM_BACKEND` environment variable:

| `LLM_BACKEND` | Description | Requires |
|---|---|---|
| `ollama` *(default)* | Ollama local HTTP API | Ollama running + model pulled |
| `llamacpp` | llama-cpp-python in-process | GGUF model file + `LLAMACPP_MODEL_PATH` |
| `openai` | OpenAI Chat Completions | `OPENAI_API_KEY` |

> **LLM output is always printed to the console** in a labelled block:
> ```
> ──────────────────── LLM [ollama] → code-explorer ────────────────────
> This codebase implements a simple inventory …
> ────────────────────────────────────────────────────────────────────────
> ```

### llama.cpp Setup

1. **Install** the Python binding:
   ```bash
   # CPU only:
   pip install "llama-cpp-python>=0.2.90"
   
   # Apple Silicon (Metal GPU):
   CMAKE_ARGS="-DLLAMA_METAL=on" pip install "llama-cpp-python>=0.2.90"
   
   # NVIDIA CUDA:
   CMAKE_ARGS="-DLLAMA_CUBLAS=on" pip install "llama-cpp-python>=0.2.90"
   ```

2. **Download a GGUF model** (example: Llama-3 8B 4-bit quantised):
   ```bash
   mkdir -p models
   # Download from https://huggingface.co/TheBloke/Llama-3-8B-Instruct-GGUF
   # e.g. models/llama-3-8b-instruct.Q4_K_M.gguf
   ```

3. **Configure** `.env`:
   ```ini
   LLM_BACKEND=llamacpp
   LLAMACPP_MODEL_PATH=/absolute/path/to/models/llama-3-8b-instruct.Q4_K_M.gguf
   LLAMACPP_N_CTX=4096
   LLAMACPP_N_GPU_LAYERS=0   # set to 999 for full GPU offload
   LLAMACPP_MAX_TOKENS=512
   LLAMACPP_TEMPERATURE=0.7
   ```

---

## Prerequisites

| Tool | Min version | Notes |
|---|---|---|
| Python | 3.10 | `python3 --version` |
| Ollama | any | <https://ollama.com> — for default backend |
| `llama3` model | — | `ollama pull llama3` |

---

## Quick Start (CLI)

```bash
cd subagents-demo
./scripts/run.sh          # sets up venv + deps, runs orchestrator
```

---

## Web UI

A Streamlit UI exposes the full pipeline in a browser:

```bash
./scripts/ui.sh           # starts on port 8501 (macOS AirDrop port 5000 avoided)
```

Open **http://localhost:8501** in your browser.

### UI Tabs

| Tab | What you can do |
|---|---|
| 🏠 Overview | Understand the architecture and agent roles |
| ⚙️ Configure | Switch LLM backend, set target dir, model paths |
| ▶️ Run Pipeline | Execute and watch live console output |
| 🔍 Sub-Agent Results | Inspect per-agent summaries and tools |
| 📄 Final Report | View and download the assembled Markdown |
| 📖 About Bob Subagents | Read inline Bob documentation |

---

## Configuration

Copy `.env.example` to `.env` and edit:

```bash
cp .env.example .env
```

| Variable | Default | Description |
|---|---|---|
| `LLM_BACKEND` | `ollama` | Backend: `ollama`, `llamacpp`, `openai` |
| `OLLAMA_BASE_URL` | `http://localhost:11434` | Ollama server URL |
| `OLLAMA_MODEL` | `llama3` | Ollama model name |
| `LLAMACPP_MODEL_PATH` | *(unset)* | Absolute path to GGUF model file |
| `LLAMACPP_N_CTX` | `4096` | Context window (tokens) |
| `LLAMACPP_N_GPU_LAYERS` | `0` | GPU layers (0 = CPU only) |
| `LLAMACPP_MAX_TOKENS` | `512` | Max tokens per generation |
| `LLAMACPP_TEMPERATURE` | `0.7` | Sampling temperature |
| `OPENAI_API_KEY` | *(unset)* | OpenAI key (only for `openai` backend) |
| `OPENAI_MODEL` | `gpt-4o` | OpenAI model |
| `TARGET_CODEBASE_DIR` | `input` | Directory explorer agent scans |
| `OUTPUT_DIR` | `output` | Where the final report is written |
| `PROJECT_NAME` | *(demo name)* | Report title |
| `UI_PORT` | `8501` | Streamlit port (never 5000) |
| `LOG_LEVEL` | `INFO` | `DEBUG` \| `INFO` \| `WARNING` \| `ERROR` |

---

## Running

### CLI – foreground
```bash
./scripts/run.sh
```

### CLI – background (orchestrator + UI, detached)
```bash
./scripts/start.sh           # prints URL on console immediately
tail -f logs/subagents.log   # follow orchestrator output
tail -f logs/ui.log          # follow UI output
./scripts/stop.sh            # stop both processes
```

### Web UI
```bash
./scripts/ui.sh
```

### Direct Python
```bash
source venv/bin/activate
cd src
python3 orchestrator.py
```

---

## Expected Console Output

```
┌──────────────────────────────────────────────────────────────────────┐
│  Bob Sub-Agents Demo – Orchestrator starting                         │
└──────────────────────────────────────────────────────────────────────┘

  ✓  Sub-agent registered: [code-explorer]    (type=explore)
  ✓  Sub-agent registered: [documentation-writer] (type=general)

  ✓  Task assigned → [code-explorer]
  ⏳  Executing [code-explorer] …

────────────── LLM [ollama] → code-explorer ───────────────
This codebase implements a simple inventory management system …
────────────────────────────────────────────────────────────────────────

  ✓  [code-explorer] completed in 4.2s  – tools used: ['directory_scan', 'ast_parser', 'llm_summarise']
     Files analysed: 5
     ── Explorer LLM summary ──
     This codebase implements a simple inventory management system …
     … (see report for full text)

  ✓  Task assigned → [documentation-writer] (fork_context=True, 5 module(s) forwarded)
  ⏳  Executing [documentation-writer] …

──────────── LLM [ollama] → documentation-writer ─────────────
## inventory.py – Inventory Management Module …
────────────────────────────────────────────────────────────────────────

  ✓  [documentation-writer] completed in 6.1s  – tools used: ['payload_reader', 'llm_document', 'markdown_assembler']
     Modules documented: 5

────────────────────────────────────────────────────────────────────────
  ORCHESTRATOR SUMMARY
────────────────────────────────────────────────────────────────────────
  ✓ SUCCESS  [code-explorer]          4.2s  tools=['directory_scan', 'ast_parser', 'llm_summarise']
  ✓ SUCCESS  [documentation-writer]  6.1s  tools=['payload_reader', 'llm_document', 'markdown_assembler']

  Total time   : 10.3s
  All succeeded: True
  Report file  : output/subagents_report_20250601_143022.md
────────────────────────────────────────────────────────────────────────
```

---

## Running Tests

```bash
source venv/bin/activate
pytest tests/ -v
```

**31 tests, all pass.** LLM is fully mocked — no Ollama, no API key, no GGUF file needed.

---

## Cleanup

```bash
./scripts/cleanup.sh           # remove reports, logs, caches
./scripts/cleanup.sh --all     # also remove venv and downloaded models
./scripts/cleanup.sh --force   # skip confirmation prompts
```

---

## How It Maps to the Bob Documentation

| Bob Concept | This Demo |
|---|---|
| Sub-agent spawned with isolated context | Each agent is a separate class instance with no shared state |
| `description` passed to sub-agent | `SubAgentTask.description` |
| `fork_context: true` | `SubAgentTask.fork_context=True` on the doc-writer task |
| `explore` type (read-only) | `CodeExplorerAgent` – uses `ast` only, never writes files |
| `general` type (full access) | `DocumentationWriterAgent` – reads payload + LLM + assembles |
| Sub-agent returns summary to parent | `SubAgentResult.summary` returned to orchestrator |
| Orchestrator aggregates results | `Orchestrator.run()` → `OrchestratorReport` |
| LLM response visible in output | `_print_llm_response()` called after every inference |

---

## Architecture

See [`Docs/Architecture.md`](Docs/Architecture.md) for Mermaid diagrams.

---

## License

MIT — see [LICENSE](LICENSE).
