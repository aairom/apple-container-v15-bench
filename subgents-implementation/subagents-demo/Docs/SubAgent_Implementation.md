# Sub-Agent Implementation Reference

> A thorough, code-grounded technical document explaining every aspect of the
> sub-agent implementation in the **Sub-Agents Demo** codebase.
>
> Every claim in this document has been verified directly against the source code.
> Where a behaviour is inferred rather than explicitly stated in the code, it is
> marked **[inferred]**.

---

## Table of Contents

1. [What a Sub-Agent Is in This Codebase](#1-what-a-sub-agent-is-in-this-codebase)
2. [How Sub-Agents Are Defined and Instantiated](#2-how-sub-agents-are-defined-and-instantiated)
3. [How Sub-Agents Communicate](#3-how-sub-agents-communicate)
4. [The Sub-Agent Lifecycle](#4-the-sub-agent-lifecycle)
5. [Sub-Agent 1 — `code-explorer`](#5-sub-agent-1--code-explorer)
6. [Sub-Agent 2 — `documentation-writer`](#6-sub-agent-2--documentation-writer)
7. [Relationship to the Orchestrator](#7-relationship-to-the-orchestrator)
8. [LLM Integration at the Sub-Agent Level](#8-llm-integration-at-the-sub-agent-level)
9. [Testing Coverage](#9-testing-coverage)

---

## 1. What a Sub-Agent Is in This Codebase

### Conceptual role

In this codebase a **sub-agent** is a self-contained Python object that:

- receives a single, clearly bounded task from the orchestrator (parent agent),
- executes that task entirely within its own `execute()` method,
- returns its findings as a `SubAgentResult` dataclass, and
- shares **no mutable state** with any other agent.

This mirrors the Bob IDE sub-agent concept described at
<https://bob.ibm.com/docs/ide/features/subagents>:

> *"A new, independent agent is created with its own context window. Bob passes it
> a description of the focused task to perform. The subagent executes the task
> using its available tools. The subagent returns a summary of its findings back
> to Bob."*

The demo implements two concrete sub-agents — one of the `explore` type (read-only
codebase analysis) and one of the `general` type (documentation generation) — and
one orchestrator that manages them sequentially.

### What sub-agents are NOT in this codebase

- They are **not** OS processes, threads, or coroutines. They are synchronous Python
  class instances called sequentially within a single process.
- They do **not** have independent network identities or message queues.
- They do **not** share Python object references — the only channel of communication
  is the `SubAgentTask` the orchestrator hands in and the `SubAgentResult` the
  sub-agent hands back.

---

## 2. How Sub-Agents Are Defined and Instantiated

### Class structure

Both sub-agents follow an identical structural pattern. Neither uses inheritance,
metaclasses, or a registry. They are plain Python classes defined in their own
dedicated modules:

| Source file | Class | Agent ID | Agent type |
|---|---|---|---|
| `src/code_explorer_agent.py` | `CodeExplorerAgent` | `"code-explorer"` | `"explore"` |
| `src/documentation_writer_agent.py` | `DocumentationWriterAgent` | `"documentation-writer"` | `"general"` |

The agent identity is stored in module-level constants (`AGENT_ID`, `AGENT_TYPE`)
and copied to instance attributes in `__init__`, making each instance self-describing.

### Construction

Both constructors follow the same three-step pattern:

```python
def __init__(self) -> None:
    self.agent_id   = AGENT_ID        # e.g. "code-explorer"
    self.agent_type = AGENT_TYPE      # e.g. "explore"
    self._task: SubAgentTask | None = None   # no task yet
    self._llm = get_llm_client(agent_label=AGENT_ID)  # LLM callable
```

`get_llm_client()` is called at construction time, **not** at task execution time.
This means the LLM backend is selected and (for `llamacpp`) the model is loaded as
soon as the agent is instantiated. If `LLM_BACKEND=llamacpp`, model loading happens
during `Orchestrator.run()` step 1 (registration), not step 3/5 (execution).
**[inferred: this could slow the registration step noticeably for large GGUF files.]**

### No factory or registry

There is no factory function, service locator, or agent registry. The orchestrator
creates agents with direct constructor calls:

```python
# src/orchestrator.py, inside Orchestrator.run()
explorer   = CodeExplorerAgent()
doc_writer = DocumentationWriterAgent()
```

---

## 3. How Sub-Agents Communicate

All communication flows through three dataclasses defined in `src/models.py`.
There are no shared variables, no global state, and no side-channel communication.

### `SubAgentTask` — orchestrator → sub-agent

```python
@dataclass
class SubAgentTask:
    agent_id:     str               # which agent this task is for
    task_type:    str               # "explore" | "general"
    description:  str               # natural-language task description
    fork_context: bool = False      # whether parent context is forwarded
    payload:      dict[str, Any] = field(default_factory=dict)
```

`fork_context=True` signals that the task payload carries context forwarded from
the parent conversation — specifically the explorer's findings. The flag itself
does not trigger any technical mechanism beyond label semantics; the actual data
is always in `payload`.

**What each sub-agent reads from its payload:**

| Sub-agent | Payload key | Type | Content |
|---|---|---|---|
| `code-explorer` | `"target_directory"` | `str` | Absolute path to the directory to scan |
| `documentation-writer` | `"modules"` | `list[dict]` | AST metadata from the explorer |
| `documentation-writer` | `"project_name"` | `str` | Human-readable project name |
| `documentation-writer` | `"explorer_summary"` | `str` | Explorer's LLM-generated summary |

### `SubAgentResult` — sub-agent → orchestrator

```python
@dataclass
class SubAgentResult:
    agent_id:          str
    success:           bool
    summary:           str               # human-readable outcome sentence
    data:              dict[str, Any] = field(default_factory=dict)
    error:             str | None = None
    duration_seconds:  float = 0.0
    tools_used:        list[str] = field(default_factory=list)
```

**What each sub-agent puts in `data`:**

| Sub-agent | Key | Type | Content |
|---|---|---|---|
| `code-explorer` | `"files_found"` | `int` | Number of `.py` files discovered |
| `code-explorer` | `"modules"` | `list[dict]` | Per-file AST metadata (see §5) |
| `documentation-writer` | `"markdown"` | `str` | The assembled Markdown document |
| `documentation-writer` | `"modules_documented"` | `int` | Count of modules documented |

### `OrchestratorReport` — aggregated by the orchestrator

```python
@dataclass
class OrchestratorReport:
    generated_at:           datetime
    target_directory:       str
    results:                list[SubAgentResult] = field(default_factory=list)
    unified_documentation:  str = ""
    total_duration_seconds: float = 0.0

    @property
    def all_succeeded(self) -> bool: ...
    @property
    def failed_agents(self) -> list[str]: ...
```

The orchestrator appends each `SubAgentResult` to `report.results` immediately
after `execute()` returns, then sets `unified_documentation` to either the
doc-writer's Markdown or a fallback string.

---

## 4. The Sub-Agent Lifecycle

Both sub-agents go through exactly four phases in order. Each phase maps to a
named section in the source code comments.

```
Phase 1 – REGISTRATION       __init__() called by orchestrator
Phase 2 – TASK ASSIGNMENT    assign_task(SubAgentTask) called by orchestrator
Phase 3 – EXECUTION          execute() called by orchestrator → returns SubAgentResult
Phase 4 – RESULT CONSUMPTION orchestrator reads SubAgentResult fields
```

There is no teardown / cleanup phase. The agent instances go out of scope when
`Orchestrator.run()` returns and are garbage-collected normally.

### Phase 1 — Registration

The orchestrator instantiates the sub-agent with no arguments. The constructor:

1. Sets `agent_id` and `agent_type` from module-level constants.
2. Sets `_task = None`.
3. Calls `get_llm_client(agent_label=AGENT_ID)` and stores the returned callable
   in `self._llm`.
4. Logs `"Sub-agent registered: id=… type=…"` at `INFO` level.

At this point the agent cannot do useful work — it has no task.

### Phase 2 — Task Assignment

`assign_task(task: SubAgentTask)` performs one validation:

```python
if task.agent_id != self.agent_id:
    raise ValueError(...)
```

If the task is addressed to a different agent the method raises immediately with a
descriptive `ValueError` naming the mismatched IDs. This is the only explicit
routing guard; there is no type-check on `task_type`. After passing validation the
task is stored as `self._task`.

### Phase 3 — Execution

`execute()` is the agent's primary method. Its internal structure is identical in
both agents:

```
1. Guard: return failed SubAgentResult if self._task is None
2. Start monotonic timer
3. Initialize tools_used = []
4. try:
     a. Do the actual work (different per agent)
     b. Append tool names to tools_used as each logical step completes
     c. Stop timer
     d. Build and return successful SubAgentResult
   except Exception:
     e. Stop timer
     f. Build and return failed SubAgentResult with error message
```

The broad `except Exception` ensures the orchestrator always gets a
`SubAgentResult` back — the orchestrator never has to handle an uncaught exception
from `execute()`. The error is captured as `f"{type(exc).__name__}: {exc}"`.

### Phase 4 — Result consumption

The orchestrator reads `result.success` to decide whether to continue normally
or enter degraded mode. It always appends the result to `report.results`
regardless of success or failure.

---

## 5. Sub-Agent 1 — `code-explorer`

**File:** `src/code_explorer_agent.py`  
**Class:** `CodeExplorerAgent`  
**Agent ID:** `"code-explorer"`  
**Agent type:** `"explore"` (read-only, per Bob documentation)

### Responsibility

Discover all `.py` files in a target directory, parse each one with Python's
built-in `ast` module to extract structural metadata, then ask the LLM to produce
a concise natural-language summary of the codebase. This agent **never writes any
file** — it is strictly read-only.

### Internal execution steps

Inside `execute()`, the agent performs four sequential steps, each adding a
tool name to `tools_used`:

#### Step a — Directory scan (`"directory_scan"`)

```python
target_dir = Path(self._task.payload.get("target_directory", "input"))
py_files = sorted(target_dir.rglob("*.py"))
```

`rglob("*.py")` discovers files recursively. Results are sorted for
deterministic ordering. If the directory contains no `.py` files, the method
returns early with `success=True`, `data={"files_found": 0, "modules": []}`,
and a descriptive summary — no LLM call is made.

#### Step b — AST parsing (`"ast_parser"`)

`_parse_python_file(path: Path) -> dict[str, Any]` is called for each file.
It reads the file as UTF-8 text (replacing undecodable bytes) and calls
`ast.parse()`. If `ast.parse()` raises `SyntaxError`, the error is stored under
`info["parse_error"]` and processing continues to the next file.

The metadata dict returned per file has the following structure:

```python
{
    "filename":         str,   # e.g. "inventory.py"
    "relative_path":    str,   # str(path) as passed in
    "module_docstring": str,   # ast.get_docstring(tree) or ""
    "imports":          list[str],   # module names (Import + ImportFrom)
    "classes":          list[dict],  # see below
    "functions":        list[dict],  # see below
    "lines":            int,   # newline count + 1
    # "parse_error": str  # only present on SyntaxError
}
```

**Class dict structure:**
```python
{
    "name":      str,
    "docstring": str,         # ast.get_docstring(class_node) or ""
    "methods":   list[str],   # names of all FunctionDef nodes anywhere inside the class
}
```

> **Note:** The method list is collected with `ast.walk(node)` rather than by
> inspecting only direct children. This means nested function definitions (e.g.
> functions defined inside methods) are included in the `methods` list.
> **[inferred: for the sample codebase this has no practical impact because no
> methods contain nested function definitions.]**

**Function dict structure** (top-level functions only):
```python
{
    "name":      str,
    "docstring": str,         # ast.get_docstring(func_node) or ""
    "args":      list[str],   # positional argument names
}
```

**ImportFrom handling:** the import is stored as `"module.name"` (e.g.
`"dataclasses.dataclass"`). If `node.module` is `None` (a bare `from . import x`),
the stored string is `".name"`.

#### Step c — LLM summarisation (`"llm_summarise"`)

`_llm_summarise(modules: list[dict]) -> str` builds a compact text block
describing all modules and sends it to the LLM via `self._llm(system, user)`.

The metadata block format per module:
```
### <filename> (<lines> lines)
Docstring: <first 200 chars of module docstring>
Classes: <comma-separated class names>
Top-level functions: <comma-separated function names>
Key imports: <first 10 imports, comma-separated>
```

**System prompt (verbatim from source):**
> *"You are a senior software architect performing a read-only codebase exploration
> (you are acting as a Bob 'explore' sub-agent). Your job is to produce a concise,
> structured summary of the codebase you have been given metadata for. Focus on:
> overall purpose, main components, key patterns, and any notable dependencies.
> Keep the summary under 400 words."*

The LLM's text response becomes the `summary` field of the returned `SubAgentResult`.

#### Step d — Result return

```python
SubAgentResult(
    agent_id="code-explorer",
    success=True,
    summary=<LLM text>,
    data={"files_found": len(py_files), "modules": modules},
    duration_seconds=elapsed,
    tools_used=["directory_scan", "ast_parser", "llm_summarise"],
)
```

### Error handling

Any exception raised during steps a–c is caught by the outer `except Exception`
block. The agent returns a failed `SubAgentResult` with
`error=f"{type(exc).__name__}: {exc}"`. A `SyntaxError` during `ast.parse()` is
handled earlier and does not abort the run — it only populates `info["parse_error"]`
for that file and processing continues.

---

## 6. Sub-Agent 2 — `documentation-writer`

**File:** `src/documentation_writer_agent.py`  
**Class:** `DocumentationWriterAgent`  
**Agent ID:** `"documentation-writer"`  
**Agent type:** `"general"` (full tool access, default model, per Bob documentation)

### Responsibility

Receive the module metadata produced by the `code-explorer`, call the LLM once
per module to generate a Markdown documentation section, then assemble all
sections into a single, publishable Markdown document. This agent **does not
read the filesystem** — it is entirely payload-driven.

### Internal execution steps

Inside `execute()`, the agent performs three sequential steps, each adding a
tool name to `tools_used`:

#### Step a — Payload reading (`"payload_reader"`)

Three values are extracted from `self._task.payload`:

```python
modules:          list[dict]  = payload.get("modules", [])
project_name:     str         = payload.get("project_name", "Unknown Project")
explorer_summary: str         = payload.get("explorer_summary", "")
```

If `modules` is empty, the agent continues normally — `module_sections` will
be an empty list and `_assemble_document()` produces a valid (though minimal)
Markdown document.

#### Step b — Per-module LLM documentation (`"llm_document"`)

`_document_module(module: dict) -> str` is called once for each entry in `modules`.
It constructs a prompt context string summarising the module's metadata:

```
Module: `<filename>`
Lines: <line count>
Module docstring: <first 300 chars>
Classes:
  - Class `<name>`: methods = `<method1>`, `<method2>`, ... (up to 8 methods)
Top-level functions:
  - `<name>(<args>)`: <first 120 chars of docstring>
Key imports: `<import1>`, `<import2>`, ... (up to 12 imports)
```

**System prompt (verbatim from source):**
> *"You are a technical documentation writer acting as a Bob 'general' sub-agent.
> Write clear, concise Markdown documentation for a single Python module. Include:
> a brief purpose statement, description of main classes and their responsibilities,
> description of key functions and their roles, and any notable dependencies. Use
> proper Markdown heading levels (##, ###). Keep the section under 300 words. Do
> NOT add a top-level # heading."*

The LLM's response is returned as a string and appended to `module_sections`.

#### Step c — Markdown assembly (`"markdown_assembler"`)

`_assemble_document(project_name, explorer_summary, module_sections, total_files) -> str`
concatenates all sections with a `"\n\n---\n\n"` divider and wraps them in a
fixed template:

```markdown
# <project_name> – API & Module Documentation

> Auto-generated by the **documentation-writer** sub-agent

---

## Project Overview

<explorer_summary>

---

## Module Reference

*<N> module(s) documented below.*

<section_1>

---

<section_2>

---

*Documentation generated by the Sub-Agents Demo orchestrator.*
```

The `explorer_summary` placed in the **Project Overview** section is the raw LLM
text returned by the `code-explorer` — it is not post-processed by the
`documentation-writer`.

#### Step d — Result return

```python
SubAgentResult(
    agent_id="documentation-writer",
    success=True,
    summary=f"Generated documentation for {len(modules)} module(s) in project '{project_name}'.",
    data={"markdown": markdown, "modules_documented": len(modules)},
    duration_seconds=elapsed,
    tools_used=["payload_reader", "llm_document", "markdown_assembler"],
)
```

### Error handling

The same broad `except Exception` pattern as the explorer. Any exception during
steps a–c produces a failed `SubAgentResult`. In the case of an LLM failure the
`error` string will contain the exception type and message (e.g.
`"ConnectionError: …"` if Ollama is not running).

---

## 7. Relationship to the Orchestrator

### Control flow

The orchestrator (`src/orchestrator.py`, class `Orchestrator`) is the single
entry point for a pipeline run. `Orchestrator.run()` executes a strict
**six-step sequential** procedure:

```
Step 1 – Register sub-agents    (instantiate both agents)
Step 2 – Assign Task 1          (explorer task, fork_context=False)
Step 3 – Execute Sub-Agent 1    (explorer.execute())
Step 4 – Assign Task 2          (doc-writer task, fork_context=True)
Step 5 – Execute Sub-Agent 2    (doc_writer.execute())
Step 6 – Aggregate results      (build OrchestratorReport, write file)
```

The doc-writer task **cannot** be assigned until after the explorer has finished,
because the payload is built from the explorer's result:

```python
# Step 4 payload — built from explorer_result obtained in Step 3
payload={
    "project_name": PROJECT_NAME,
    "modules": explorer_modules,          # explorer_result.data["modules"]
    "explorer_summary": explorer_summary, # explorer_result.summary
}
```

This sequential dependency is the concrete implementation of `fork_context=True`.

### Dependency ordering

- The explorer has **no dependency** on the doc-writer.
- The doc-writer has a **hard data dependency** on the explorer's output.
- If the explorer fails, `explorer_modules` is set to `[]` and `explorer_summary`
  is set to an error string, but the doc-writer is still executed (with an empty
  module list). The resulting document is valid but will have no Module Reference
  sections.

### Result aggregation

After both sub-agents have run:

```python
report.unified_documentation = (
    doc_result.data.get("markdown", "")
    if doc_result.success
    else _fallback_report(explorer_result, doc_result)
)
report.total_duration_seconds = time.monotonic() - overall_start
```

`_fallback_report()` is a module-level helper (not a method) that constructs a
minimal Markdown report listing both agents' statuses and any error messages.

The report is then written to disk:

```python
filename = timestamped_filename("subagents_report")   # e.g. subagents_report_20260101_120000.md
out_path  = write_report(
    content=report.unified_documentation,
    output_dir=self.output_dir,     # resolved absolute path, never relative to cwd
    filename=filename,
)
```

`self.output_dir` is resolved to an absolute path at construction time via
`_resolve_project_path("OUTPUT_DIR", "output")`, which anchors relative paths to
`_PROJECT_ROOT` (the parent of `src/`).

### Console output

The orchestrator prints a live progress trace to stdout during `run()`. Key lines
and their source locations:

| Console line | Source location |
|---|---|
| Banner box | `orchestrator.py:96` — `box("Bob Sub-Agents Demo …")` |
| `✓  Sub-agent registered: [code-explorer]` | `orchestrator.py:107` |
| `✓  Task assigned → [code-explorer]` | `orchestrator.py:158` |
| `⏳  Executing [code-explorer] …` | `orchestrator.py:168` |
| `✓  [code-explorer] completed in …s – tools used: …` | `orchestrator.py:189–191` |
| `── Explorer LLM summary ──` + first 6 lines | `orchestrator.py:196–200` |
| `✓  Task assigned → [documentation-writer] (fork_context=True, N module(s))` | `orchestrator.py:233–234` |
| `⏳  Executing [documentation-writer] …` | `orchestrator.py:240` |
| `✓  [documentation-writer] completed in …s – tools used: …` | `orchestrator.py:252–254` |
| `Modules documented: N` | `orchestrator.py:255–256` |
| Summary table | `orchestrator.py:280–291` |

---

## 8. LLM Integration at the Sub-Agent Level

### Backend selection

Both agents call `get_llm_client(agent_label: str)` from `src/utils.py` in their
constructors. This function reads two environment variables:

```python
backend = (
    os.getenv("LLM_BACKEND")
    or os.getenv("LLM_PROVIDER")   # legacy alias
    or "ollama"
).lower().strip()
```

`LLM_PROVIDER` is accepted only as a legacy fallback. `LLM_BACKEND` takes
precedence when both are set. If neither is set, `"ollama"` is used.

The function dispatches to one of three private factory functions:

| `LLM_BACKEND` value(s) | Factory called | Returns |
|---|---|---|
| `"ollama"` (default) | `_ollama_chat_factory(agent_label)` | closure over `requests.post` |
| `"llamacpp"`, `"llama_cpp"`, `"llama-cpp"` | `_llamacpp_chat_factory(agent_label)` | closure over `Llama` instance |
| `"openai"` | `_openai_chat_factory(agent_label)` | closure over `openai.OpenAI` client |

All three factories return a callable with the signature:
```python
def chat(system_prompt: str, user_message: str) -> str: ...
```

### Ollama backend (`_ollama_chat_factory`)

- Reads `OLLAMA_BASE_URL` (default: `http://localhost:11434`) and `OLLAMA_MODEL`
  (default: `llama3`).
- POSTs to `{base_url}/api/chat` with `{"model": …, "stream": false, "messages": […]}`.
- Extracts `response.json()["message"]["content"]`.
- Calls `_print_llm_response("ollama", agent_label, text)` before returning.
- Timeout is 180 seconds.

### llama.cpp backend (`_llamacpp_chat_factory`)

- Reads `LLAMACPP_MODEL_PATH` (required — raises `EnvironmentError` if empty,
  `FileNotFoundError` if the file does not exist).
- Reads `LLAMACPP_N_CTX` (default `4096`), `LLAMACPP_N_GPU_LAYERS` (default `0`),
  `LLAMACPP_MAX_TOKENS` (default `512`), `LLAMACPP_TEMPERATURE` (default `0.7`).
- Instantiates `llama_cpp.Llama` **at factory-call time** (i.e., at agent
  construction). The `Llama` instance is captured in the closure.
- Uses a ChatML-style prompt template:
  ```
  <|system|>\n{system_prompt}\n<|user|>\n{user_message}\n<|assistant|>\n
  ```
- Calls `llm(prompt, max_tokens=…, temperature=…, stop=["<|user|>", "<|system|>"])`.
- Extracts `output["choices"][0]["text"].strip()`.
- Calls `_print_llm_response("llamacpp", agent_label, text)` before returning.

### OpenAI backend (`_openai_chat_factory`)

- Reads `OPENAI_API_KEY` (raises `EnvironmentError` if not set) and `OPENAI_MODEL`
  (default: `"gpt-4o"`).
- Instantiates `openai.OpenAI(api_key=api_key)` at factory-call time.
- Calls `client.chat.completions.create(model=…, messages=[…])`.
- Extracts `response.choices[0].message.content`.
- Calls `_print_llm_response("openai", agent_label, text)` before returning.

### Console printing (`_print_llm_response`)

Every backend calls `_print_llm_response(backend, agent_label, response)` defined
in `utils.py`. This function always prints to stdout before returning the text to
the caller:

```
\n
────<centered " LLM [<backend>] → <agent_label> ">────
<response, word-wrapped at 72 chars>
────────────────────────────────────────────────────────────────────────\n
```

The display width is the module-level constant `_LLM_RESPONSE_WIDTH = 72`. The
header dashes are computed dynamically so they always total 72 characters. The
footer is always exactly 72 dashes. `sys.stdout.flush()` is called after printing.

### Prompt engineering

Neither sub-agent post-processes the LLM's response. The raw text returned by
`self._llm(system, user)` is used directly as the summary or documentation section.
There is no JSON extraction, structured output parsing, or retry logic. If the LLM
returns an empty string or malformed Markdown the demo proceeds without error —
an empty summary or section is valid from the orchestrator's perspective.

**[inferred: for production use, structured output validation and retry logic would
be appropriate additions.]**

---

## 9. Testing Coverage

All tests are in `tests/test_subagents.py`. The test suite contains **31 test cases**
across 8 test classes. All external I/O — LLM backends, filesystem, HTTP — is fully
mocked; the suite runs with no Ollama instance, no API key, and no GGUF model file.

### Test classes and what they cover

#### `TestSubAgentTask` (2 tests)

| Test | What it verifies |
|---|---|
| `test_defaults` | `fork_context` defaults to `False`; `payload` defaults to `{}` |
| `test_custom_payload` | Custom `fork_context=True` and `payload` round-trip correctly |

#### `TestSubAgentResult` (2 tests)

| Test | What it verifies |
|---|---|
| `test_success_result` | `success=True`, `error=None` |
| `test_failure_result` | `success=False`, `error` contains expected substring |

#### `TestOrchestratorReport` (2 tests)

| Test | What it verifies |
|---|---|
| `test_all_succeeded_true` | `all_succeeded` property returns `True` when all results succeed |
| `test_all_succeeded_false` | `all_succeeded` returns `False`; `failed_agents` contains the failed id |

#### `TestCodeExplorerAgent` (5 tests)

| Test | What it verifies |
|---|---|
| `test_registration` | `agent_id == "code-explorer"`, `agent_type == "explore"` |
| `test_task_assignment_wrong_id_raises` | `ValueError` raised when task's `agent_id` doesn't match |
| `test_execute_without_task_returns_failure` | `success=False`, `"No task assigned"` in error |
| `test_execute_with_real_input_dir` | Runs against real `input/` directory; LLM mocked; verifies `files_found >= 3`, correct `summary`, `"ast_parser"` in `tools_used` |
| `test_execute_empty_directory` | Uses `tmp_path`; verifies early-return with `files_found == 0` |

> **Note:** `test_execute_with_real_input_dir` asserts `files_found >= 3`.  The
> `input/` directory now contains 5 files (`data_processor.py`, `inventory.py`,
> `order_processor.py`, `reporting.py`, `utils.py`), so this assertion still passes.

#### `TestDocumentationWriterAgent` (5 tests)

| Test | What it verifies |
|---|---|
| `test_registration` | `agent_id == "documentation-writer"`, `agent_type == "general"` |
| `test_task_assignment_wrong_id_raises` | `ValueError` raised on ID mismatch |
| `test_execute_without_task_returns_failure` | `success=False`, `"No task assigned"` in error |
| `test_execute_produces_markdown` | LLM mocked; verifies `modules_documented == 1`, `"# Test Project"` in markdown, presence of `"Project Overview"` and `"Module Reference"` headings |
| `test_execute_empty_modules` | Empty `modules` list; verifies `success=True`, `modules_documented == 0` |

#### `TestGetLlmClientBackendSelection` (6 tests)

Tests that `get_llm_client()` routes to the correct private factory function based
on env-var values. Each test patches the private factory and asserts it is called
exactly once with the agent label.

| Test | Scenario |
|---|---|
| `test_default_is_ollama` | Neither `LLM_BACKEND` nor `LLM_PROVIDER` set |
| `test_llm_backend_ollama` | `LLM_BACKEND=ollama` |
| `test_llm_backend_llamacpp` | `LLM_BACKEND=llamacpp` |
| `test_llm_backend_openai` | `LLM_BACKEND=openai` |
| `test_legacy_llm_provider_variable` | `LLM_BACKEND` unset, `LLM_PROVIDER=openai` |
| `test_llamacpp_alias_variants` | All three aliases: `"llama_cpp"`, `"llama-cpp"`, `"llamacpp"` |

#### `TestLlamaCppBackend` (4 tests)

All four tests patch `sys.modules["llama_cpp"]` so no real `llama_cpp` package
is needed.

| Test | Scenario |
|---|---|
| `test_llamacpp_factory_produces_callable` | Creates fake `.gguf` file; verifies factory returns callable, callable returns mocked text, `_print_llm_response` is called |
| `test_llamacpp_missing_model_path_raises` | Empty `LLAMACPP_MODEL_PATH` → `EnvironmentError` |
| `test_llamacpp_nonexistent_model_file_raises` | Nonexistent path → `FileNotFoundError` |
| `test_llamacpp_import_error_raised_clearly` | `llama_cpp` absent from `sys.modules` → `ImportError` or `TypeError` |

#### `TestOllamaBackend` (1 test)

| Test | What it verifies |
|---|---|
| `test_ollama_returns_response_and_prints` | Patches `requests.post`; verifies response text is returned, `_print_llm_response` is called with correct args |

#### `TestPrintLlmResponse` (2 tests)

| Test | What it verifies |
|---|---|
| `test_output_contains_backend_and_label` | stdout contains `"ollama"`, `"code-explorer"`, and response text |
| `test_output_contains_llamacpp_label` | stdout contains `"llamacpp"` and `"documentation-writer"` |

#### `TestUIImport` (2 tests)

| Test | What it verifies |
|---|---|
| `test_ui_module_can_be_parsed` | `src/ui.py` is syntactically valid Python (parsed with `ast.parse`) |
| `test_ui_imports_are_available` | `models`, `orchestrator`, and `utils` are importable from `src/` |

### What is NOT covered by the test suite

| Gap | Notes |
|---|---|
| `Orchestrator.run()` end-to-end | No integration test exercises the full pipeline with both sub-agents chained |
| `_fallback_report()` | The fallback Markdown generator for partial failures is not tested |
| Orchestrator console output | The printed lines (`✓`, `⏳`, banner, summary table) are not asserted |
| `OrchestratorReport.failed_agents` property returning multiple IDs | Only a single-failure scenario is tested |
| OpenAI backend chat callable | `_openai_chat_factory` is tested only at routing level; the returned callable is not exercised |
| `_assemble_document()` with multiple sections | Only a single-module `SAMPLE_MODULES` fixture is used |
| `_parse_python_file()` with `SyntaxError` | The `parse_error` fallback path is not tested |
| File write via `write_report()` | The output directory creation and file write are not tested |
| UI runtime behaviour | The Streamlit UI is only syntax-checked; no tab interaction or pipeline execution through the UI is tested |

---

*This document was generated by cross-referencing every claim against the source
files listed below:*

| File | Role |
|---|---|
| `src/models.py` | Shared DTOs |
| `src/utils.py` | LLM factory, logging, output helpers |
| `src/code_explorer_agent.py` | Sub-Agent 1 source |
| `src/documentation_writer_agent.py` | Sub-Agent 2 source |
| `src/orchestrator.py` | Orchestrator source |
| `src/ui.py` | Streamlit UI |
| `tests/test_subagents.py` | Full test suite |
| `input/inventory.py` | Sample input module 1 |
| `input/order_processor.py` | Sample input module 2 |
| `input/reporting.py` | Sample input module 3 |
| `input/data_processor.py` | Sample input module 4 |
| `input/utils.py` | Sample input module 5 |
| `plugins/rules/AGENTS.md` | Project development rules |
