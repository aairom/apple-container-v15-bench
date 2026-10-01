"""
orchestrator.py – Parent / Orchestrator Agent

This module is the heart of the sub-agents demo.  It implements the full
sub-agent lifecycle as described in the Bob IDE documentation:
  https://bob.ibm.com/docs/ide/features/subagents

Orchestrator responsibilities
------------------------------
1. Load configuration and prepare the shared context.
2. REGISTER both sub-agents (CodeExplorerAgent, DocumentationWriterAgent).
3. ASSIGN tasks – each sub-agent receives a focused, non-overlapping task.
4. EXECUTE sub-agents – in this demo they run sequentially; the explorer
   runs first and its output feeds the documentation-writer.
5. COLLECT results – gather SubAgentResult objects returned by each agent.
6. AGGREGATE – combine both results into a single OrchestratorReport and
   write a timestamped Markdown file to the output/ directory.

Sub-agent lifecycle per Bob documentation
------------------------------------------
  • A new, independent agent is created with its own context window.
  • Bob passes it a description of the focused task to perform.
  • The subagent executes the task using its available tools.
  • The subagent returns a summary of its findings back to Bob.
  • Bob uses that summary to continue the main conversation.
"""

from __future__ import annotations

import os
import sys
import time
from datetime import datetime
from pathlib import Path

# ── ensure src/ is on the path when run directly ─────────────────────────────
_SRC_DIR = Path(__file__).parent          # …/subagents-demo/src
_PROJECT_ROOT = _SRC_DIR.parent           # …/subagents-demo
sys.path.insert(0, str(_SRC_DIR))

from models import OrchestratorReport, SubAgentTask
from code_explorer_agent import CodeExplorerAgent
from documentation_writer_agent import DocumentationWriterAgent
from utils import (
    box,
    get_logger,
    timestamped_filename,
    write_report,
)

logger = get_logger("Orchestrator")


def _resolve_project_path(env_var: str, default: str) -> Path:
    """Return an absolute Path for a directory config value.

    If the env var holds an absolute path it is used as-is.  If it is
    relative it is anchored to the project root (the parent of src/)
    so the orchestrator works correctly regardless of the working
    directory it is launched from (CLI, UI, background process).
    """
    raw = os.getenv(env_var, default)
    p = Path(raw)
    if p.is_absolute():
        return p
    return (_PROJECT_ROOT / p).resolve()


# ─────────────────────────────────────────────────────────────────────────────
# Configuration (read from environment, with sensible defaults)
# ─────────────────────────────────────────────────────────────────────────────

TARGET_DIR = _resolve_project_path("TARGET_CODEBASE_DIR", "input")
OUTPUT_DIR  = _resolve_project_path("OUTPUT_DIR", "output")
PROJECT_NAME = os.getenv("PROJECT_NAME", "Sub-Agents Demo – Sample Codebase")


# ─────────────────────────────────────────────────────────────────────────────
# Orchestrator
# ─────────────────────────────────────────────────────────────────────────────

