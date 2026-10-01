"""
documentation_writer_agent.py – Sub-Agent 2: "general" type

Responsibility
--------------
This sub-agent is responsible for **generating structured Markdown documentation**
from the code-exploration findings produced by Sub-Agent 1.

It mirrors the 'general' sub-agent type described in the Bob IDE documentation:
  "Full tool access, runs on the default model.
   Best for: Any self-contained task requiring reads, writes, or commands."
  (https://bob.ibm.com/docs/ide/features/subagents#subagent-types)

Lifecycle (per Bob documentation)
----------------------------------
1. REGISTRATION  – Orchestrator instantiates the agent and sets its identity.
2. TASK ASSIGNMENT – Orchestrator calls `assign_task()` with a SubAgentTask
                     that carries the explorer's findings in its payload.
3. EXECUTION     – `execute()` runs in its own isolated context:
                   it calls the LLM to turn raw exploration data into
                   polished per-module documentation, then assembles a
                   full Markdown document.
4. RESULT RETURN – Returns a SubAgentResult containing the Markdown text
                   back to the orchestrator.

Isolation guarantee
-------------------
This agent does NOT re-read the filesystem.  It only consumes the structured
data passed by the orchestrator in the task payload (the 'fork_context'
pattern).  It does NOT share state with the explorer agent.
"""

from __future__ import annotations

import time
from pathlib import Path
from typing import Any

from models import SubAgentResult, SubAgentTask
from utils import get_llm_client, get_logger

logger = get_logger("DocumentationWriterAgent")

# ── Agent identity ─────────────────────────────────────────────────────────────
AGENT_ID = "documentation-writer"
AGENT_TYPE = "general"   # full tool access, default model – per Bob docs


class DocumentationWriterAgent:
    """
    Sub-Agent 2 – General type.

    Receives structured code-exploration data from the orchestrator (via the
    task payload), calls the LLM to write polished Markdown documentation for
    each module, and assembles a complete project-level document.

    The returned SubAgentResult contains the finished Markdown string which the
    orchestrator writes to the output/ directory.
    """

    # ── LIFECYCLE STEP 1: REGISTRATION ────────────────────────────────────────
    def __init__(self) -> None:
        """
        Registration phase.

        The orchestrator spawns this sub-agent by instantiating it.
        The agent prepares its internal state but does not yet have a task.
        """
        self.agent_id: str = AGENT_ID
        self.agent_type: str = AGENT_TYPE
        self._task: SubAgentTask | None = None
        # Pass the agent label so console output is clearly attributed
        self._llm = get_llm_client(agent_label=AGENT_ID)
        logger.info(
            "Sub-agent registered: id=%s type=%s", self.agent_id, self.agent_type
        )

    # ── LIFECYCLE STEP 2: TASK ASSIGNMENT ─────────────────────────────────────
    def assign_task(self, task: SubAgentTask) -> None:
        """
        Task-assignment phase.

        The orchestrator passes a SubAgentTask containing the explorer's output
        in `task.payload["exploration_data"]`.  This is the Bob 'fork_context'
        pattern: the parent explicitly forwards only the data this agent needs.
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
          a) Extract exploration data from the task payload.
          b) Generate a per-module documentation section via LLM.
          c) Assemble a full Markdown project document.
          d) Return the finished Markdown in a SubAgentResult.

        On any error the agent returns a failed SubAgentResult with a
        meaningful message so the orchestrator can handle it gracefully.
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
            # ── a) Extract exploration data from the task payload ─────────────
            tools_used.append("payload_reader")
            payload = self._task.payload
            modules: list[dict[str, Any]] = payload.get("modules", [])
            project_name: str = payload.get("project_name", "Unknown Project")
            explorer_summary: str = payload.get("explorer_summary", "")

            logger.info(
                "Writing documentation for project '%s' – %d module(s)",
                project_name, len(modules),
            )

            # ── b) Generate per-module sections ──────────────────────────────
            tools_used.append("llm_document")
            module_sections: list[str] = []
            for mod in modules:
                section = self._document_module(mod)
                module_sections.append(section)
                logger.debug("Documented module: %s", mod.get("filename", "?"))

            # ── c) Assemble full Markdown document ────────────────────────────
            tools_used.append("markdown_assembler")
            markdown = self._assemble_document(
                project_name=project_name,
                explorer_summary=explorer_summary,
                module_sections=module_sections,
                total_files=len(modules),
            )

            elapsed = time.monotonic() - start_time

            # ── d) Return result to orchestrator ──────────────────────────────
            result = SubAgentResult(
                agent_id=self.agent_id,
                success=True,
                summary=(
                    f"Generated documentation for {len(modules)} module(s) "
                    f"in project '{project_name}'."
                ),
                data={"markdown": markdown, "modules_documented": len(modules)},
                duration_seconds=elapsed,
                tools_used=tools_used,
            )
            logger.info(
                "Sub-agent %s completed in %.2fs – documented %d module(s)",
                self.agent_id, elapsed, len(modules),
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

    def _document_module(self, module: dict[str, Any]) -> str:
        """
        Ask the LLM to produce a Markdown documentation section for one module.
        """
        # Build a concise metadata description
        classes_info = ""
        if module.get("classes"):
            classes_info = "\n".join(
                f"  - Class `{c['name']}`: methods = "
                f"{', '.join('`' + m + '`' for m in c['methods'][:8])}"
                for c in module["classes"]
            )

        functions_info = ""
        if module.get("functions"):
            functions_info = "\n".join(
                f"  - `{f['name']}({', '.join(f['args'])})`: "
                f"{f.get('docstring', '')[:120]}"
                for f in module["functions"]
            )

        imports_info = ", ".join(
            f"`{i}`" for i in (module.get("imports") or [])[:12]
        )

        prompt_context = (
            f"Module: `{module.get('filename', 'unknown')}`\n"
            f"Lines: {module.get('lines', '?')}\n"
            f"Module docstring: {module.get('module_docstring', 'None')[:300]}\n"
            f"Classes:\n{classes_info or '  None'}\n"
            f"Top-level functions:\n{functions_info or '  None'}\n"
            f"Key imports: {imports_info or 'None'}\n"
        )

        system_prompt = (
            "You are a technical documentation writer acting as a Bob 'general' "
            "sub-agent. Write clear, concise Markdown documentation for a single "
            "Python module. "
            "Include: a brief purpose statement, description of main classes and "
            "their responsibilities, description of key functions and their roles, "
            "and any notable dependencies. "
            "Use proper Markdown heading levels (##, ###). "
            "Keep the section under 300 words. Do NOT add a top-level # heading."
        )

        user_message = (
            f"Write a Markdown documentation section for this Python module:\n\n"
            f"{prompt_context}"
        )

        return self._llm(system_prompt, user_message)

    def _assemble_document(
        self,
        project_name: str,
        explorer_summary: str,
        module_sections: list[str],
        total_files: int,
    ) -> str:
        """
        Assemble the per-module sections into a single Markdown project document.
        """
        divider = "\n\n---\n\n"
        sections_body = divider.join(module_sections)

        document = (
            f"# {project_name} – API & Module Documentation\n\n"
            f"> Auto-generated by the **documentation-writer** sub-agent\n\n"
            f"---\n\n"
            f"## Project Overview\n\n"
            f"{explorer_summary}\n\n"
            f"---\n\n"
            f"## Module Reference\n\n"
            f"*{total_files} module(s) documented below.*\n\n"
            f"{sections_body}\n\n"
            f"---\n\n"
            f"*Documentation generated by the Sub-Agents Demo orchestrator.*\n"
        )
        return document
