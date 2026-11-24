#!/usr/bin/env python3
"""Collaboration gates G-1/G-2/G-3 (ADR 0006 + ADR 0008 P0a).

G-3 reads the reviewed spec/baseline/ledger/test refs from the head commit
(``git ls-tree``/``git cat-file --batch``), never the working tree, and applies
the ADR-0008 §6.7 governance evidence model to AC-69+ only.

Read-only: inspects git history, Spec versions and test references; never mutates
the repository. Bash wrapper is scripts/check-collab-gates.sh.
"""
from __future__ import annotations

import argparse
import datetime
import hashlib
import os
import re
import subprocess
import sys
from pathlib import Path

MANAGED = {
    "docs/specs/": "Product Owner",
    "docs/adr/": "System Architect",
    "docs/implementation/": "Developer",
    "docs/po/": "Product Owner",
    "docs/architect/": "System Architect",
    # v1.2.0 governance: the constitution and its execution细则 are
    # Architect-owned; AGENTS.md is the top-level execution charter.
    ".specify/memory/": "System Architect",
    "AGENTS.md": "System Architect",
    "docs/collab/CONVENTIONS.md": "System Architect",
}
ROLE_ALIASES = {
    "po": "Product Owner", "product owner": "Product Owner", "productowner": "Product Owner",
    "architect": "System Architect", "system architect": "System Architect", "systemarchitect": "System Architect",
    "developer": "Developer", "dev": "Developer",
    "reviewer": "Reviewer",
}
AC_RE = re.compile(r"\bAC-(\d{1,3})\b")
TEST_NAME_RE = r"TestAC0*{n}\b"
TEST_COMMENT_RE = r"AC-0*{n}\b"
VERSION_RE = re.compile(r"^\s*version:\s*([0-9]+\.[0-9]+)\s*$", re.M)

# ADR-0008 §6.5/§6.7: business AC-01…68 keep AC-REF/deferral semantics; the
# governance evidence model applies to AC-69…112 only.
GOVERNANCE_AC_MIN = 69
GOVERNANCE_STATUSES = {"REF", "PENDING", "VERIFIED", "DEFERRED"}
GOVERNANCE_TEST_TYPES = {
    "unit", "integration", "race", "fault-injection", "manual", "static", "script",
}
# ADR-0008 §12.2: GV-S20 audit rows, fixed 15-column TSV.
AUDIT_COLUMNS = (
    "timestamp", "change_id", "parent_change_id", "effective_tier", "tier_source",
    "actor", "base", "head", "mode", "verdict", "spec_version", "adr_revision",
    "evidence", "digest", "supersedes",
)
AUDIT_HEADING_RE = re.compile(r"^##\s+Governance audit log\s*$", re.I)
AUDIT_BASE_RE = re.compile(r"^[0-9a-f]{7,40}$")
AUDIT_DIGEST_RE = re.compile(r"^sha1:([0-9a-f]{40})$")
BASELINE_PATH = "docs/collab/ac-baseline.txt"
LEDGER_PATH = "docs/collab/ac-coverage.tsv"
AUDIT_PATH = "docs/review-tracking.md"


def run(repo: Path, args: list[str], check: bool = False) -> str:
    proc = subprocess.run(["git", "-C", str(repo), *args], text=True, capture_output=True)
    if check and proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or f"git {' '.join(args)} failed")
    return proc.stdout


def normalize_role(value: str) -> str:
    return ROLE_ALIASES.get(value.strip().lower(), value.strip())


def changed_files(repo: Path, args: argparse.Namespace, base: str, head: str) -> list[str]:
    if args.staged:
        out = run(repo, ["diff", "--cached", "--name-only"])
        return [line.strip() for line in out.splitlines() if line.strip()]
    out = run(repo, ["diff-tree", "-r", "--no-commit-id", "--name-only", base, head])
    return [line.strip() for line in out.splitlines() if line.strip()]


def split_range(repo: Path, value: str | None) -> tuple[str, str]:
    if value:
        if ".." in value:
            base, head = value.split("..", 1)
            return base or "HEAD^", head or "HEAD"
        return value, "HEAD"
    # default: last commit; if there is no parent use the root commit
    parent = run(repo, ["rev-parse", "--verify", "HEAD^"]).strip()
    return ("HEAD^", "HEAD") if parent else ("HEAD", "HEAD")


