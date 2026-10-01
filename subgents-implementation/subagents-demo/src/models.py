"""
models.py – Shared data-transfer objects used by the orchestrator and both sub-agents.

These lightweight dataclasses mirror the conceptual model described in the Bob
sub-agents documentation:

  • SubAgentTask   – what the orchestrator hands off to a sub-agent
  • SubAgentResult – what a sub-agent hands back to the orchestrator
  • OrchestratorReport – the unified output the orchestrator assembles
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any


@dataclass
class SubAgentTask:
    """
    Encapsulates a focused, self-contained task that the orchestrator delegates
    to a sub-agent.  Mirrors the 'description' parameter passed when Bob spawns
    a subagent (see: https://bob.ibm.com/docs/ide/features/subagents).

    Attributes
    ----------
    agent_id : str
        Logical name of the sub-agent being targeted.
    task_type : str
        Short label for the category of work (e.g. "explore", "general").
    description : str
        Full natural-language description of the work to perform.
    fork_context : bool
        When True the sub-agent receives the orchestrator's accumulated context
        (mirrors the fork_context flag in the Bob spawn_subagent call).
    payload : dict[str, Any]
        Arbitrary data the sub-agent may need (file paths, config, etc.).
    """

    agent_id: str
    task_type: str          # "explore" | "general"
    description: str
    fork_context: bool = False
    payload: dict[str, Any] = field(default_factory=dict)


@dataclass
class SubAgentResult:
    """
    Encapsulates the summary a sub-agent returns to the orchestrator after it
    finishes its isolated work.

    Attributes
    ----------
    agent_id : str
        Identifies which sub-agent produced this result.
    success : bool
        True when the sub-agent completed without error.
    summary : str
        Human-readable summary of what was accomplished (the 'results back to
        the main conversation' as described by the Bob documentation).
    data : dict[str, Any]
        Structured output produced by the sub-agent.
    error : str | None
        Error message when success is False.
    duration_seconds : float
        Wall-clock time the sub-agent spent on its task.
    tools_used : list[str]
        Names of tools the sub-agent invoked during execution.
    """

    agent_id: str
    success: bool
    summary: str
    data: dict[str, Any] = field(default_factory=dict)
    error: str | None = None
    duration_seconds: float = 0.0
    tools_used: list[str] = field(default_factory=list)


@dataclass
class OrchestratorReport:
    """
    The unified output the orchestrator assembles by aggregating every
    sub-agent result.  This is written to the output/ directory as a
    timestamped Markdown file.

    Attributes
    ----------
    generated_at : datetime
        Timestamp of report generation.
    target_directory : str
        Codebase directory that was analysed.
    results : list[SubAgentResult]
        One entry per sub-agent that ran.
    unified_documentation : str
        Final Markdown documentation produced by aggregation.
    total_duration_seconds : float
        Sum of all sub-agent durations.
    """

    generated_at: datetime
    target_directory: str
    results: list[SubAgentResult] = field(default_factory=list)
    unified_documentation: str = ""
    total_duration_seconds: float = 0.0

    # ── convenience helpers ───────────────────────────────────────────────────

    @property
    def all_succeeded(self) -> bool:
        return all(r.success for r in self.results)

    @property
    def failed_agents(self) -> list[str]:
        return [r.agent_id for r in self.results if not r.success]
