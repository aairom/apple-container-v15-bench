"""
code_explorer_agent.py – Sub-Agent 1: "explore" type

Responsibility
--------------
This sub-agent is responsible for **read-only codebase exploration**.
It mirrors the 'explore' sub-agent type described in the Bob IDE documentation:
  "Read-only codebase exploration, runs on a lighter model.
   Best for: Searching and summarising code, finding relevant files,
   understanding structure."
  (https://bob.ibm.com/docs/ide/features/subagents#subagent-types)

Lifecycle (per Bob documentation)
----------------------------------
1. REGISTRATION  – The orchestrator instantiates the agent and sets its task.
2. TASK ASSIGNMENT – The orchestrator calls `assign_task()` with a SubAgentTask.
3. EXECUTION     – `execute()` runs in an isolated context:
                   it scans the target directory, collects Python file metadata,
                   and uses the LLM to summarise the code structure.
4. RESULT RETURN – `execute()` returns a SubAgentResult summary back to the
                   orchestrator, which uses it to continue the main flow.

Isolation guarantee
-------------------
This agent does NOT write any files and does NOT share state with the
documentation-writer agent.  It only returns a summary + structured data.
"""

from __future__ import annotations

import ast
import os
import time
from pathlib import Path
from typing import Any

from models import SubAgentResult, SubAgentTask
from utils import get_llm_client, get_logger

logger = get_logger("CodeExplorerAgent")

# ── Agent identity ─────────────────────────────────────────────────────────────
AGENT_ID = "code-explorer"
AGENT_TYPE = "explore"   # read-only, lighter model – per Bob docs


