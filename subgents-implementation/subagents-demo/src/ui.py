"""
ui.py – Streamlit web UI for the Sub-Agents Demo

Purpose
-------
This interactive UI teaches users how the Bob sub-agent orchestration pattern
works by letting them watch every stage of the pipeline execute live.

Tabs
----
1. 🏠 Overview          – explains the architecture and how to use the app
2. ⚙️  Configure         – choose LLM backend, target directory, project name
3. ▶️  Run Pipeline      – execute the orchestrator and watch live output
4. 🔍 Sub-Agent Results  – per-agent results: tools used, summary, raw data
5. 📄 Final Report       – rendered Markdown of the aggregated output
6. 📖 About Bob Subagents – inline copy of the key Bob documentation concepts

Usage
-----
    cd subagents-demo
    streamlit run src/ui.py --server.port 8501

Or via the script:
    ./scripts/ui.sh
"""

from __future__ import annotations

import io
import os
import sys
import time
import threading
from contextlib import redirect_stdout, redirect_stderr
from datetime import datetime
from pathlib import Path

# ── ensure src/ is importable ─────────────────────────────────────────────────
sys.path.insert(0, str(Path(__file__).parent))

import streamlit as st

# ─────────────────────────────────────────────────────────────────────────────
# Page config (must be first Streamlit call)
# ─────────────────────────────────────────────────────────────────────────────
st.set_page_config(
    page_title="Bob Sub-Agents Demo",
    page_icon="🤖",
    layout="wide",
    initial_sidebar_state="expanded",
)

# ─────────────────────────────────────────────────────────────────────────────
# Lazy imports (so the app starts even if llama-cpp-python isn't installed)
# ─────────────────────────────────────────────────────────────────────────────
try:
    from dotenv import load_dotenv  # type: ignore
    load_dotenv()
except ImportError:
    pass

from models import OrchestratorReport, SubAgentResult
from orchestrator import Orchestrator

# ─────────────────────────────────────────────────────────────────────────────
# Session-state initialisation
# ─────────────────────────────────────────────────────────────────────────────
if "report" not in st.session_state:
    st.session_state.report: OrchestratorReport | None = None
if "console_output" not in st.session_state:
    st.session_state.console_output: str = ""
if "running" not in st.session_state:
    st.session_state.running: bool = False
if "backend" not in st.session_state:
    st.session_state.backend: str = os.getenv("LLM_BACKEND", "ollama")

# ─────────────────────────────────────────────────────────────────────────────
# Sidebar
# ─────────────────────────────────────────────────────────────────────────────
with st.sidebar:
    st.image("https://www.ibm.com/brand/experience-guides/developer/b1db1ae501d522a1a4b49613fe07c9f1/01_8-bar-positive.svg",
             width=80)
    st.title("Bob Sub-Agents Demo")
    st.caption("Reference implementation of the IBM Bob sub-agents pattern.")
    st.divider()
    st.markdown(
        """
**Quick links**
- [Bob Subagents docs](https://bob.ibm.com/docs/ide/features/subagents)
- [Project README](https://github.com/)
        """
    )
    st.divider()
    backend_choice = st.selectbox(
        "🔧 Active LLM backend",
        options=["ollama", "llamacpp", "openai"],
        index=["ollama", "llamacpp", "openai"].index(
            st.session_state.backend
            if st.session_state.backend in ["ollama", "llamacpp", "openai"]
            else "ollama"
        ),
        help=(
            "ollama  – Ollama local API (default, no key needed)\n"
            "llamacpp – llama-cpp-python in-process (needs GGUF model)\n"
            "openai  – OpenAI API (needs OPENAI_API_KEY)"
        ),
    )
    if backend_choice != st.session_state.backend:
        st.session_state.backend = backend_choice
        os.environ["LLM_BACKEND"] = backend_choice
        st.success(f"Backend switched to **{backend_choice}**")

# ─────────────────────────────────────────────────────────────────────────────
# Tab layout
# ─────────────────────────────────────────────────────────────────────────────
tabs = st.tabs([
    "🏠 Overview",
    "⚙️ Configure",
    "▶️ Run Pipeline",
    "🔍 Sub-Agent Results",
    "📄 Final Report",
    "📖 About Bob Subagents",
])

