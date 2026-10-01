"""
tests/test_subagents.py – Unit tests for the Sub-Agents Demo

Covers:
  • SubAgentTask / SubAgentResult / OrchestratorReport data models
  • CodeExplorerAgent lifecycle (register → assign → execute)
  • DocumentationWriterAgent lifecycle
  • utils.get_llm_client: backend selection and LLM_BACKEND env var routing
  • llama.cpp backend (llama-cpp-python) – fully mocked, no model file needed
  • Ollama backend – mocked HTTP response
  • Console LLM output (_print_llm_response)
  • Streamlit UI import smoke-check

All tests mock external I/O so they run with no Ollama, no API key, and no
GGUF model file.
"""

from __future__ import annotations

import importlib
import os
import sys
from io import StringIO
from pathlib import Path
from unittest.mock import MagicMock, patch, PropertyMock

# ── ensure src/ is on the path ────────────────────────────────────────────────
SRC_DIR = Path(__file__).parent.parent / "src"
sys.path.insert(0, str(SRC_DIR))
INPUT_DIR = Path(__file__).parent.parent / "input"

import pytest

from models import OrchestratorReport, SubAgentResult, SubAgentTask
from code_explorer_agent import CodeExplorerAgent
from documentation_writer_agent import DocumentationWriterAgent

MOCK_LLM_RESPONSE = "This is a mock LLM summary for testing purposes."


def make_mock_llm():
    """Return a mock chat callable that returns a canned response."""
    return MagicMock(return_value=MOCK_LLM_RESPONSE)


# ─────────────────────────────────────────────────────────────────────────────
# Model tests
# ─────────────────────────────────────────────────────────────────────────────

class TestSubAgentTask:
    def test_defaults(self):
        task = SubAgentTask(
            agent_id="test-agent",
            task_type="explore",
            description="Do something",
        )
        assert task.fork_context is False
        assert task.payload == {}

    def test_custom_payload(self):
        task = SubAgentTask(
            agent_id="test",
            task_type="general",
            description="Write docs",
            fork_context=True,
            payload={"key": "value"},
        )
        assert task.fork_context is True
        assert task.payload["key"] == "value"


class TestSubAgentResult:
    def test_success_result(self):
        result = SubAgentResult(
            agent_id="explorer",
            success=True,
            summary="Found 3 files",
            data={"files_found": 3},
        )
        assert result.success is True
        assert result.error is None

    def test_failure_result(self):
        result = SubAgentResult(
            agent_id="explorer",
            success=False,
            summary="",
            error="FileNotFoundError: no such directory",
        )
        assert result.success is False
        assert "FileNotFoundError" in result.error


class TestOrchestratorReport:
    def test_all_succeeded_true(self):
        import datetime as dt
        report = OrchestratorReport(
            generated_at=dt.datetime.now(),
            target_directory="/tmp",
            results=[
                SubAgentResult("a1", True, "ok"),
                SubAgentResult("a2", True, "ok"),
            ],
        )
        assert report.all_succeeded is True

    def test_all_succeeded_false(self):
        import datetime as dt
        report = OrchestratorReport(
            generated_at=dt.datetime.now(),
            target_directory="/tmp",
            results=[
                SubAgentResult("a1", True,  "ok"),
                SubAgentResult("a2", False, "", error="boom"),
            ],
        )
        assert report.all_succeeded is False
        assert "a2" in report.failed_agents


# ─────────────────────────────────────────────────────────────────────────────
# CodeExplorerAgent tests
# ─────────────────────────────────────────────────────────────────────────────

