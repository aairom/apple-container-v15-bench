"""
utils.py – Shared helpers: logging, LLM backend factory, output utilities.

LLM Backend selection
---------------------
Set the environment variable  LLM_BACKEND  to choose a backend:

  LLM_BACKEND=ollama      (default) – Ollama local HTTP API
  LLM_BACKEND=llamacpp    – llama-cpp-python in-process inference
  LLM_BACKEND=openai      – OpenAI Chat Completions API

Each factory returns a callable with signature:
    chat(system_prompt: str, user_message: str) -> str

The callable prints its response to the console so LLM output is always
visible during a run (prefixed with the backend name for clarity).
"""

from __future__ import annotations

import json
import logging
import os
import sys
import textwrap
from datetime import datetime
from pathlib import Path
from typing import Any, Callable

# ── optional third-party imports (fail gracefully) ────────────────────────────
try:
    from dotenv import load_dotenv  # type: ignore
    load_dotenv()
except ImportError:
    pass  # python-dotenv not installed; rely on real env vars

# ─────────────────────────────────────────────────────────────────────────────
# Logging
# ─────────────────────────────────────────────────────────────────────────────

LOG_FORMAT = "%(asctime)s  [%(levelname)-8s]  %(name)s  –  %(message)s"
DATE_FORMAT = "%Y-%m-%d %H:%M:%S"

_LLM_RESPONSE_WIDTH = 72  # console display width for LLM output blocks


def get_logger(name: str) -> logging.Logger:
    """Return a consistently-formatted logger for the given name."""
    level_name = os.getenv("LOG_LEVEL", "INFO").upper()
    level = getattr(logging, level_name, logging.INFO)

    logger = logging.getLogger(name)
    if not logger.handlers:
        handler = logging.StreamHandler()
        handler.setFormatter(logging.Formatter(LOG_FORMAT, DATE_FORMAT))
        logger.addHandler(handler)
    logger.setLevel(level)
    return logger


# ─────────────────────────────────────────────────────────────────────────────
# LLM response display
# ─────────────────────────────────────────────────────────────────────────────

def _print_llm_response(backend: str, agent_label: str, response: str) -> None:
    """
    Print an LLM response to stdout so it is always visible in the console.

    Output format:
        ── LLM response [backend] → agent_label ──
        <response text, word-wrapped>
        ──────────────────────────────────────────
    """
    w = _LLM_RESPONSE_WIDTH
    header = f" LLM [{backend}] → {agent_label} "
    dashes_l = (w - len(header)) // 2
    dashes_r = w - len(header) - dashes_l
    print(f"\n{'─' * dashes_l}{header}{'─' * dashes_r}")
    # Word-wrap the response so long lines don't overflow narrow terminals
    for paragraph in response.split("\n"):
        if paragraph.strip():
            print(textwrap.fill(paragraph, width=w))
        else:
            print()
    print("─" * w + "\n")
    sys.stdout.flush()


# ─────────────────────────────────────────────────────────────────────────────
# LLM client factory
# ─────────────────────────────────────────────────────────────────────────────

def get_llm_client(agent_label: str = "agent") -> Callable[[str, str], str]:
    """
    Return a chat callable for the configured LLM backend.

    Reads the environment variable  LLM_BACKEND  (falling back to the legacy
    LLM_PROVIDER for backward compatibility).

    Supported values
    ----------------
    ollama    – Ollama local HTTP API (default)
    llamacpp  – llama-cpp-python in-process inference
    openai    – OpenAI Chat Completions API

    The returned callable always prints its response to the console.
    """
    # Support both the new LLM_BACKEND and the legacy LLM_PROVIDER variable
    backend = (
        os.getenv("LLM_BACKEND")
        or os.getenv("LLM_PROVIDER")
        or "ollama"
    ).lower().strip()

    logger = get_logger("utils.llm_factory")
    logger.info("LLM backend selected: %s  (agent=%s)", backend, agent_label)

    if backend in ("llamacpp", "llama_cpp", "llama-cpp"):
        return _llamacpp_chat_factory(agent_label)
    elif backend == "openai":
        return _openai_chat_factory(agent_label)
    else:
        # Default: ollama
        return _ollama_chat_factory(agent_label)


# ─────────────────────────────────────────────────────────────────────────────
# Backend implementations
# ─────────────────────────────────────────────────────────────────────────────

def _ollama_chat_factory(agent_label: str) -> Callable[[str, str], str]:
    """
    Ollama local HTTP API backend.

    Environment variables:
      OLLAMA_BASE_URL  – base URL (default: http://localhost:11434)
      OLLAMA_MODEL     – model name (default: llama3)
    """
    try:
        import requests as req_lib  # type: ignore
    except ImportError as exc:
        raise ImportError(
            "requests package not installed. Run: pip install requests"
        ) from exc

    base_url = os.getenv("OLLAMA_BASE_URL", "http://localhost:11434")
    model = os.getenv("OLLAMA_MODEL", "llama3")
    logger = get_logger("utils.ollama")

    def chat(system_prompt: str, user_message: str) -> str:
        payload = {
            "model": model,
            "stream": False,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user",   "content": user_message},
            ],
        }
        logger.info("  [ollama] Calling model=%s …", model)
        response = req_lib.post(
            f"{base_url}/api/chat",
            json=payload,
            timeout=180,
        )
        response.raise_for_status()
        text = response.json()["message"]["content"]
        # Print LLM response so it is visible in the console
        _print_llm_response("ollama", agent_label, text)
        return text

    return chat


