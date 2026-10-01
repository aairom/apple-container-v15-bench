"""
utils.py
--------
General-purpose helpers used across the demo project.

Covers four concerns:
  - String normalisation (``slugify``, ``truncate``)
  - Simple retry logic for flaky callables (``retry``)
  - Flat key-value configuration loaded from environment or a dict
    (``Config``)

All functions use type annotations and are safe to import in any
Python 3.10+ environment with no third-party packages.
"""

import os
import re
import time
from typing import Any, Callable, TypeVar

_T = TypeVar("_T")


# ---------------------------------------------------------------------------
# String helpers
# ---------------------------------------------------------------------------

def slugify(text: str) -> str:
    """Convert *text* to a URL/filename-safe slug.

    Steps applied:
      1. Lowercase the text.
      2. Replace runs of non-alphanumeric characters with a single hyphen.
      3. Strip leading and trailing hyphens.

    Parameters
    ----------
    text:
        Arbitrary input string.

    Returns
    -------
    str
        Slug suitable for use in file names or URL segments.

    Examples
    --------
    >>> slugify("Hello, World!")
    'hello-world'
    >>> slugify("  multiple   spaces  ")
    'multiple-spaces'
    """
    lowered = text.lower()
    slug = re.sub(r"[^a-z0-9]+", "-", lowered)
    return slug.strip("-")


def truncate(text: str, max_length: int = 80, suffix: str = "…") -> str:
    """Return *text* truncated to *max_length* characters.

    If *text* is already within the limit it is returned unchanged.
    Otherwise the string is cut and *suffix* is appended so the total
    length does not exceed *max_length*.

    Parameters
    ----------
    text:
        Input string.
    max_length:
        Maximum number of characters in the returned string (including
        the suffix).  Must be greater than ``len(suffix)``.
    suffix:
        Appended when truncation occurs.  Defaults to ``'…'``.

    Returns
    -------
    str
        Possibly-truncated string.
    """
    if len(text) <= max_length:
        return text
    cut = max_length - len(suffix)
    return text[:cut] + suffix


def flatten_dict(
    nested: dict[str, Any], sep: str = ".", prefix: str = ""
) -> dict[str, Any]:
    """Recursively flatten a nested dict into a single-level dict.

    Nested keys are joined with *sep*.

    Parameters
    ----------
    nested:
        Dict that may contain further dict values at arbitrary depth.
    sep:
        Separator inserted between parent and child key names.
    prefix:
        Internal accumulator for the current key path; callers should
        leave this at its default empty string.

    Returns
    -------
    dict[str, Any]
        Flat dict where each key encodes the full path to a leaf value.

    Examples
    --------
    >>> flatten_dict({"a": {"b": 1, "c": {"d": 2}}})
    {'a.b': 1, 'a.c.d': 2}
    """
    result: dict[str, Any] = {}
    for key, value in nested.items():
        full_key = f"{prefix}{sep}{key}" if prefix else key
        if isinstance(value, dict):
            result.update(flatten_dict(value, sep=sep, prefix=full_key))
        else:
            result[full_key] = value
    return result


# ---------------------------------------------------------------------------
# Retry helper
# ---------------------------------------------------------------------------

def retry(
    func: Callable[..., _T],
    *args: Any,
    attempts: int = 3,
    delay: float = 1.0,
    exceptions: tuple[type[Exception], ...] = (Exception,),
    **kwargs: Any,
) -> _T:
    """Call *func* with *args* / *kwargs*, retrying on failure.

    Parameters
    ----------
    func:
        Callable to invoke.
    *args:
        Positional arguments forwarded to *func*.
    attempts:
        Maximum number of total attempts (must be ≥ 1).
    delay:
        Seconds to wait between attempts.
    exceptions:
        Tuple of exception types that trigger a retry.  Any other
        exception propagates immediately.
    **kwargs:
        Keyword arguments forwarded to *func*.

    Returns
    -------
    _T
        The return value of *func* on success.

    Raises
    ------
    Exception
        The last exception raised by *func* if all attempts are exhausted.
    """
    last_exc: Exception | None = None
    for attempt in range(1, attempts + 1):
        try:
            return func(*args, **kwargs)
        except exceptions as exc:
            last_exc = exc
            if attempt < attempts:
                time.sleep(delay)
    # All attempts exhausted – re-raise the most recent exception.
    raise last_exc  # type: ignore[misc]


# ---------------------------------------------------------------------------
# Config class
# ---------------------------------------------------------------------------

class Config:
    """Lightweight key-value configuration store.

    Values can be loaded from a plain dict, from the process environment,
    or from both (environment values take precedence).

    Usage
    -----
    ::

        cfg = Config(defaults={"timeout": "30", "retries": "3"})
        cfg.load_env(prefix="APP_")
        timeout = cfg.get_int("timeout", default=10)
    """

    def __init__(self, defaults: dict[str, str] | None = None) -> None:
        """Initialise with optional default values.

        Parameters
        ----------
        defaults:
            Initial key→value mapping.  All values must be strings so
            they remain compatible with environment variable semantics.
        """
        self._store: dict[str, str] = dict(defaults or {})

    def load_env(self, prefix: str = "") -> None:
        """Import environment variables into the store.

        Parameters
        ----------
        prefix:
            Only variables whose names start with *prefix* are imported.
            The prefix is stripped from the key before storing, and the
            key is lower-cased.  Pass ``""`` to import all variables.
        """
        for key, value in os.environ.items():
            if key.startswith(prefix):
                clean_key = key[len(prefix):].lower()
                self._store[clean_key] = value

    def get(self, key: str, default: str = "") -> str:
        """Return the string value for *key*, or *default* if absent.

        Parameters
        ----------
        key:
            Config key (case-sensitive).
        default:
            Fallback value when the key is not present.

        Returns
        -------
        str
        """
        return self._store.get(key, default)

    def get_int(self, key: str, default: int = 0) -> int:
        """Return the integer value for *key*, or *default* if absent or
        not parseable as an integer.

        Parameters
        ----------
        key:
            Config key.
        default:
            Fallback integer.

        Returns
        -------
        int
        """
        raw = self._store.get(key, "")
        try:
            return int(raw)
        except (ValueError, TypeError):
            return default

    def as_dict(self) -> dict[str, str]:
        """Return a shallow copy of the internal store.

        Returns
        -------
        dict[str, str]
        """
        return dict(self._store)