class TestCodeExplorerAgent:
    def _make_agent(self):
        agent = CodeExplorerAgent()
        agent._llm = make_mock_llm()
        return agent

    def test_registration(self):
        agent = self._make_agent()
        assert agent.agent_id == "code-explorer"
        assert agent.agent_type == "explore"

    def test_task_assignment_wrong_id_raises(self):
        agent = self._make_agent()
        bad_task = SubAgentTask(
            agent_id="wrong-id",
            task_type="explore",
            description="nope",
        )
        with pytest.raises(ValueError, match="wrong-id"):
            agent.assign_task(bad_task)

    def test_execute_without_task_returns_failure(self):
        agent = self._make_agent()
        result = agent.execute()
        assert result.success is False
        assert "No task assigned" in result.error

    def test_execute_with_real_input_dir(self):
        agent = self._make_agent()
        task = SubAgentTask(
            agent_id="code-explorer",
            task_type="explore",
            description="Explore the sample input codebase.",
            payload={"target_directory": str(INPUT_DIR)},
        )
        agent.assign_task(task)
        result = agent.execute()

        assert result.success is True
        assert result.data["files_found"] >= 3  # inventory, order_processor, reporting
        assert result.summary == MOCK_LLM_RESPONSE
        assert "ast_parser" in result.tools_used

    def test_execute_empty_directory(self, tmp_path):
        agent = self._make_agent()
        task = SubAgentTask(
            agent_id="code-explorer",
            task_type="explore",
            description="Explore empty dir",
            payload={"target_directory": str(tmp_path)},
        )
        agent.assign_task(task)
        result = agent.execute()

        assert result.success is True
        assert result.data["files_found"] == 0


# ─────────────────────────────────────────────────────────────────────────────
# DocumentationWriterAgent tests
# ─────────────────────────────────────────────────────────────────────────────

class TestDocumentationWriterAgent:
    SAMPLE_MODULES = [
        {
            "filename": "foo.py",
            "lines": 50,
            "module_docstring": "A foo module.",
            "classes": [{"name": "Foo", "methods": ["bar", "baz"], "docstring": ""}],
            "functions": [{"name": "helper", "args": ["x"], "docstring": "Help."}],
            "imports": ["os", "sys"],
        }
    ]

    def _make_agent(self):
        agent = DocumentationWriterAgent()
        agent._llm = make_mock_llm()
        return agent

    def test_registration(self):
        agent = self._make_agent()
        assert agent.agent_id == "documentation-writer"
        assert agent.agent_type == "general"

    def test_task_assignment_wrong_id_raises(self):
        agent = self._make_agent()
        bad_task = SubAgentTask(
            agent_id="wrong-id",
            task_type="general",
            description="nope",
        )
        with pytest.raises(ValueError, match="wrong-id"):
            agent.assign_task(bad_task)

    def test_execute_without_task_returns_failure(self):
        agent = self._make_agent()
        result = agent.execute()
        assert result.success is False
        assert "No task assigned" in result.error

    def test_execute_produces_markdown(self):
        agent = self._make_agent()
        task = SubAgentTask(
            agent_id="documentation-writer",
            task_type="general",
            description="Write docs.",
            fork_context=True,
            payload={
                "project_name": "Test Project",
                "modules": self.SAMPLE_MODULES,
                "explorer_summary": "A simple test project.",
            },
        )
        agent.assign_task(task)
        result = agent.execute()

        assert result.success is True
        assert "modules_documented" in result.data
        assert result.data["modules_documented"] == 1
        markdown = result.data["markdown"]
        assert "# Test Project" in markdown
        assert "Project Overview" in markdown
        assert "Module Reference" in markdown

    def test_execute_empty_modules(self):
        agent = self._make_agent()
        task = SubAgentTask(
            agent_id="documentation-writer",
            task_type="general",
            description="Write docs.",
            fork_context=True,
            payload={
                "project_name": "Empty Project",
                "modules": [],
                "explorer_summary": "Nothing here.",
            },
        )
        agent.assign_task(task)
        result = agent.execute()

        assert result.success is True
        assert result.data["modules_documented"] == 0


# ─────────────────────────────────────────────────────────────────────────────
# utils.get_llm_client – backend selection tests
# ─────────────────────────────────────────────────────────────────────────────

