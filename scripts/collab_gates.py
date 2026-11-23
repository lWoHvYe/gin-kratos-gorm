#!/usr/bin/env python3
"""Collaboration gates G-1/G-2/G-3 (ADR 0006).

Read-only: inspects git history, Spec versions and test references; never mutates
the repository. Bash wrapper is scripts/check-collab-gates.sh.
"""
from __future__ import annotations

import argparse
import datetime
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


def run(repo: Path, args: list[str], check: bool = False) -> str:
    proc = subprocess.run(["git", "-C", str(repo), *args], text=True, capture_output=True)
    if check and proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or f"git {' '.join(args)} failed")
    return proc.stdout


def normalize_role(value: str) -> str:
    return ROLE_ALIASES.get(value.strip().lower(), value.strip())


def changed_files(repo: Path, args: argparse.Namespace) -> list[str]:
    if args.staged:
        out = run(repo, ["diff", "--cached", "--name-only"])
        return [line.strip() for line in out.splitlines() if line.strip()]
    base, head = split_range(repo, args.range)
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


def check_g1(repo: Path, args: argparse.Namespace, files: list[str]) -> list[str]:
    failures: list[str] = []
    warnings: list[str] = []
    managed = [f for f in files if any(f.startswith(prefix) for prefix in MANAGED)]
    if not managed:
        return failures
    if not args.staged:
        _base, head = split_range(repo, args.range)
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
        base, head = split_range(repo, args.range)
        roles = set()
        exceptions = []
        for _sha, body in commits(repo, base, head):
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


def check_g2(repo: Path, args: argparse.Namespace, files: list[str], non_normative: bool) -> list[str]:
    failures: list[str] = []
    base, head = split_range(repo, args.range)
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


def test_files(repo: Path) -> list[Path]:
    out = run(repo, ["ls-files", "--cached", "--others", "--exclude-standard"])
    files = []
    for line in out.splitlines():
        path = line.strip()
        if not path.endswith("_test.go"):
            continue
        if "/.tmp/" in f"/{path}/" or "/node_modules/" in f"/{path}/" or path.startswith("vendor/"):
            continue
        files.append(repo / path)
    return files


def coverage_ledger(repo: Path) -> tuple[dict[int, dict[str, str]], list[str]]:
    path = repo / "docs/collab/ac-coverage.tsv"
    ledger: dict[int, dict[str, str]] = {}
    errors: list[str] = []
    if not path.exists():
        return ledger, errors
    for line in path.read_text(errors="replace").splitlines():
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
        }
    return ledger, errors


def check_g3(repo: Path) -> list[str]:
    failures: list[str] = []
    warnings: list[str] = []
    baseline_path = repo / "docs/collab/ac-baseline.txt"
    if not baseline_path.exists():
        return ["G-3 FAIL missing docs/collab/ac-baseline.txt"]
    baseline = parse_baseline(baseline_path.read_text())
    spec_text = "\n".join((repo / p).read_text(errors="replace") for p in sorted((repo / "docs/specs").glob("*.md")))
    current = parse_acs(spec_text)
    base_set, current_set = set(baseline), set(current)
    for ac in sorted(base_set - current_set):
        failures.append(f"G-3 FAIL AC-{ac:02d} deleted")
    for ac in sorted(current_set - base_set):
        failures.append(f"G-3 FAIL AC-{ac:02d} not in baseline")
    if baseline != sorted(baseline):
        failures.append("G-3 FAIL ac-baseline.txt is not ascending")
    if baseline and baseline != list(range(1, max(baseline) + 1)):
        failures.append("G-3 FAIL ac-baseline.txt is not contiguous")
    base_ref, _head = split_range(repo, None)
    old_text = git_show(repo, base_ref, "docs/collab/ac-baseline.txt")
    if old_text:
        old_set = set(parse_baseline(old_text))
        for ac in sorted(old_set - base_set):
            failures.append(f"G-3 FAIL AC-{ac:02d} removed from baseline")
    files = test_files(repo)
    blobs = []
    for path in files:
        try:
            blobs.append(path.read_text(errors="replace"))
        except OSError:
            continue
    all_tests = "\n".join(blobs)
    refs = set()
    for match in re.finditer(r"TestAC0*(\d{1,3})\b", all_tests):
        refs.add(int(match.group(1)))
    for match in re.finditer(r"AC-0*(\d{1,3})\b", all_tests):
        refs.add(int(match.group(1)))
    ledger, ledger_errors = coverage_ledger(repo)
    warnings.extend(ledger_errors)
    today = datetime.date.today()
    uncovered = []
    for ac in sorted(current_set):
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
    for ac in sorted(set(ledger) - current_set):
        warnings.append(f"G-3 WARN AC-{ac:02d} stale ledger row")
    covered = len(current_set) - len(uncovered)
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
    files = changed_files(repo, args)
    failures: list[str] = []
    failures += check_g1(repo, args, files)
    if not args.staged:
        base, head = split_range(repo, args.range)
        non_normative = any(parse_trailers(body)[2] for _sha, body in commits(repo, base, head))
        failures += check_g2(repo, args, files, non_normative)
    failures += check_g3(repo)
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
