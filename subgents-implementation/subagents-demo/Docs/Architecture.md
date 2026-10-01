# Architecture

> Sub-Agents Demo – system structure and execution flow

---

## Component Overview

The application consists of five Python modules, three agent roles, and a Streamlit web UI.

```
subagents-demo/src/
├── models.py                     # DTOs shared across all agents
├── utils.py                      # LLM factory, logging, output helpers
├── code_explorer_agent.py        # Sub-Agent 1: explore type
├── documentation_writer_agent.py # Sub-Agent 2: general type
├── orchestrator.py               # Parent / orchestrator agent
└── ui.py                         # Streamlit web UI (6-tab browser interface)
```

---

## High-Level Architecture

```mermaid
graph TD
    subgraph Orchestrator["Orchestrator (orchestrator.py)"]
        O1[Load config<br/>from .env]
        O2[Register sub-agents]
        O3[Assign Task 1<br/>to Explorer]
        O4[Execute Explorer]
        O5[Collect explorer result]
        O6[Assign Task 2<br/>to Doc-Writer<br/>fork_context=True]
        O7[Execute Doc-Writer]
        O8[Collect doc result]
        O9[Aggregate → OrchestratorReport]
        O10[Write timestamped<br/>Markdown to output/]
    end

    subgraph SubAgent1["Sub-Agent 1 – code-explorer (explore type)"]
        E1[Scan directory<br/>for .py files]
        E2[Parse with ast<br/>read-only]
        E3[LLM summarise<br/>structure]
        E4[Return SubAgentResult]
    end

    subgraph SubAgent2["Sub-Agent 2 – documentation-writer (general type)"]
        D1[Read modules<br/>from payload]
        D2[LLM document<br/>each module]
        D3[Assemble<br/>Markdown document]
        D4[Return SubAgentResult]
    end

    subgraph Storage
        IN[(input/ .py files)]
        OUT[(output/ report.md)]
        LLM[[LLM<br/>Ollama / llama.cpp / OpenAI]]
    end

    O1 --> O2 --> O3 --> O4
    O4 --> E1 --> E2 --> E3 --> E4
    E4 --> O5 --> O6 --> O7
    O7 --> D1 --> D2 --> D3 --> D4
    D4 --> O8 --> O9 --> O10

    E1 -->|reads| IN
    E3 -->|calls| LLM
    D2 -->|calls| LLM
    O10 -->|writes| OUT
```

---

## Sub-Agent Lifecycle (per Bob documentation)

```mermaid
sequenceDiagram
    participant O  as Orchestrator
    participant E  as code-explorer<br/>(explore)
    participant D  as documentation-writer<br/>(general)
    participant FS as Filesystem
    participant LLM as LLM Backend

    Note over O: Step 1 – Registration
    O->>E: instantiate CodeExplorerAgent()
    O->>D: instantiate DocumentationWriterAgent()

    Note over O,E: Step 2 – Task Assignment
    O->>E: assign_task(SubAgentTask)<br/>fork_context=False

    Note over O,E: Step 3 – Execute Sub-Agent 1
    O->>E: execute()
    activate E
    E->>FS: scan input/ for .py files
    FS-->>E: [data_processor.py, inventory.py, order_processor.py, reporting.py, utils.py]
    E->>E: ast.parse() each file (read-only)
    E->>LLM: summarise(metadata)
    LLM-->>E: natural-language summary
    E-->>O: SubAgentResult(success=True, summary=…, data={modules:…})
    deactivate E

    Note over O,D: Step 4 – Task Assignment (fork_context=True)
    O->>D: assign_task(SubAgentTask)<br/>payload = explorer.data<br/>fork_context=True

    Note over O,D: Step 5 – Execute Sub-Agent 2
    O->>D: execute()
    activate D
    D->>D: read modules from payload
    D->>LLM: document_module() × 3
    LLM-->>D: Markdown sections
    D->>D: assemble_document()
    D-->>O: SubAgentResult(success=True, data={markdown:…})
    deactivate D

    Note over O: Step 6 – Aggregate
    O->>O: build OrchestratorReport
    O->>FS: write output/subagents_report_<ts>.md
```

---

## Data Flow

```mermaid
flowchart LR
    A[input/*.py] -->|directory_scan + ast_parser| B(SubAgentResult\nfrom explorer)
    B -->|forwarded via\nSubAgentTask.payload| C(SubAgentResult\nfrom doc-writer)
    C --> D[OrchestratorReport]
    D --> E[output/subagents_report_*.md]
```

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| Agents are plain Python classes, not threads | Keeps the demo simple and portable; the key concepts (isolation, delegation, result-return) are fully demonstrated regardless of concurrency model |
| Explorer runs first, writer second (sequential) | The writer explicitly depends on the explorer's output – this demonstrates the `fork_context` pattern cleanly |
| LLM abstracted behind a callable | Allows switching between Ollama (default, no API key), llama.cpp (fully offline GGUF), and OpenAI with a single `LLM_BACKEND` env-var change |
| `SubAgentTask.fork_context` flag | Mirrors the exact `fork_context` parameter described in the Bob documentation |
| `requests` library for HTTP | Avoids `urllib` dynamic URL risks flagged by static analysis |