class TestGetLlmClientBackendSelection:
    """Verify that LLM_BACKEND (and legacy LLM_PROVIDER) route to correct factory."""

    def _import_utils(self):
        """Re-import utils fresh to pick up env-var changes."""
        import utils as u
        return u

    def test_default_is_ollama(self, monkeypatch):
        monkeypatch.delenv("LLM_BACKEND", raising=False)
        monkeypatch.delenv("LLM_PROVIDER", raising=False)
        u = self._import_utils()
        with patch.object(u, "_ollama_chat_factory", return_value=MagicMock()) as mock_f:
            u.get_llm_client("test-agent")
            mock_f.assert_called_once_with("test-agent")

    def test_llm_backend_ollama(self, monkeypatch):
        monkeypatch.setenv("LLM_BACKEND", "ollama")
        monkeypatch.delenv("LLM_PROVIDER", raising=False)
        u = self._import_utils()
        with patch.object(u, "_ollama_chat_factory", return_value=MagicMock()) as mock_f:
            u.get_llm_client("ag")
            mock_f.assert_called_once_with("ag")

    def test_llm_backend_llamacpp(self, monkeypatch):
        monkeypatch.setenv("LLM_BACKEND", "llamacpp")
        u = self._import_utils()
        with patch.object(u, "_llamacpp_chat_factory", return_value=MagicMock()) as mock_f:
            u.get_llm_client("ag")
            mock_f.assert_called_once_with("ag")

    def test_llm_backend_openai(self, monkeypatch):
        monkeypatch.setenv("LLM_BACKEND", "openai")
        u = self._import_utils()
        with patch.object(u, "_openai_chat_factory", return_value=MagicMock()) as mock_f:
            u.get_llm_client("ag")
            mock_f.assert_called_once_with("ag")

    def test_legacy_llm_provider_variable(self, monkeypatch):
        """LLM_PROVIDER (legacy) is still respected when LLM_BACKEND is unset."""
        monkeypatch.delenv("LLM_BACKEND", raising=False)
        monkeypatch.setenv("LLM_PROVIDER", "openai")
        u = self._import_utils()
        with patch.object(u, "_openai_chat_factory", return_value=MagicMock()) as mock_f:
            u.get_llm_client("ag")
            mock_f.assert_called_once_with("ag")

    def test_llamacpp_alias_variants(self, monkeypatch):
        """Both 'llama_cpp' and 'llama-cpp' should route to the llama.cpp factory."""
        u = self._import_utils()
        for alias in ("llama_cpp", "llama-cpp", "llamacpp"):
            monkeypatch.setenv("LLM_BACKEND", alias)
            with patch.object(u, "_llamacpp_chat_factory", return_value=MagicMock()) as mock_f:
                u.get_llm_client("ag")
                mock_f.assert_called_once_with("ag")


# ─────────────────────────────────────────────────────────────────────────────
# llama.cpp backend – fully mocked (no model file needed)
# ─────────────────────────────────────────────────────────────────────────────

class TestLlamaCppBackend:
    """Test the llamacpp factory with llama_cpp.Llama fully mocked."""

    def _make_mock_llama(self, output_text: str = "mocked llamacpp response"):
        """Return a mock Llama class whose instances return a canned completion."""
        mock_instance = MagicMock()
        mock_instance.return_value = {
            "choices": [{"text": output_text}]
        }
        mock_class = MagicMock(return_value=mock_instance)
        return mock_class, mock_instance

    def test_llamacpp_factory_produces_callable(self, monkeypatch, tmp_path):
        """The factory must return a callable even when Llama is mocked."""
        # Create a fake model file so path-existence check passes
        fake_model = tmp_path / "model.gguf"
        fake_model.write_bytes(b"\x00" * 16)

        monkeypatch.setenv("LLAMACPP_MODEL_PATH", str(fake_model))
        monkeypatch.setenv("LLAMACPP_N_CTX",       "512")
        monkeypatch.setenv("LLAMACPP_N_GPU_LAYERS", "0")
        monkeypatch.setenv("LLAMACPP_MAX_TOKENS",  "64")

        mock_class, mock_instance = self._make_mock_llama("hello from llamacpp")

        import utils as u
        with patch.dict("sys.modules", {"llama_cpp": MagicMock(Llama=mock_class)}):
            # Re-run the factory with the mocked module in place
            with patch.object(u, "_print_llm_response") as mock_print:
                factory = u._llamacpp_chat_factory("test-agent")
                result = factory("sys prompt", "user msg")

        assert result == "hello from llamacpp"
        mock_print.assert_called_once_with("llamacpp", "test-agent", "hello from llamacpp")

    def test_llamacpp_missing_model_path_raises(self, monkeypatch):
        """Factory must raise EnvironmentError when LLAMACPP_MODEL_PATH is empty."""
        monkeypatch.setenv("LLAMACPP_MODEL_PATH", "")
        mock_llama_module = MagicMock()

        import utils as u
        with patch.dict("sys.modules", {"llama_cpp": mock_llama_module}):
            with pytest.raises(EnvironmentError, match="LLAMACPP_MODEL_PATH"):
                u._llamacpp_chat_factory("ag")

    def test_llamacpp_nonexistent_model_file_raises(self, monkeypatch):
        """Factory must raise FileNotFoundError when path does not exist."""
        monkeypatch.setenv("LLAMACPP_MODEL_PATH", "/nonexistent/model.gguf")
        mock_llama_module = MagicMock()

        import utils as u
        with patch.dict("sys.modules", {"llama_cpp": mock_llama_module}):
            with pytest.raises(FileNotFoundError, match="/nonexistent/model.gguf"):
                u._llamacpp_chat_factory("ag")

    def test_llamacpp_import_error_raised_clearly(self, monkeypatch, tmp_path):
        """If llama_cpp is not importable, raise ImportError with install hint."""
        fake_model = tmp_path / "m.gguf"
        fake_model.write_bytes(b"\x00")
        monkeypatch.setenv("LLAMACPP_MODEL_PATH", str(fake_model))

        import utils as u
        # Simulate llama_cpp not being installed
        with patch.dict("sys.modules", {"llama_cpp": None}):
            with pytest.raises((ImportError, TypeError)):
                u._llamacpp_chat_factory("ag")


