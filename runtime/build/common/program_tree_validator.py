import json
import os
import pathlib
from dataclasses import dataclass, field
from typing import Dict, List, Optional

CONTRACT_PATH = os.path.join(os.path.dirname(__file__), "..", "contracts", "runtime-program-contract.json")


@dataclass
class ProgramTreeValidationResult:
    valid: bool = False
    errors: List[str] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)
    missing_required: List[str] = field(default_factory=list)
    missing_subdirs: List[str] = field(default_factory=list)
    forbidden_entries: List[str] = field(default_factory=list)


def load_program_contract(contract_path: str = CONTRACT_PATH) -> dict:
    if not os.path.exists(contract_path):
        return {}
    with open(contract_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data if isinstance(data, dict) else {}


def validate_program_tree(
    program_root: str,
    contract_path: str = CONTRACT_PATH,
    strict: bool = True,
) -> ProgramTreeValidationResult:
    result = ProgramTreeValidationResult()
    contract = load_program_contract(contract_path)

    if not contract:
        result.errors.append(f"Program contract not found: {contract_path}")
        return result

    if not os.path.isdir(program_root):
        result.errors.append(f"Program root is not a directory: {program_root}")
        return result

    required_paths = contract.get("requiredProgramPaths", [])
    expected_subdirs = contract.get("programSubdirs", [])

    actual_entries = set()
    for dirpath, dirnames, filenames in os.walk(program_root):
        rel_root = os.path.relpath(dirpath, program_root)
        if rel_root == ".":
            rel_root = ""
        for name in dirnames:
            rel = os.path.join(rel_root, name).replace(os.sep, "/") if rel_root else name
            actual_entries.add(rel + "/")
        for name in filenames:
            rel = os.path.join(rel_root, name).replace(os.sep, "/") if rel_root else name
            actual_entries.add(rel)

    actual_subdirs = set()
    for entry in actual_entries:
        if entry.endswith("/"):
            top = entry.split("/")[0]
            if top:
                actual_subdirs.add(top)

    for req in required_paths:
        req_norm = req.replace("\\", "/").lstrip("/")
        if req_norm not in actual_entries:
            result.missing_required.append(req_norm)

    for sub in expected_subdirs:
        sub_norm = sub.replace("\\", "/").lstrip("/")
        if sub_norm not in actual_subdirs:
            result.missing_subdirs.append(sub_norm)

    if result.missing_required:
        result.errors.append(
            f"Missing required program paths: {result.missing_required}"
        )
    if result.missing_subdirs:
        result.errors.append(
            f"Missing program subdirs: {result.missing_subdirs}"
        )

    if not strict and not result.errors:
        result.valid = True
    elif strict and not result.errors and not result.missing_required and not result.missing_subdirs:
        result.valid = True

    return result