def commits(repo: Path, base: str, head: str) -> list[tuple[str, str]]:
    fmt = "%H%x00%B%x00"
    out = run(repo, ["log", "--format=" + fmt, f"{base}..{head}"])
    parts = out.split("\x00")
    result = []
    for i in range(0, len(parts) - 1, 2):
        sha = parts[i].strip()
        body = parts[i + 1]
        if sha:
            result.append((sha, body))
    return result


def parse_trailers(body: str) -> tuple[set[str], list[str], bool]:
    roles: set[str] = set()
    exceptions: list[str] = []
    non_normative = False
    for line in body.splitlines():
        key, _, value = line.partition(":")
        if not _:
            continue
        key = key.strip().lower()
        value = value.strip()
        if key == "role":
            for role in value.split(","):
                if role.strip():
                    roles.add(normalize_role(role))
        elif key == "exception":
            exceptions.extend(re.findall(r"T-\d{3}", value))
        elif key == "non-normative" and value.lower() == "true":
            non_normative = True
    return roles, exceptions, non_normative


def trailer_value(body: str, key: str) -> str:
    for line in body.splitlines():
        k, sep, v = line.partition(":")
        if sep and k.strip().lower() == key.lower():
            return v.strip()
    return ""


def thread_status(repo: Path, thread: str) -> str | None:
    matches = list(repo.glob(f"docs/collab/*/threads/{thread}.md"))
    if not matches:
        return None
    text = matches[0].read_text(errors="replace")
    match = re.search(r"^status:\s*(\S+)\s*$", text, re.M)
    return match.group(1) if match else "UNKNOWN"


def check_g1(
    repo: Path, args: argparse.Namespace, files: list[str],
    head: str, range_commits: list[tuple[str, str]],
) -> list[str]:
    failures: list[str] = []
    warnings: list[str] = []
    managed = [f for f in files if any(f.startswith(prefix) for prefix in MANAGED)]
    if not managed:
        return failures
    if not args.staged:
        body = run(repo, ["log", "-1", "--format=%B", head])
        change_id = trailer_value(body, "change_id")
        actor = trailer_value(body, "Actor")
        reviewed_by = trailer_value(body, "Reviewed-by")
        if not change_id or not actor or not reviewed_by:
            failures.append("G-1 FAIL HEAD missing change_id/Actor/Reviewed-by trailer")
        elif actor.strip().lower() == reviewed_by.strip().lower():
            failures.append("G-1 FAIL Actor and Reviewed-by must differ")
    if args.staged:
        roles = {normalize_role(r) for r in (args.role or "").split(",") if r.strip()}
        exceptions: list[str] = []
    else:
        roles = set()
        exceptions = []
        for _sha, body in range_commits:
            commit_roles, commit_exceptions, _ = parse_trailers(body)
            roles |= commit_roles
            exceptions.extend(commit_exceptions)
            if not commit_roles:
                warnings.append("G-1 WARN missing Role")
        for thread in exceptions:
            status = thread_status(repo, thread)
            if status is None:
                failures.append(f"G-1 FAIL Exception {thread} thread not found")
                exceptions.remove(thread)
            elif status.upper() == "CLOSED":
                failures.append(f"G-1 FAIL Exception {thread} thread is CLOSED")
                exceptions.remove(thread)
    for path in managed:
        owner = next((MANAGED[p] for p in MANAGED if path.startswith(p)), None)
        if owner is None:
            continue
        if args.staged and not roles:
            failures.append(f"G-1 FAIL {path} missing --role")
            continue
        if owner not in roles:
            if exceptions:
                for thread in exceptions:
                    print(f"G-1 EXEMPT {thread} {path}")
                continue
            failures.append(f"G-1 FAIL {path} role={','.join(sorted(roles)) or 'missing'} expected={owner}")
    if warnings and not failures:
        for warning in warnings:
            print(warning)
        failures.append("G-1 FAIL missing Role trailer")
    return failures


def yaml_version(text: str) -> str | None:
    match = VERSION_RE.search(text)
    return match.group(1) if match else None


def git_show(repo: Path, ref: str, path: str) -> str | None:
    proc = subprocess.run(["git", "-C", str(repo), "show", f"{ref}:{path}"], text=True, capture_output=True)
    return proc.stdout if proc.returncode == 0 else None


def readme_index_version(text: str, spec_path: str) -> str | None:
    name = Path(spec_path).name
    for line in text.splitlines():
        if name in line:
            cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
            for cell in reversed(cells):
                if re.fullmatch(r"\d+\.\d+", cell):
                    return cell
    return None