# ─────────────────────────────────────────────────────────────────────────────
# Ollama backend – mocked HTTP
# ─────────────────────────────────────────────────────────────────────────────

class TestOllamaBackend:
    def test_ollama_returns_response_and_prints(self, monkeypatch):
        monkeypatch.setenv("OLLAMA_MODEL", "llama3")
        monkeypatch.setenv("OLLAMA_BASE_URL", "http://localhost:11434")

        mock_response = MagicMock()
        mock_response.json.return_value = {
            "message": {"content": "ollama test response"}
        }

        import utils as u
        with patch("requests.post", return_value=mock_response) as mock_post, \
             patch.object(u, "_print_llm_response") as mock_print:
            factory = u._ollama_chat_factory("explorer")
            result = factory("system", "user")

        assert result == "ollama test response"
        mock_print.assert_called_once_with("ollama", "explorer", "ollama test response")
        mock_post.assert_called_once()


# ─────────────────────────────────────────────────────────────────────────────
# Console output – _print_llm_response
# ─────────────────────────────────────────────────────────────────────────────

class TestPrintLlmResponse:
    def test_output_contains_backend_and_label(self, capsys):
        import utils as u
        u._print_llm_response("ollama", "code-explorer", "Hello world response.")
        captured = capsys.readouterr()
        assert "ollama" in captured.out
        assert "code-explorer" in captured.out
        assert "Hello world response." in captured.out

    def test_output_contains_llamacpp_label(self, capsys):
        import utils as u
        u._print_llm_response("llamacpp", "documentation-writer", "Docs text here.")
        captured = capsys.readouterr()
        assert "llamacpp" in captured.out
        assert "documentation-writer" in captured.out


# ─────────────────────────────────────────────────────────────────────────────
# Streamlit UI – import smoke test
# ─────────────────────────────────────────────────────────────────────────────

class TestUIImport:
    def test_ui_module_can_be_parsed(self):
        """
        Verify that ui.py is syntactically valid Python and can be read.
        This is a static check that does not start Streamlit.
        """
        import ast as ast_mod
        ui_path = SRC_DIR / "ui.py"
        assert ui_path.exists(), "src/ui.py must exist"
        source = ui_path.read_text(encoding="utf-8")
        # Will raise SyntaxError if the file has syntax problems
        tree = ast_mod.parse(source)
        assert tree is not None

    def test_ui_imports_are_available(self):
        """
        Verify that all modules imported by ui.py (except streamlit itself)
        are importable from the src/ directory.
        """
        # These are our own modules that ui.py imports
        for mod_name in ("models", "orchestrator", "utils"):
            assert importlib.util.find_spec(mod_name) is not None, \
                f"Module '{mod_name}' not found on sys.path"