# ─────────────────────────────────────────────────────────────────────────────
# Tab 1 – Overview
# ─────────────────────────────────────────────────────────────────────────────
with tabs[0]:
    st.header("🏠 Overview")
    st.markdown(
        """
This application is a **live, runnable reference implementation** of the
[IBM Bob sub-agents](https://bob.ibm.com/docs/ide/features/subagents) feature.

It demonstrates the complete sub-agent lifecycle:

| Stage | What happens |
|---|---|
| **Registration** | The orchestrator instantiates each sub-agent |
| **Task assignment** | A `SubAgentTask` (with `description` and `payload`) is handed to each agent |
| **Execution** | Each agent runs in isolation, calls the LLM, and returns a `SubAgentResult` |
| **Result collection** | The orchestrator gathers both results |
| **Aggregation** | A unified Markdown report is assembled and saved |

### Two sub-agents
        """
    )
    col1, col2 = st.columns(2)
    with col1:
        st.info(
            "**🔍 code-explorer** (explore type)\n\n"
            "Scans a directory of Python files using `ast`, extracts structural "
            "metadata (classes, functions, imports, docstrings), and asks the LLM "
            "to produce a natural-language summary. **Read-only** – never writes any file.",
        )
    with col2:
        st.success(
            "**✍️ documentation-writer** (general type)\n\n"
            "Receives the explorer's findings via `fork_context`, calls the LLM "
            "to write polished Markdown API docs for each module, and assembles a "
            "complete project document. Returns the Markdown to the orchestrator.",
        )
    st.markdown(
        """
### How to use this UI

1. **Configure** – set the LLM backend and target directory in the ⚙️ tab  
   *(or use the sidebar selector to switch backends instantly)*
2. **Run Pipeline** – hit **▶ Run** and watch live console output
3. **Sub-Agent Results** – inspect per-agent summaries and tool lists
4. **Final Report** – read the assembled Markdown documentation
        """
    )

# ─────────────────────────────────────────────────────────────────────────────
# Tab 2 – Configure
# ─────────────────────────────────────────────────────────────────────────────
with tabs[1]:
    st.header("⚙️ Configure")
    st.caption("Settings here override the .env file for this session only.")

    with st.expander("🔧 LLM Backend", expanded=True):
        backend_tab = st.radio(
            "Backend",
            options=["ollama", "llamacpp", "openai"],
            index=["ollama", "llamacpp", "openai"].index(st.session_state.backend),
            horizontal=True,
            help="Choose which LLM engine to use for inference.",
        )
        os.environ["LLM_BACKEND"] = backend_tab
        st.session_state.backend = backend_tab

        if backend_tab == "ollama":
            c1, c2 = st.columns(2)
            with c1:
                base_url = st.text_input(
                    "OLLAMA_BASE_URL",
                    value=os.getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
                    help="URL of your local Ollama server.",
                )
                os.environ["OLLAMA_BASE_URL"] = base_url
            with c2:
                model = st.text_input(
                    "OLLAMA_MODEL",
                    value=os.getenv("OLLAMA_MODEL", "llama3"),
                    help="Ollama model name (e.g. llama3, mistral, phi3).",
                )
                os.environ["OLLAMA_MODEL"] = model
            st.caption("💡 Ollama must be running: `ollama serve`")

        elif backend_tab == "llamacpp":
            model_path = st.text_input(
                "LLAMACPP_MODEL_PATH",
                value=os.getenv("LLAMACPP_MODEL_PATH", ""),
                placeholder="/absolute/path/to/model.Q4_K_M.gguf",
                help="Absolute path to a GGUF model file. Download from Hugging Face.",
            )
            c1, c2, c3 = st.columns(3)
            with c1:
                n_ctx = st.number_input("N_CTX", value=int(os.getenv("LLAMACPP_N_CTX", "4096")),
                                        min_value=512, max_value=32768, step=512,
                                        help="Context window size in tokens.")
            with c2:
                n_gpu = st.number_input("N_GPU_LAYERS", value=int(os.getenv("LLAMACPP_N_GPU_LAYERS", "0")),
                                        min_value=0, max_value=999,
                                        help="GPU layers (0 = CPU only).")
            with c3:
                max_tok = st.number_input("MAX_TOKENS", value=int(os.getenv("LLAMACPP_MAX_TOKENS", "512")),
                                          min_value=64, max_value=4096, step=64,
                                          help="Max tokens per generation.")
            os.environ["LLAMACPP_MODEL_PATH"] = model_path
            os.environ["LLAMACPP_N_CTX"] = str(n_ctx)
            os.environ["LLAMACPP_N_GPU_LAYERS"] = str(n_gpu)
            os.environ["LLAMACPP_MAX_TOKENS"] = str(max_tok)
            if not model_path:
                st.warning("⚠️ Set LLAMACPP_MODEL_PATH before running.")
            else:
                if Path(model_path).exists():
                    st.success(f"✓ Model file found: {Path(model_path).name}")
                else:
                    st.error(f"✗ Model file not found at: {model_path}")

        elif backend_tab == "openai":
            api_key = st.text_input(
                "OPENAI_API_KEY",
                value=os.getenv("OPENAI_API_KEY", ""),
                type="password",
                help="Your OpenAI API key. Never commit this to source control.",
            )
            model = st.text_input(
                "OPENAI_MODEL",
                value=os.getenv("OPENAI_MODEL", "gpt-4o"),
                help="OpenAI model ID.",
            )
            if api_key:
                os.environ["OPENAI_API_KEY"] = api_key
            os.environ["OPENAI_MODEL"] = model

    with st.expander("📁 Orchestrator Settings"):
        c1, c2 = st.columns(2)
        with c1:
            target_dir = st.text_input(
                "TARGET_CODEBASE_DIR",
                value=os.getenv("TARGET_CODEBASE_DIR", "input"),
                help="Directory the code-explorer sub-agent will scan. Relative paths are resolved from the project root (subagents-demo/).",
            )
            os.environ["TARGET_CODEBASE_DIR"] = target_dir
        with c2:
            output_dir = st.text_input(
                "OUTPUT_DIR",
                value=os.getenv("OUTPUT_DIR", "output"),
                help="Where the final Markdown report will be written.",
            )
            os.environ["OUTPUT_DIR"] = output_dir

        project_name = st.text_input(
            "PROJECT_NAME",
            value=os.getenv("PROJECT_NAME", "Sub-Agents Demo – Sample Codebase"),
            help="Human-readable name that appears at the top of the report.",
        )
        os.environ["PROJECT_NAME"] = project_name

    with st.expander("🪵 Logging"):
        log_level = st.selectbox(
            "LOG_LEVEL",
            options=["DEBUG", "INFO", "WARNING", "ERROR"],
            index=["DEBUG", "INFO", "WARNING", "ERROR"].index(
                os.getenv("LOG_LEVEL", "INFO")
            ),
            help="Verbosity of the Python logger output.",
        )
        os.environ["LOG_LEVEL"] = log_level

