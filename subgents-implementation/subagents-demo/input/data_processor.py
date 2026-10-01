"""
data_processor.py
-----------------
Provides classes and helpers for loading, validating, and transforming
tabular data records.  Records are represented as plain Python dicts so
the module remains self-contained with no third-party dependencies.

Typical usage
-------------
    from data_processor import CsvLoader, RecordTransformer, summarise

    loader = CsvLoader("sales.csv", delimiter=",")
    records = loader.load()
    clean   = loader.filter_empty(records)

    transformer = RecordTransformer(records=clean, id_field="sale_id")
    normalised  = transformer.normalise_keys()
    print(summarise(normalised))
"""

import csv
import datetime
from typing import Any


# ---------------------------------------------------------------------------
# Loader
# ---------------------------------------------------------------------------

class CsvLoader:
    """Load and pre-process records from a CSV file.

    The loader reads the file lazily row-by-row and converts every value
    to a Python string.  Callers are responsible for type-casting fields
    after loading.
    """

    def __init__(self, filepath: str, delimiter: str = ",") -> None:
        """Initialise the loader with a file path and optional delimiter.

        Parameters
        ----------
        filepath:
            Absolute or relative path to the CSV file.
        delimiter:
            Column separator character (default ``','``).
        """
        self.filepath = filepath
        self.delimiter = delimiter
        self._loaded: list[dict[str, str]] = []

    def load(self) -> list[dict[str, str]]:
        """Read the CSV file and return all rows as a list of dicts.

        Returns
        -------
        list[dict[str, str]]
            One dict per data row; keys are taken from the header row.

        Raises
        ------
        FileNotFoundError
            If ``self.filepath`` does not exist.
        """
        rows: list[dict[str, str]] = []
        with open(self.filepath, newline="", encoding="utf-8") as fh:
            reader = csv.DictReader(fh, delimiter=self.delimiter)
            for row in reader:
                rows.append(dict(row))
        self._loaded = rows
        return rows

    def filter_empty(
        self, records: list[dict[str, str]], required_field: str = ""
    ) -> list[dict[str, str]]:
        """Remove records where *required_field* is blank or missing.

        If *required_field* is the empty string every record is returned
        unchanged (useful for a no-op pass-through).

        Parameters
        ----------
        records:
            Sequence of row dicts produced by :meth:`load`.
        required_field:
            Name of the field that must be non-empty.  Pass ``""`` to
            skip filtering.

        Returns
        -------
        list[dict[str, str]]
            Filtered subset of *records*.
        """
        if not required_field:
            return records
        return [r for r in records if r.get(required_field, "").strip()]


# ---------------------------------------------------------------------------
# Transformer
# ---------------------------------------------------------------------------

class RecordTransformer:
    """Apply in-place transformations to a list of record dicts.

    All transformation methods return *self* so they can be chained::

        transformer.normalise_keys().add_timestamp()
    """

    def __init__(
        self, records: list[dict[str, Any]], id_field: str = "id"
    ) -> None:
        """Initialise the transformer.

        Parameters
        ----------
        records:
            Mutable list of record dicts to transform.
        id_field:
            Name of the field treated as the unique row identifier.
        """
        self.records = records
        self.id_field = id_field

    def normalise_keys(self) -> "RecordTransformer":
        """Strip whitespace and lowercase every key in every record.

        Returns
        -------
        RecordTransformer
            *self*, for method chaining.
        """
        self.records = [
            {k.strip().lower(): v for k, v in record.items()}
            for record in self.records
        ]
        return self

    def add_timestamp(
        self, field_name: str = "processed_at"
    ) -> "RecordTransformer":
        """Inject a UTC timestamp string into every record.

        Parameters
        ----------
        field_name:
            Key under which the timestamp is stored.

        Returns
        -------
        RecordTransformer
            *self*, for method chaining.
        """
        ts = datetime.datetime.utcnow().isoformat(timespec="seconds") + "Z"
        for record in self.records:
            record[field_name] = ts
        return self

    def drop_field(self, field_name: str) -> "RecordTransformer":
        """Remove *field_name* from every record, if present.

        Parameters
        ----------
        field_name:
            Key to remove.

        Returns
        -------
        RecordTransformer
            *self*, for method chaining.
        """
        for record in self.records:
            record.pop(field_name, None)
        return self


# ---------------------------------------------------------------------------
# Standalone helpers
# ---------------------------------------------------------------------------

def summarise(records: list[dict[str, Any]]) -> dict[str, Any]:
    """Return a lightweight summary of a record collection.

    The summary includes the total record count, the set of field names
    found across *all* records, and the number of records that contain
    at least one empty-string value.

    Parameters
    ----------
    records:
        List of record dicts to summarise.

    Returns
    -------
    dict[str, Any]
        A dict with keys ``total``, ``fields``, and ``records_with_empty``.
    """
    if not records:
        return {"total": 0, "fields": [], "records_with_empty": 0}

    all_fields: set[str] = set()
    empty_count = 0
    for record in records:
        all_fields.update(record.keys())
        if any(str(v).strip() == "" for v in record.values()):
            empty_count += 1

    return {
        "total": len(records),
        "fields": sorted(all_fields),
        "records_with_empty": empty_count,
    }