class CodeExplorerAgent:
    """
    Sub-Agent 1 – Explore type.

    Scans a directory of Python files, extracts structural metadata (modules,
    classes, functions, docstrings, imports), then asks the LLM to produce
    a concise natural-language summary of what the codebase does.

    The result is returned to the orchestrator; this agent never writes files.
    """

    # ── LIFECYCLE STEP 1: REGISTRATION ────────────────────────────────────────
    def __init__(self) -> None:
        """
        Registration phase.

        The orchestrator creates this instance to 'spawn' the sub-agent into
        existence.  The agent is given an identity and prepares its internal
        state – but has no task yet.
        """
        self.agent_id: str = AGENT_ID
        self.agent_type: str = AGENT_TYPE
        self._task: SubAgentTask | None = None
        # Pass the agent label so console output is clearly attributed
        self._llm = get_llm_client(agent_label=AGENT_ID)
        logger.info("Sub-agent registered: id=%s type=%s", self.agent_id, self.agent_type)

    # ── LIFECYCLE STEP 2: TASK ASSIGNMENT ─────────────────────────────────────
    def assign_task(self, task: SubAgentTask) -> None:
        """
        Task-assignment phase.

        The orchestrator passes a SubAgentTask describing exactly what to do.
        This mirrors the Bob documentation's step where Bob 'passes it a
        description of the focused task to perform'.

        The agent validates that the task is meant for it, then stores it.
        """
        if task.agent_id != self.agent_id:
            raise ValueError(
                f"Task addressed to '{task.agent_id}', "
                f"but this agent is '{self.agent_id}'"
            )
        self._task = task
        logger.info(
            "Task assigned to %s: %s",
            self.agent_id,
            task.description[:80] + ("…" if len(task.description) > 80 else ""),
        )

    # ── LIFECYCLE STEP 3: EXECUTION ───────────────────────────────────────────
    def execute(self) -> SubAgentResult:
        """
        Execution phase – runs in an isolated context.

        Steps performed:
          a) Validate task is assigned.
          b) Scan the target directory for Python files (read-only).
          c) Parse each file with ast to extract structural metadata.
          d) Send the metadata to the LLM for a natural-language summary.
          e) Package findings into a SubAgentResult and return to caller.

        On any error the agent returns a failed SubAgentResult with a
        meaningful error message rather than raising an exception, so the
        orchestrator can handle it gracefully.
        """
        if self._task is None:
            return SubAgentResult(
                agent_id=self.agent_id,
                success=False,
                summary="",
                error="No task assigned before execute() was called.",
            )

        start_time = time.monotonic()
        tools_used: list[str] = []

        try:
            target_dir = Path(self._task.payload.get("target_directory", "input"))
            logger.info("Exploring directory: %s", target_dir.resolve())

            # ── a) Discover Python files ──────────────────────────────────────
            tools_used.append("directory_scan")
            py_files = sorted(target_dir.rglob("*.py"))
            logger.info("Found %d Python file(s)", len(py_files))

            if not py_files:
                elapsed = time.monotonic() - start_time
                return SubAgentResult(
                    agent_id=self.agent_id,
                    success=True,
                    summary=(
                        f"No Python files found in '{target_dir}'. "
                        "The directory may be empty or contain non-Python sources."
                    ),
                    data={"files_found": 0, "modules": []},
                    duration_seconds=elapsed,
                    tools_used=tools_used,
                )

            # ── b) Parse each file with AST (read-only tool) ──────────────────
            tools_used.append("ast_parser")
            modules: list[dict[str, Any]] = []
            for py_path in py_files:
                module_info = self._parse_python_file(py_path)
                modules.append(module_info)
                logger.debug("Parsed: %s → %d function(s), %d class(es)",
                             py_path.name,
                             len(module_info["functions"]),
                             len(module_info["classes"]))

            # ── c) Summarise via LLM ──────────────────────────────────────────
            tools_used.append("llm_summarise")
            summary_text = self._llm_summarise(modules)

            elapsed = time.monotonic() - start_time

            # ── d) Return result to orchestrator ──────────────────────────────
            result = SubAgentResult(
                agent_id=self.agent_id,
                success=True,
                summary=summary_text,
                data={
                    "files_found": len(py_files),
                    "modules": modules,
                },
                duration_seconds=elapsed,
                tools_used=tools_used,
            )
            logger.info(
                "Sub-agent %s completed in %.2fs – analysed %d file(s)",
                self.agent_id, elapsed, len(py_files),
            )
            return result

        except Exception as exc:  # noqa: BLE001
            elapsed = time.monotonic() - start_time
            error_msg = f"{type(exc).__name__}: {exc}"
            logger.error("Sub-agent %s failed: %s", self.agent_id, error_msg)
            return SubAgentResult(
                agent_id=self.agent_id,
                success=False,
                summary="",
                error=error_msg,
                duration_seconds=elapsed,
                tools_used=tools_used,
            )

    # ── private helpers ────────────────────────────────────────────────────────

    def _parse_python_file(self, path: Path) -> dict[str, Any]:
        """
        Use the `ast` module to extract structural metadata from a Python file.
        This is a read-only operation: no file is modified.
        """
        source = path.read_text(encoding="utf-8", errors="replace")
        info: dict[str, Any] = {
            "filename": path.name,
            "relative_path": str(path),
            "module_docstring": "",
            "imports": [],
            "classes": [],
            "functions": [],
            "lines": source.count("\n") + 1,
        }

        try:
            tree = ast.parse(source)
        except SyntaxError as exc:
            info["parse_error"] = str(exc)
            return info

        # Module-level docstring
        info["module_docstring"] = ast.get_docstring(tree) or ""

        for node in ast.walk(tree):
            # Top-level imports
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                if isinstance(node, ast.Import):
                    for alias in node.names:
                        info["imports"].append(alias.name)
                else:
                    module = node.module or ""
                    for alias in node.names:
                        info["imports"].append(f"{module}.{alias.name}")

            # Class definitions
            elif isinstance(node, ast.ClassDef):
                info["classes"].append({
                    "name": node.name,
                    "docstring": ast.get_docstring(node) or "",
                    "methods": [
                        n.name for n in ast.walk(node)
                        if isinstance(n, ast.FunctionDef)
                    ],
                })

            # Top-level function definitions
            elif isinstance(node, ast.FunctionDef) and isinstance(
                getattr(node, "parent", None), type(None)
            ):
                info["functions"].append({
                    "name": node.name,
                    "docstring": ast.get_docstring(node) or "",
                    "args": [a.arg for a in node.args.args],
                })

        return info

    def _llm_summarise(self, modules: list[dict[str, Any]]) -> str:
        """
        Send the extracted metadata to the LLM and ask for a concise summary
        of the overall codebase structure and purpose.
        """
        # Build a compact text representation of the metadata
        lines: list[str] = ["## Codebase Metadata\n"]
        for m in modules:
            lines.append(f"### {m['filename']} ({m['lines']} lines)")
            if m.get("module_docstring"):
                lines.append(f"Docstring: {m['module_docstring'][:200]}")
            if m.get("classes"):
                class_names = ", ".join(c["name"] for c in m["classes"])
                lines.append(f"Classes: {class_names}")
            if m.get("functions"):
                fn_names = ", ".join(f["name"] for f in m["functions"])
                lines.append(f"Top-level functions: {fn_names}")
            if m.get("imports"):
                imp_sample = ", ".join(m["imports"][:10])
                lines.append(f"Key imports: {imp_sample}")
            lines.append("")

        metadata_text = "\n".join(lines)

        system_prompt = (
            "You are a senior software architect performing a read-only codebase "
            "exploration (you are acting as a Bob 'explore' sub-agent). "
            "Your job is to produce a concise, structured summary of the codebase "
            "you have been given metadata for. "
            "Focus on: overall purpose, main components, key patterns, and any "
            "notable dependencies. Keep the summary under 400 words."
        )

        user_message = (
            f"Please summarise the following Python codebase metadata:\n\n"
            f"{metadata_text}"
        )

        return self._llm(system_prompt, user_message)