def check_g2(
    repo: Path, args: argparse.Namespace, files: list[str], non_normative: bool,
    base: str, head: str,
) -> list[str]:
    failures: list[str] = []
    if args.staged:
        return failures
    for path in files:
        if not path.startswith("docs/specs/") or not path.endswith(".md") or path == "docs/specs/README.md":
            continue
        new_text = git_show(repo, head, path)
        if new_text is None:
            new_text = (repo / path).read_text(errors="replace")
        new_version = yaml_version(new_text)
        if not new_version:
            failures.append(f"G-2 FAIL {path} missing YAML version")
            continue
        old_text = git_show(repo, base, path)
        if old_text is None:
            # New spec must be indexed and record its version.
            readme = git_show(repo, head, "docs/specs/README.md") or ""
            if Path(path).name not in readme:
                failures.append(f"G-2 FAIL {path} not found in docs/specs/README.md")
            if new_version not in new_text.split("## 13. 变更记录", 1)[-1]:
                failures.append(f"G-2 FAIL {path} changelog missing {new_version}")
            continue
        old_version = yaml_version(old_text)
        if old_version == new_version:
            if non_normative:
                print(f"G-2 EXEMPT non-normative {path}")
            else:
                failures.append(f"G-2 FAIL {path} version {old_version} -> {new_version} (no bump)")
            continue
        try:
            if tuple(map(int, new_version.split("."))) <= tuple(map(int, (old_version or "0.0").split("."))):
                failures.append(f"G-2 FAIL {path} version {old_version} -> {new_version} (not greater)")
        except ValueError:
            failures.append(f"G-2 FAIL {path} invalid version {new_version}")
        readme = git_show(repo, head, "docs/specs/README.md") or (repo / "docs/specs/README.md").read_text(errors="replace")
        index_version = readme_index_version(readme, path)
        if index_version != new_version:
            failures.append(f"G-2 FAIL docs/specs/README.md index={index_version} yaml={new_version}")
        if new_version not in new_text.split("## 13. 变更记录", 1)[-1]:
            failures.append(f"G-2 FAIL {path} changelog missing {new_version}")
    return failures


def parse_acs(text: str) -> set[int]:
    return {int(m.group(1)) for m in AC_RE.finditer(text)}


def parse_baseline(text: str) -> list[int]:
    values = []
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        match = re.search(r"AC-(\d{1,3})", line)
        if match:
            values.append(int(match.group(1)))
    return values


def git_bytes(repo: Path, args: list[str]) -> bytes | None:
    proc = subprocess.run(["git", "-C", str(repo), *args], capture_output=True)
    return proc.stdout if proc.returncode == 0 else None


def git_batch_blobs(repo: Path, specs: list[str]) -> list[bytes | None]:
    """Read many ``<ref>:<path>`` blobs in one git process (head-bound reads)."""
    if not specs:
        return []
    payload = "".join(f"{spec}\n" for spec in specs).encode()
    proc = subprocess.run(
        ["git", "-C", str(repo), "cat-file", "--batch"],
        input=payload, capture_output=True,
    )
    if proc.returncode != 0:
        return [None] * len(specs)
    data = proc.stdout
    result: list[bytes | None] = []
    pos = 0
    for _ in specs:
        newline = data.find(b"\n", pos)
        if newline < 0:
            result.append(None)
            continue
        header = data[pos:newline].decode(errors="replace")
        pos = newline + 1
        parts = header.split()
        if len(parts) != 3 or parts[1] != "blob":
            result.append(None)
            continue
        size = int(parts[2])
        result.append(data[pos:pos + size])
        pos += size + 1
    if len(result) < len(specs):
        result.extend([None] * (len(specs) - len(result)))
    return result


def tracked_files(repo: Path, ref: str) -> list[str] | None:
    """Tracked paths at ``ref`` (ADR-0008 §6.7 item 9). None ⇒ unreadable head."""
    out = git_bytes(repo, ["ls-tree", "-r", "--name-only", ref])
    if out is None:
        return None
    return [line for line in out.decode(errors="replace").splitlines() if line.strip()]


def blob_map(repo: Path, ref: str, paths: list[str]) -> dict[str, bytes | None]:
    unique = list(dict.fromkeys(paths))
    values = git_batch_blobs(repo, [f"{ref}:{path}" for path in unique])
    return dict(zip(unique, values))