# ─────────────────────────────────────────────────────────────────────────────
# Tab 3 – Run Pipeline
# ─────────────────────────────────────────────────────────────────────────────
with tabs[2]:
    st.header("▶️ Run Pipeline")
    st.caption(
        "Click **Run** to execute the full orchestrator pipeline. "
        "Console output (including LLM responses) will appear live below."
    )

    col_run, col_clear = st.columns([1, 5])
    with col_run:
        run_clicked = st.button(
            "▶ Run",
            type="primary",
            disabled=st.session_state.running,
            help="Start the orchestrator. Both sub-agents will execute sequentially.",
        )
    with col_clear:
        if st.button("🗑 Clear output", help="Clear console output and previous results."):
            st.session_state.console_output = ""
            st.session_state.report = None
            st.rerun()

    output_placeholder = st.empty()

    if run_clicked and not st.session_state.running:
        st.session_state.running = True
        st.session_state.console_output = ""
        st.session_state.report = None

        buf = io.StringIO()
        progress = st.progress(0, text="Initialising orchestrator …")

        try:
            # Capture stdout (print statements) during the run
            with redirect_stdout(buf), redirect_stderr(buf):
                orch = Orchestrator()
                progress.progress(10, text="Sub-agents registered …")

                # Monkey-patch execute to update progress
                _original_run = orch.run

                def _instrumented_run():
                    return _original_run()

                report = _instrumented_run()

            st.session_state.report = report
            progress.progress(100, text="Pipeline complete ✓")

        except Exception as exc:  # noqa: BLE001
            buf.write(f"\n\n⚠️  Pipeline error: {type(exc).__name__}: {exc}\n")
            progress.progress(100, text="Pipeline finished with errors.")
        finally:
            st.session_state.running = False
            st.session_state.console_output = buf.getvalue()

        st.rerun()

    # Display captured output
    if st.session_state.console_output:
        output_placeholder.code(st.session_state.console_output, language="text")
    elif not st.session_state.running:
        output_placeholder.info("Press **▶ Run** to start the pipeline.")

    if st.session_state.report:
        r = st.session_state.report
        all_ok = r.all_succeeded
        if all_ok:
            st.success(
                f"✓ Pipeline complete — all {len(r.results)} sub-agent(s) succeeded "
                f"in {r.total_duration_seconds:.1f}s"
            )
        else:
            st.error(
                f"Pipeline finished with failures: {r.failed_agents}"
            )