class Orchestrator:
    """
    Parent agent that manages exactly two sub-agents:

      1. CodeExplorerAgent        (type: explore) – read-only analysis
      2. DocumentationWriterAgent (type: general) – documentation generation

    The orchestrator delegates, collects, and aggregates – it never performs
    the sub-agents' specific work itself.
    """

    def __init__(self) -> None:
        self.target_dir = Path(TARGET_DIR)
        self.output_dir = Path(OUTPUT_DIR)
        logger.info("Orchestrator initialised")
        logger.info("  Target codebase : %s", self.target_dir.resolve())
        logger.info("  Output directory: %s", self.output_dir.resolve())

    # ──────────────────────────────────────────────────────────────────────────
    def run(self) -> OrchestratorReport:
        """
        Main orchestration loop.

        Returns an OrchestratorReport that can be inspected or written to disk.
        """
        overall_start = time.monotonic()
        report = OrchestratorReport(
            generated_at=datetime.now(),
            target_directory=str(self.target_dir.resolve()),
        )

        print()
        print(box("Bob Sub-Agents Demo – Orchestrator starting", "", width=72))
        print()

        # ── STEP 1: Register sub-agents ───────────────────────────────────────
        logger.info("─── Step 1: Registering sub-agents ───────────────────────")

        # Each sub-agent is 'spawned' (instantiated) independently – per Bob docs:
        # "A new, independent agent is created with its own context window."
        explorer = CodeExplorerAgent()           # type: explore
        doc_writer = DocumentationWriterAgent()  # type: general

        print(f"  ✓  Sub-agent registered: [{explorer.agent_id}]   "
              f"(type={explorer.agent_type})")
        print(f"  ✓  Sub-agent registered: [{doc_writer.agent_id}] "
              f"(type={doc_writer.agent_type})")
        print()

        # ── STEP 2: Assign tasks ──────────────────────────────────────────────
        #
        # Per Bob documentation:
        #   "Bob passes it a description of the focused task to perform."
        #
        # Each task is self-contained and non-overlapping:
        #   • explorer   → only reads & analyses files
        #   • doc_writer → only writes documentation (no filesystem reads)
        # ─────────────────────────────────────────────────────────────────────

        logger.info("─── Step 2: Assigning tasks to sub-agents ────────────────")

        # Task for Sub-Agent 1 (explore)
        explorer_task = SubAgentTask(
            agent_id=explorer.agent_id,
            task_type="explore",
            description=(
                "Scan the target Python codebase directory. "
                "For each .py file: extract module metadata (classes, functions, "
                "imports, docstrings). Then produce a concise natural-language "
                "summary of the overall codebase structure and purpose. "
                "Return only a summary – do NOT write any files."
            ),
            fork_context=False,   # explorer needs no parent context
            payload={"target_directory": str(self.target_dir)},
        )
        explorer.assign_task(explorer_task)
        print(f"  ✓  Task assigned → [{explorer.agent_id}]")

        # Task for Sub-Agent 2 (general) – assigned after explorer result is
        # available (sequential dependency shown explicitly here)
        # The payload will be filled once the explorer finishes.

        # ── STEP 3: Execute Sub-Agent 1 (explorer) ───────────────────────────
        logger.info("─── Step 3: Executing sub-agent [%s] ─────────────────────",
                    explorer.agent_id)
        print()
        print(f"  ⏳  Executing [{explorer.agent_id}] …")

        # Per Bob docs: "The subagent executes the task using its available tools."
        explorer_result = explorer.execute()

        # Collect the result
        report.results.append(explorer_result)

        if not explorer_result.success:
            # Graceful error handling: log and continue with degraded output
            logger.error(
                "Sub-agent [%s] failed: %s",
                explorer.agent_id, explorer_result.error,
            )
            print(f"  ✗  [{explorer.agent_id}] FAILED: {explorer_result.error}")
            # Continue – documentation writer will note the missing explorer data
            explorer_modules = []
            explorer_summary = (
                f"[Explorer sub-agent failed: {explorer_result.error}]"
            )
        else:
            print(f"  ✓  [{explorer.agent_id}] completed in "
                  f"{explorer_result.duration_seconds:.1f}s  "
                  f"– tools used: {explorer_result.tools_used}")
            print(f"     Files analysed: {explorer_result.data.get('files_found', 0)}")
            explorer_modules = explorer_result.data.get("modules", [])
            explorer_summary = explorer_result.summary
            # Show the LLM summary produced by the explorer sub-agent
            print(f"     ── Explorer LLM summary ──")
            for line in explorer_summary.splitlines()[:6]:  # first 6 lines preview
                print(f"     {line}")
            if len(explorer_summary.splitlines()) > 6:
                print(f"     … (see report for full text)")

        # ── STEP 4: Assign task to Sub-Agent 2 with explorer's output ─────────
        #
        # This demonstrates the 'fork_context' pattern from the Bob docs:
        #   "When a subagent needs to understand prior decisions, constraints,
        #    or preferences expressed earlier in the conversation, Bob can set
        #    fork_context: true to pass the conversation history into the
        #    subagent."
        #
        # Here we selectively forward only the data the doc-writer needs.
        # ─────────────────────────────────────────────────────────────────────

        logger.info("─── Step 4: Assigning task to sub-agent [%s] ─────────────",
                    doc_writer.agent_id)

        doc_writer_task = SubAgentTask(
            agent_id=doc_writer.agent_id,
            task_type="general",
            description=(
                "Using the code exploration results provided in the payload, "
                "write professional Markdown API documentation for each module. "
                "Assemble everything into a single cohesive project document. "
                "Return the Markdown text as your result."
            ),
            fork_context=True,   # receives explorer's output → fork_context=True
            payload={
                "project_name": PROJECT_NAME,
                "modules": explorer_modules,
                "explorer_summary": explorer_summary,
            },
        )
        doc_writer.assign_task(doc_writer_task)
        print(f"  ✓  Task assigned → [{doc_writer.agent_id}] "
              f"(fork_context=True, {len(explorer_modules)} module(s) forwarded)")

        # ── STEP 5: Execute Sub-Agent 2 (documentation writer) ───────────────
        logger.info("─── Step 5: Executing sub-agent [%s] ─────────────────────",
                    doc_writer.agent_id)
        print()
        print(f"  ⏳  Executing [{doc_writer.agent_id}] …")

        doc_result = doc_writer.execute()
        report.results.append(doc_result)

        if not doc_result.success:
            logger.error(
                "Sub-agent [%s] failed: %s",
                doc_writer.agent_id, doc_result.error,
            )
            print(f"  ✗  [{doc_writer.agent_id}] FAILED: {doc_result.error}")
        else:
            print(f"  ✓  [{doc_writer.agent_id}] completed in "
                  f"{doc_result.duration_seconds:.1f}s  "
                  f"– tools used: {doc_result.tools_used}")
            print(f"     Modules documented: "
                  f"{doc_result.data.get('modules_documented', 0)}")

        # ── STEP 6: Aggregate results into the final report ───────────────────
        #
        # Per Bob docs: "Bob uses that summary to continue the main conversation."
        # Here the orchestrator assembles both summaries into one unified output.
        # ─────────────────────────────────────────────────────────────────────

        logger.info("─── Step 6: Aggregating results ──────────────────────────")

        report.unified_documentation = doc_result.data.get("markdown", "") \
            if doc_result.success else _fallback_report(explorer_result, doc_result)
        report.total_duration_seconds = time.monotonic() - overall_start

        # Write the unified report to output/
        filename = timestamped_filename("subagents_report")
        out_path = write_report(
            content=report.unified_documentation,
            output_dir=self.output_dir,
            filename=filename,
        )

        # ── Final console summary ─────────────────────────────────────────────
        print()
        print("─" * 72)
        print("  ORCHESTRATOR SUMMARY")
        print("─" * 72)
        for r in report.results:
            status = "✓ SUCCESS" if r.success else "✗ FAILED "
            print(f"  {status}  [{r.agent_id}]  "
                  f"{r.duration_seconds:.1f}s  tools={r.tools_used}")
        print()
        print(f"  Total time   : {report.total_duration_seconds:.1f}s")
        print(f"  All succeeded: {report.all_succeeded}")
        print(f"  Report file  : {out_path}")
        print("─" * 72)
        print()

        return report


# ─────────────────────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────────────────────

def _fallback_report(explorer_result, doc_result) -> str:
    """Produce a minimal report when the documentation writer failed."""
    lines = [
        "# Sub-Agents Demo – Partial Report\n",
        "One or more sub-agents encountered errors.\n",
        f"## Explorer Sub-Agent\n",
        f"Status: {'SUCCESS' if explorer_result.success else 'FAILED'}\n",
        explorer_result.summary or explorer_result.error or "",
        f"\n## Documentation Writer Sub-Agent\n",
        f"Status: FAILED\n",
        doc_result.error or "Unknown error",
    ]
    return "\n".join(lines)


# ─────────────────────────────────────────────────────────────────────────────
# Entry point
# ─────────────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    orchestrator = Orchestrator()
    final_report = orchestrator.run()
    sys.exit(0 if final_report.all_succeeded else 1)