def head_test_files(tracked: list[str]) -> list[str]:
    files = []
    for path in tracked:
        if not path.endswith("_test.go"):
            continue
        if "/.tmp/" in f"/{path}/" or "/node_modules/" in f"/{path}/" or path.startswith("vendor/"):
            continue
        files.append(path)
    return files


def coverage_ledger(text: str | None) -> tuple[dict[int, dict[str, str]], list[str]]:
    """Parse ac-coverage.tsv text read from the reviewed head."""
    ledger: dict[int, dict[str, str]] = {}
    errors: list[str] = []
    if text is None:
        return ledger, errors
    for line in text.splitlines():
        if not line.strip() or line.startswith("#"):
            continue
        cells = line.split("\t")
        if cells[0].lower() == "ac":
            continue
        match = re.fullmatch(r"AC-(\d{1,3})", cells[0].strip())
        if not match or len(cells) < 5:
            errors.append(f"G-3 WARN malformed coverage row: {line}")
            continue
        ac = int(match.group(1))
        ledger[ac] = {
            "owner": cells[1].strip(), "expiry": cells[2].strip(),
            "reason": cells[3].strip(), "thread": cells[4].strip(),
            "test_type": cells[5].strip() if len(cells) > 5 else "",
            "evidence_path": cells[6].strip() if len(cells) > 6 else "",
            "status": cells[7].strip() if len(cells) > 7 else "",
        }
    return ledger, errors


def parse_evidence_keys(text: str) -> dict[str, str]:
    values: dict[str, str] = {}
    for part in text.split(";"):
        key, sep, value = part.partition("=")
        if sep:
            values[key.strip().lower()] = value.strip()
    return values


def governance_audit_rows(text: str | None) -> tuple[list[dict[str, str]], list[str]]:
    """Parse the GV-S20 ``## Governance audit log`` TSV at the reviewed head."""
    if text is None:
        return [], []
    rows: list[dict[str, str]] = []
    warnings: list[str] = []
    in_section = False
    for line in text.splitlines():
        if line.startswith("## "):
            in_section = bool(AUDIT_HEADING_RE.match(line.strip()))
            continue
        if not in_section or "\t" not in line:
            continue
        cells = [cell.strip() for cell in line.split("\t")]
        if len(cells) < len(AUDIT_COLUMNS):
            warnings.append("G-3 WARN malformed Governance audit log row")
            continue
        if cells[0].lower() == "timestamp":
            continue
        rows.append(dict(zip(AUDIT_COLUMNS, cells[:len(AUDIT_COLUMNS)])))
    return rows, warnings


def governance_evidence_problem(
    ac: int, row: dict[str, str], audit_rows: list[dict[str, str]],
    evidence_blobs: dict[str, bytes],
) -> tuple[str | None, str]:
    """Return (problem, evidence_sha1). problem is None only for AC-VERIFIED."""
    status = row.get("status", "").strip()
    if status not in GOVERNANCE_STATUSES:
        return f"invalid status={status or 'missing'}", ""
    if status != "VERIFIED":
        return f"status={status}", ""
    test_type = row.get("test_type", "").strip()
    if test_type not in GOVERNANCE_TEST_TYPES:
        return f"invalid test_type={test_type or 'missing'}", ""
    evidence_path = row.get("evidence_path", "").strip()
    if not evidence_path or evidence_path == "-":
        return "empty evidence_path", ""
    blob = evidence_blobs.get(evidence_path)
    if blob is None:
        return f"evidence_path not present at head: {evidence_path}", ""
    digest = hashlib.sha1(blob).hexdigest()
    for record in audit_rows:
        if record["mode"].strip().lower() != "ac-verify":
            continue
        if normalize_role(record["actor"]) != "Reviewer":
            continue
        if not record["spec_version"] or not record["adr_revision"]:
            continue
        evidence = parse_evidence_keys(record["evidence"])
        ac_match = re.fullmatch(r"AC-0*(\d{1,3})", evidence.get("ac", ""), re.I)
        if not ac_match or int(ac_match.group(1)) != ac:
            continue
        if evidence.get("test_type", "") != test_type:
            continue
        if not AUDIT_BASE_RE.match(evidence.get("base", "")) or not AUDIT_BASE_RE.match(evidence.get("head", "")):
            continue
        digest_match = AUDIT_DIGEST_RE.match(record["digest"].strip())
        if not digest_match or digest_match.group(1) != digest:
            continue
        return None, digest
    return "no Reviewer ac-verify row with matching ac/test_type/base/head/digest", ""