# ─────────────────────────────────────────────────────────────────────────────
# Tab 4 – Sub-Agent Results
# ─────────────────────────────────────────────────────────────────────────────
with tabs[3]:
    st.header("🔍 Sub-Agent Results")
    st.caption(
        "Inspect the result returned by each sub-agent to the orchestrator. "
        "This mirrors the 'summary returned to Bob' described in the documentation."
    )

    if st.session_state.report is None:
        st.info("Run the pipeline first (▶️ Run Pipeline tab).")
    else:
        report = st.session_state.report
        for result in report.results:
            icon = "✅" if result.success else "❌"
            with st.expander(
                f"{icon} **{result.agent_id}** — {result.duration_seconds:.1f}s",
                expanded=True,
            ):
                c1, c2, c3 = st.columns(3)
                c1.metric("Status", "SUCCESS" if result.success else "FAILED")
                c2.metric("Duration", f"{result.duration_seconds:.2f}s")
                c3.metric("Tools used", len(result.tools_used))

                if result.tools_used:
                    st.markdown("**Tools invoked:**")
                    st.code(" → ".join(result.tools_used))

                if result.success:
                    st.markdown("**LLM Summary returned to orchestrator:**")
                    st.markdown(
                        f"> {result.summary}" if result.summary else "*No summary.*"
                    )

                    if result.data:
                        with st.expander("📦 Raw structured data"):
                            st.json(
                                {k: v for k, v in result.data.items() if k != "markdown"},
                                expanded=False,
                            )
                else:
                    st.error(f"Error: {result.error}")

# ─────────────────────────────────────────────────────────────────────────────
# Tab 5 – Final Report
# ─────────────────────────────────────────────────────────────────────────────
with tabs[4]:
    st.header("📄 Final Report")
    st.caption(
        "The orchestrator aggregates both sub-agent results into this unified "
        "Markdown document and writes it to the output/ directory."
    )

    if st.session_state.report is None:
        st.info("Run the pipeline first (▶️ Run Pipeline tab).")
    else:
        report = st.session_state.report
        md = report.unified_documentation

        if md:
            view_mode = st.radio(
                "View as",
                options=["Rendered Markdown", "Raw Markdown"],
                horizontal=True,
            )
            if view_mode == "Rendered Markdown":
                st.markdown(md)
            else:
                st.code(md, language="markdown")

            # Download button
            ts = report.generated_at.strftime("%Y%m%d_%H%M%S")
            st.download_button(
                label="⬇️ Download report",
                data=md,
                file_name=f"subagents_report_{ts}.md",
                mime="text/markdown",
            )
        else:
            st.warning("No report content available.")

# ─────────────────────────────────────────────────────────────────────────────
# Tab 6 – About Bob Subagents
# ─────────────────────────────────────────────────────────────────────────────
with tabs[5]:
    st.header("📖 About Bob Subagents")
    st.caption("Key concepts from the official IBM Bob documentation, inline.")

    st.markdown(
        """
## What are subagents?

A subagent is an independent agent that Bob can spawn to handle a **focused,
self-contained task**. It runs in its own isolated context window, executes its
assigned work, and returns a **summary of the results** back to the main
conversation.

---

## How subagents work

When Bob spawns a subagent:

1. A new, independent agent is created with its own context window.
2. Bob passes it a **description** of the focused task to perform.
3. You are prompted to approve the spawn before it starts.
4. The subagent executes the task using its available tools.
5. The subagent returns a **summary of its findings** back to Bob.
6. Bob uses that summary to continue the main conversation.

> The subagent runs **separately** from the main conversation. It does not
> share state, files, or tool results with Bob unless Bob explicitly passes
> them in the description.

---

## Subagent types

| Type | Description | Best for |
|---|---|---|
| `explore` | Read-only codebase exploration, lighter model | Searching and summarising code, finding relevant files, understanding structure |
| `general` | Full tool access, default model | Any self-contained task requiring reads, writes, or commands |

---

## Context and conversation history (`fork_context`)

By default, a subagent does **not** see the parent conversation history.
This keeps its context lean and focused.

When a subagent needs to understand prior decisions or constraints, Bob can
set **`fork_context: true`** to pass the conversation history into the
subagent.

In this demo:
- **code-explorer** uses `fork_context=False` (needs no prior context)
- **documentation-writer** uses `fork_context=True` (receives explorer output)

---

## Subagents vs. subtasks

| | Subagent | Subtask |
|---|---|---|
| **Visibility** | Runs silently in the background | Appears in UI with breadcrumb |
| **Interaction** | Not interactive — returns only summary | Fully interactive |
| **Purpose** | Isolated background work | Complex work needing visibility |
| **Output** | Summary returned to Bob | Independent conversation thread |

*Source: [https://bob.ibm.com/docs/ide/features/subagents](https://bob.ibm.com/docs/ide/features/subagents)*
        """
    )