def _llamacpp_chat_factory(agent_label: str) -> Callable[[str, str], str]:
    """
    llama-cpp-python in-process inference backend.

    Environment variables:
      LLAMACPP_MODEL_PATH  – absolute path to the GGUF model file (required)
      LLAMACPP_N_CTX       – context window size (default: 4096)
      LLAMACPP_N_GPU_LAYERS – GPU layers to offload (default: 0 = CPU only)
      LLAMACPP_MAX_TOKENS  – max tokens to generate (default: 512)
      LLAMACPP_TEMPERATURE – sampling temperature (default: 0.7)

    Install:
        pip install llama-cpp-python>=0.2.90
    """
    try:
        from llama_cpp import Llama  # type: ignore
    except ImportError as exc:
        raise ImportError(
            "llama-cpp-python not installed.\n"
            "Install with:  pip install llama-cpp-python>=0.2.90\n"
            "For GPU support see: https://github.com/abetlen/llama-cpp-python"
        ) from exc

    model_path = os.getenv("LLAMACPP_MODEL_PATH", "")
    if not model_path:
        raise EnvironmentError(
            "LLAMACPP_MODEL_PATH is not set.\n"
            "Download a GGUF model (e.g. llama-3-8b-instruct.Q4_K_M.gguf) and "
            "set this variable to its absolute path."
        )
    if not Path(model_path).exists():
        raise FileNotFoundError(
            f"GGUF model not found at: {model_path}\n"
            "Download a GGUF model from https://huggingface.co and set "
            "LLAMACPP_MODEL_PATH to its path."
        )

    n_ctx          = int(os.getenv("LLAMACPP_N_CTX", "4096"))
    n_gpu_layers   = int(os.getenv("LLAMACPP_N_GPU_LAYERS", "0"))
    max_tokens     = int(os.getenv("LLAMACPP_MAX_TOKENS", "512"))
    temperature    = float(os.getenv("LLAMACPP_TEMPERATURE", "0.7"))

    logger = get_logger("utils.llamacpp")
    logger.info("  [llamacpp] Loading model from %s …", model_path)

    # Load the model once; this object is captured in the closure
    llm = Llama(
        model_path=model_path,
        n_ctx=n_ctx,
        n_gpu_layers=n_gpu_layers,
        verbose=False,
    )
    logger.info("  [llamacpp] Model loaded (n_ctx=%d, n_gpu_layers=%d)",
                n_ctx, n_gpu_layers)

    def chat(system_prompt: str, user_message: str) -> str:
        # Use the ChatML / llama-3 instruct prompt format
        prompt = (
            f"<|system|>\n{system_prompt}\n"
            f"<|user|>\n{user_message}\n"
            f"<|assistant|>\n"
        )
        logger.info("  [llamacpp] Generating (max_tokens=%d) …", max_tokens)
        output = llm(
            prompt,
            max_tokens=max_tokens,
            temperature=temperature,
            stop=["<|user|>", "<|system|>"],
            echo=False,
        )
        text = output["choices"][0]["text"].strip()
        # Print LLM response so it is visible in the console
        _print_llm_response("llamacpp", agent_label, text)
        return text

    return chat


def _openai_chat_factory(agent_label: str) -> Callable[[str, str], str]:
    """
    OpenAI Chat Completions API backend.

    Environment variables:
      OPENAI_API_KEY  – API key (required)
      OPENAI_MODEL    – model (default: gpt-4o)
    """
    try:
        import openai  # type: ignore
    except ImportError as exc:
        raise ImportError(
            "openai package not installed. Run: pip install openai"
        ) from exc

    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise EnvironmentError("OPENAI_API_KEY is not set in the environment.")

    model = os.getenv("OPENAI_MODEL", "gpt-4o")
    client = openai.OpenAI(api_key=api_key)
    logger = get_logger("utils.openai")

    def chat(system_prompt: str, user_message: str) -> str:
        logger.info("  [openai] Calling model=%s …", model)
        response = client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user",   "content": user_message},
            ],
        )
        text = response.choices[0].message.content
        # Print LLM response so it is visible in the console
        _print_llm_response("openai", agent_label, text)
        return text

    return chat


# ─────────────────────────────────────────────────────────────────────────────
# Output helpers
# ─────────────────────────────────────────────────────────────────────────────

def timestamped_filename(prefix: str, extension: str = "md") -> str:
    """Return a filename like prefix_20250101_143022.md."""
    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    return f"{prefix}_{ts}.{extension}"


def ensure_output_dir(path: str | Path) -> Path:
    """Create the output directory if it does not exist and return it."""
    p = Path(path)
    p.mkdir(parents=True, exist_ok=True)
    return p


def write_report(content: str, output_dir: str | Path, filename: str) -> Path:
    """Write *content* to output_dir/filename and return the full path."""
    out_dir = ensure_output_dir(output_dir)
    out_path = out_dir / filename
    out_path.write_text(content, encoding="utf-8")
    return out_path


def pretty_json(data: Any) -> str:
    """Return a pretty-printed JSON string."""
    return json.dumps(data, indent=2, default=str)


def box(title: str, body: str, width: int = 72) -> str:
    """Render a simple ASCII box around a block of text for console output."""
    border = "─" * (width - 2)
    wrapped = textwrap.fill(body, width=width - 4) if body.strip() else ""
    lines = [f"│  {line:<{width - 4}}  │" for line in wrapped.splitlines()] if wrapped else []
    body_block = "\n".join(lines)
    if body_block:
        return (
            f"┌{border}┐\n"
            f"│  {title:<{width - 4}}  │\n"
            f"├{border}┤\n"
            f"{body_block}\n"
            f"└{border}┘"
        )
    return (
        f"┌{border}┐\n"
        f"│  {title:<{width - 4}}  │\n"
        f"└{border}┘"
    )