def check_g3(repo: Path, base: str, head: str) -> list[str]:
    failures: list[str] = []
    warnings: list[str] = []
    tracked = tracked_files(repo, head)
    if tracked is None:
        return [f"G-3 FAIL cannot enumerate tracked files at {head}"]
    spec_paths = [
        path for path in tracked
        if path.startswith("docs/specs/") and path.endswith(".md")
        and "/" not in path[len("docs/specs/"):]
    ]
    test_paths = head_test_files(tracked)
    # One head-bound batch: specs, baseline, ledger, audit log, test refs.
    head_blobs = blob_map(
        repo, head,
        sorted(spec_paths) + [BASELINE_PATH, LEDGER_PATH, AUDIT_PATH] + test_paths,
    )
    baseline_blob = head_blobs.get(BASELINE_PATH)
    if baseline_blob is None:
        return [f"G-3 FAIL missing {BASELINE_PATH} at {head}"]
    baseline = parse_baseline(baseline_blob.decode(errors="replace"))
    spec_chunks = []
    for path in sorted(spec_paths):
        blob = head_blobs.get(path)
        if blob is None:
            failures.append(f"G-3 FAIL cannot read {path} at {head}")
            continue
        spec_chunks.append(blob.decode(errors="replace"))
    current = parse_acs("\n".join(spec_chunks))
    base_set, current_set = set(baseline), set(current)
    for ac in sorted(base_set - current_set):
        failures.append(f"G-3 FAIL AC-{ac:02d} deleted")
    for ac in sorted(current_set - base_set):
        failures.append(f"G-3 FAIL AC-{ac:02d} not in baseline")
    if baseline != sorted(baseline):
        failures.append("G-3 FAIL ac-baseline.txt is not ascending")
    if baseline and baseline != list(range(1, max(baseline) + 1)):
        failures.append("G-3 FAIL ac-baseline.txt is not contiguous")
    blobs = []
    for path in test_paths:
        blob = head_blobs.get(path)
        if blob is None:
            failures.append(f"G-3 FAIL cannot read {path} at {head}")
            continue
        blobs.append(blob.decode(errors="replace"))
    all_tests = "\n".join(blobs)
    refs = set()
    for match in re.finditer(r"TestAC0*(\d{1,3})\b", all_tests):
        refs.add(int(match.group(1)))
    for match in re.finditer(r"AC-0*(\d{1,3})\b", all_tests):
        refs.add(int(match.group(1)))
    ledger_blob = head_blobs.get(LEDGER_PATH)
    ledger, ledger_errors = coverage_ledger(
        ledger_blob.decode(errors="replace") if ledger_blob is not None else None
    )
    warnings.extend(ledger_errors)
    today = datetime.date.today()
    # Business AC-01…68: unchanged AC-REF/deferral semantics.
    business = sorted(ac for ac in current_set if ac < GOVERNANCE_AC_MIN)
    uncovered = []
    for ac in business:
        if ac in refs:
            continue
        uncovered.append(ac)
        row = ledger.get(ac)
        if not row:
            failures.append(f"G-3 FAIL AC-{ac:02d} no test, no valid deferral")
            continue
        try:
            expiry = datetime.date.fromisoformat(row["expiry"])
        except ValueError:
            failures.append(f"G-3 FAIL AC-{ac:02d} invalid deferral expiry {row['expiry']}")
            continue
        if not row["owner"] or not row["reason"] or not row["thread"]:
            failures.append(f"G-3 FAIL AC-{ac:02d} deferral missing owner/reason/thread")
        elif expiry < today:
            failures.append(f"G-3 FAIL AC-{ac:02d} deferral expired {row['expiry']}")
        else:
            warnings.append(f"G-3 WARN AC-{ac:02d} deferred until {row['expiry']} ({(expiry - today).days}d)")
    # Governance AC-69…: VERIFIED + non-empty evidence_path + Reviewer signature
    # and reviewed_digest (GV-S20 ac-verify row). Refs/deferrals do not verify.
    governance = sorted(ac for ac in current_set if ac >= GOVERNANCE_AC_MIN)
    audit_rows: list[dict[str, str]] = []
    if governance:
        audit_blob = head_blobs.get(AUDIT_PATH)
        audit_rows, audit_warnings = governance_audit_rows(
            audit_blob.decode(errors="replace") if audit_blob is not None else None
        )
        warnings.extend(audit_warnings)
    governance_unverified = 0
    evidence_paths: list[str] = []
    for ac in governance:
        row = ledger.get(ac)
        if not row or row.get("status", "").strip() != "VERIFIED":
            continue
        path = row.get("evidence_path", "").strip()
        if path and path != "-" and path not in evidence_paths:
            evidence_paths.append(path)
    # Second head-bound batch: evidence artifacts + base-side baseline (--range).
    read_specs = [f"{head}:{path}" for path in evidence_paths]
    if base != head:
        read_specs.append(f"{base}:{BASELINE_PATH}")
    values = git_batch_blobs(repo, read_specs)
    evidence_blobs: dict[str, bytes] = {}
    for path, blob in zip(evidence_paths, values):
        if blob is not None:
            evidence_blobs[path] = blob
    old_blob = baseline_blob if base == head else (
        values[len(evidence_paths)] if len(values) > len(evidence_paths) else None
    )
    if old_blob is not None:
        old_set = set(parse_baseline(old_blob.decode(errors="replace")))
        for ac in sorted(old_set - base_set):
            failures.append(f"G-3 FAIL AC-{ac:02d} removed from baseline")
    for ac in governance:
        row = ledger.get(ac)
        if not row:
            print(f"G-3 NOT-RUN AC-{ac:02d} GOV_AC_EVIDENCE_MISSING (no ac-coverage.tsv row)")
            governance_unverified += 1
            continue
        status = row.get("status", "").strip()
        if status not in GOVERNANCE_STATUSES:
            failures.append(f"G-3 FAIL AC-{ac:02d} GOV_AC_EVIDENCE_MISSING (invalid status={status or 'missing'})")
            governance_unverified += 1
            continue
        problem, digest = governance_evidence_problem(ac, row, audit_rows, evidence_blobs)
        if problem is None:
            print(
                f"G-3 AC-VERIFIED AC-{ac:02d} test_type={row['test_type']} "
                f"evidence={row['evidence_path']} digest=sha1:{digest}"
            )
            continue
        governance_unverified += 1
        if status == "VERIFIED":
            failures.append(f"G-3 FAIL AC-{ac:02d} GOV_AC_EVIDENCE_MISSING ({problem})")
        else:
            print(f"G-3 NOT-RUN AC-{ac:02d} GOV_AC_EVIDENCE_MISSING ({problem})")
    for ac in sorted(set(ledger) - current_set):
        warnings.append(f"G-3 WARN AC-{ac:02d} stale ledger row")
    covered = len(current_set) - len(uncovered) - governance_unverified
    print(f"AC coverage: {covered}/{len(current_set)}")
    if uncovered:
        print("G-3 WARN uncovered: " + ", ".join(f"AC-{ac:02d}" for ac in uncovered))
    for warning in warnings:
        print(warning)
    return failures


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["warn", "strict"], default=os.environ.get("GATES_MODE", "warn"))
    parser.add_argument("--range", default=None)
    parser.add_argument("--staged", action="store_true")
    parser.add_argument("--role", default="")
    parser.add_argument("--repo", default=".")
    args = parser.parse_args()
    repo = Path(args.repo).resolve()
    if not (repo / ".git").exists():
        print(f"G-0 FAIL {repo} is not a git repository", file=sys.stderr)
        return 2
    base, head = split_range(repo, args.range)
    files = changed_files(repo, args, base, head)
    failures: list[str] = []
    range_commits = [] if args.staged else commits(repo, base, head)
    failures += check_g1(repo, args, files, head, range_commits)
    if not args.staged:
        non_normative = any(parse_trailers(body)[2] for _sha, body in range_commits)
        failures += check_g2(repo, args, files, non_normative, base, head)
    failures += check_g3(repo, base, head)
    mode = args.mode
    for failure in failures:
        print(failure)
    exit_code = 1 if (mode == "strict" and failures) else 0
    g1 = sum(1 for f in failures if f.startswith("G-1"))
    g2 = sum(1 for f in failures if f.startswith("G-2"))
    g3 = sum(1 for f in failures if f.startswith("G-3"))
    print(f"SUMMARY G-1={g1} G-2={g2} G-3={g3} mode={mode} exit={exit_code}")
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
