#!/usr/bin/env python3
"""
Normalize a payload to one JSON record (if possible) and print a schema table.
Designed to read from stdin (piped from curl, aws, head -1, etc.).
"""
from __future__ import annotations

import argparse
import csv
import json
import sys
from typing import Any


def infer_type(v: Any) -> str:
    if v is None:
        return "null"
    if isinstance(v, bool):
        return "boolean"
    if isinstance(v, int) and not isinstance(v, bool):
        return "integer"
    if isinstance(v, float):
        return "float"
    if isinstance(v, list):
        if not v:
            return "array (empty)"
        return f"array of {infer_type(v[0])}"
    if isinstance(v, dict):
        return "object"
    return "string"


def flatten_schema(d: Any, prefix: str = "") -> list[tuple[str, str, str]]:
    rows: list[tuple[str, str, str]] = []
    if isinstance(d, dict):
        for k, v in d.items():
            full_key = f"{prefix}.{k}" if prefix else f".{k}"
            t = infer_type(v)
            example = (
                str(v)[:60]
                if not isinstance(v, (dict, list))
                else ""
            )
            rows.append((full_key, t, example))
            if isinstance(v, dict):
                rows.extend(flatten_schema(v, full_key))
            elif isinstance(v, list) and v and isinstance(v[0], dict):
                rows.extend(flatten_schema(v[0], full_key + "[]"))
    return rows


def unwrap_wrapped_list(obj: dict[str, Any]) -> tuple[Any, str | None]:
    """If object has exactly one plausible list of dicts, return first element and key name."""
    for k, v in obj.items():
        if isinstance(v, list) and v and isinstance(v[0], dict):
            return v[0], k
    return obj, None


def normalize_to_record(raw: str) -> tuple[Any | None, str | None, str | None]:
    """
    Returns (parsed_object, note, error).
    note explains normalization (e.g. first array element, key .items).
    """
    text = raw.lstrip("\ufeff").strip()
    if not text:
        return None, None, "empty input"

    # Whole buffer JSON
    try:
        d = json.loads(text)
        if isinstance(d, list):
            if not d:
                return None, None, "JSON array is empty"
            if isinstance(d[0], dict):
                return d[0], "used first element of top-level JSON array", None
            return d[0], "used first element of top-level JSON array", None
        if isinstance(d, dict):
            inner, key = unwrap_wrapped_list(d)
            if key is not None and inner is not d:
                return inner, f"used first record from list at key '.{key}'", None
            return d, None, None
    except json.JSONDecodeError:
        pass

    # NDJSON: first line
    first_line = text.splitlines()[0].strip()
    try:
        d = json.loads(first_line)
        if isinstance(d, dict):
            inner, key = unwrap_wrapped_list(d)
            if key is not None and inner is not d:
                return inner, f"used first record from list at key '.{key}' (line 1)", None
            return d, "parsed first line as JSON (NDJSON)", None
        if isinstance(d, list) and d:
            return d[0], "used first element of JSON array on first line", None
        return d, "parsed first line as JSON", None
    except json.JSONDecodeError:
        pass

    return None, None, "not valid JSON (try CSV mode or paste a JSON object)"


def print_report(sample_label: str, obj: Any, note: str | None) -> None:
    print(f"## Source sample — {sample_label}")
    if note:
        print(f"Note: {note}")
    print()
    print("### Raw sample (one record)")
    print(json.dumps(obj, indent=2, ensure_ascii=False))
    print()
    print("### Schema (inferred)")
    print(f"{'Field':<44} {'Type':<22} {'Example'}")
    print("-" * 92)
    for field, typ, ex in flatten_schema(obj):
        print(f"{field:<44} {typ:<22} {ex}")


def csv_first_row_report(path: str) -> int:
    with open(path, newline="", encoding="utf-8", errors="replace") as f:
        reader = csv.DictReader(f)
        row = next(reader, None)
        if row is None:
            print("CSV: no data rows after header", file=sys.stderr)
            return 1
    print("## Source sample — file (CSV)")
    print()
    print("### Columns")
    print(", ".join(reader.fieldnames or []))
    print()
    print("### First row (values as strings)")
    print(json.dumps(dict(row), indent=2, ensure_ascii=False))
    print()
    print("### Schema (inferred from string cells)")
    print(f"{'Field':<44} {'Type':<22} {'Example'}")
    print("-" * 92)
    for k, v in row.items():
        print(f"{'.' + k:<44} {'string':<22} {str(v)[:60]}")
    return 0


def main() -> int:
    p = argparse.ArgumentParser(description="Infer schema from one JSON record on stdin.")
    p.add_argument(
        "--label",
        default="stdin",
        help="Label for the report header (e.g. s3://bucket/key)",
    )
    p.add_argument(
        "--raw-only",
        action="store_true",
        help="Print raw input only (no JSON/schema); for non-JSON previews",
    )
    args = p.parse_args()

    raw = sys.stdin.read()
    if args.raw_only:
        sys.stdout.write(raw)
        if raw and not raw.endswith("\n"):
            sys.stdout.write("\n")
        return 0

    obj, note, err = normalize_to_record(raw)
    if obj is None:
        print(f"Could not normalize JSON record: {err}", file=sys.stderr)
        print("--- raw (first 800 chars) ---", file=sys.stderr)
        print(raw[:800], file=sys.stderr)
        return 1

    print_report(args.label, obj, note)
    return 0


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "csv-file":
        raise SystemExit(csv_first_row_report(sys.argv[2]))
    raise SystemExit(main())
