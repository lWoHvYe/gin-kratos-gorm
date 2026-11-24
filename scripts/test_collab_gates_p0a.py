#!/usr/bin/env python3
"""P0a T-A1/T-A2 fixtures for G-3 governance evidence + head-bound reads.

Synthetic repositories only (never real history). The gate is invoked through
scripts/check-collab-gates.sh exactly as the Makefile/CI entrypoints do.

Covers SPEC-0003 AC-99 (governance evidence path), AC-71/72/102 (explicit
--range and head-bound reads), AC-106 bootstrap NOT-RUN semantics.
"""
from __future__ import annotations

import hashlib
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
GATES = ROOT / "scripts" / "check-collab-gates.sh"
EVIDENCE = "docs/lead/GOV-TEST-01/evidence/gates.log"
LEDGER_HEADER = "# ac\towner\texpiry\treason\tthread\ttest_type\tevidence_path\tstatus\n"
AUDIT_HEADER = "\t".join([
    "timestamp", "change_id", "parent_change_id", "effective_tier", "tier_source",
    "actor", "base", "head", "mode", "verdict", "spec_version", "adr_revision",
    "evidence", "digest", "supersedes",
]) + "\n"
checks = 0
failures: list[str] = []


def git(repo: Path, *args: str) -> str:
    proc = subprocess.run(["git", "-C", str(repo), *args], text=True, capture_output=True)
    if proc.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)} failed: {proc.stderr.strip()}")
    return proc.stdout


def init(repo: Path) -> None:
    for sub in ("docs/specs", "docs/collab", "internal"):
        (repo / sub).mkdir(parents=True, exist_ok=True)
    git(repo, "init", "-q")
    git(repo, "config", "user.email", "dev@example.test")
    git(repo, "config", "user.name", "Dev")


def commit(repo: Path, message: str, trailers: list[str]) -> None:
    git(repo, "add", "-A")
    args = ["commit", "-q", "-m", message]
    for trailer in trailers:
        args += ["--trailer", trailer]
    git(repo, *args)


def write_spec(repo: Path, acs, version: str = "1.0") -> None:
    lines = [
        "```yaml", "id: SPEC-9001", f"version: {version}", "```",
        "# Demo", "## 13. 变更记录", f"| {version} | fixture |",
    ]
    lines += [f"**AC-{ac:02d}** fixture {ac}" for ac in acs]
    (repo / "docs/specs/0001-demo.md").write_text("\n".join(lines) + "\n")


def write_readme(repo: Path, version: str = "1.0") -> None:
    (repo / "docs/specs/README.md").write_text(
        f"| 编号 | 版本 |\n|---|---|\n| [0001](0001-demo.md) | {version} |\n"
    )


def write_baseline(repo: Path, acs) -> None:
    (repo / "docs/collab/ac-baseline.txt").write_text("".join(f"AC-{ac:02d}\n" for ac in acs))


def write_tests(repo: Path, acs) -> None:
    lines = ["package demo", ""]
    lines += [f"func TestAC{ac:02d}(t *testing.T) {{}}" for ac in acs]
    (repo / "internal/demo_test.go").write_text("\n".join(lines) + "\n")


def write_ledger(repo: Path, rows: list[str]) -> None:
    (repo / "docs/collab/ac-coverage.tsv").write_text(LEDGER_HEADER + "".join(r + "\n" for r in rows))


def governance_row(status: str, evidence_path: str = EVIDENCE, test_type: str = "script") -> str:
    return f"AC-69\tReviewer\t2026-12-31\tgovernance fixture\tgov-test/thread\t{test_type}\t{evidence_path}\t{status}"


def write_evidence(repo: Path, text: str = "gates --mode=strict output\n") -> str:
    path = repo / EVIDENCE
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    return hashlib.sha1(text.encode()).hexdigest()


def write_audit(repo: Path, digest: str, *, actor: str = "Reviewer", mode: str = "ac-verify",
                test_type: str = "script") -> None:
    evidence = f"ac=AC-69;test_type={test_type};base=0000000;head=0000000"
    row = [
        "2026-11-24T12:00:00+08:00", "GOV-TEST-01", "-", "L2", "GV-S1", actor,
        "0000000", "0000000", mode, "AC-VERIFIED",
        "v1.4", "Rev 1.1",
        evidence, f"sha1:{digest}", "-",
    ]
    (repo / "docs/review-tracking.md").write_text(
        "# Review Tracking\n\n## Governance audit log\n\n" + AUDIT_HEADER + "\t".join(row) + "\n"
    )


def run_gates(repo: Path, *extra: str) -> tuple[int, str]:
    proc = subprocess.run(
        [str(GATES), "--mode=strict", f"--repo={repo}", *extra],
        text=True, capture_output=True,
    )
    return proc.returncode, proc.stdout + proc.stderr


def check(label: str, condition: bool, out: str = "") -> None:
    global checks
    checks += 1
    if condition:
        print(f"PASS {label}")
    else:
        failures.append(label)
        print(f"FAIL {label}")
        if out:
            print(out)


def governance_fixture(repo: Path, *, row: str, evidence: bool = True, audit: bool = True,
                       digest_text: str = "gates --mode=strict output\n",
                       audit_digest: str | None = None) -> None:
    init(repo)
    write_spec(repo, range(1, 70))
    write_readme(repo)
    write_baseline(repo, range(1, 70))
    write_tests(repo, range(1, 70))  # TestAC69 exists on purpose: refs alone must not verify
    write_ledger(repo, [row])
    if evidence:
        digest = write_evidence(repo, digest_text)
    else:
        digest = hashlib.sha1(b"absent").hexdigest()
    if audit:
        write_audit(repo, audit_digest or digest)
    commit(repo, "governance fixture", [
        "change_id: GOV-TEST-01", "Actor: dev", "Reviewed-by: reviewer",
        "Role: Product Owner, Developer",
    ])


def business_fixture(repo: Path) -> None:
    init(repo)
    write_spec(repo, (1, 2))
    write_readme(repo)
    write_baseline(repo, (1, 2))
    write_tests(repo, (1,))
    commit(repo, "business fixture", ["Role: Product Owner, Developer"])


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="gates-p0a-") as tmpdir:
        tmp = Path(tmpdir)

        # T-A1: VERIFIED + evidence_path + Reviewer signature + reviewed_digest.
        repo = tmp / "verified"
        governance_fixture(repo, row=governance_row("VERIFIED"))
        rc, out = run_gates(repo)
        check("T-A1 governance VERIFIED passes", rc == 0, out)
        check("T-A1 governance VERIFIED emits AC-VERIFIED", "G-3 AC-VERIFIED AC-69" in out, out)
        check("T-A1 governance VERIFIED has no missing-evidence code", "GOV_AC_EVIDENCE_MISSING" not in out, out)

        # T-A1: VERIFIED claim with empty evidence_path is fail-closed.
        repo = tmp / "missing-evidence"
        governance_fixture(repo, row=governance_row("VERIFIED", "-"), evidence=False)
        rc, out = run_gates(repo)
        check("T-A1 VERIFIED without evidence_path fails", rc == 1, out)
        check("T-A1 VERIFIED without evidence_path emits code", "GOV_AC_EVIDENCE_MISSING" in out, out)

        # T-A1: VERIFIED claim without Reviewer signature/digest row.
        repo = tmp / "no-signature"
        governance_fixture(repo, row=governance_row("VERIFIED"), audit=False)
        rc, out = run_gates(repo)
        check("T-A1 VERIFIED without signature fails", rc == 1, out)
        check("T-A1 VERIFIED without signature emits code", "GOV_AC_EVIDENCE_MISSING" in out, out)

        # T-A1: digest does not match the head artifact.
        repo = tmp / "digest-mismatch"
        governance_fixture(repo, row=governance_row("VERIFIED"), audit_digest="0" * 40)
        rc, out = run_gates(repo)
        check("T-A1 digest mismatch fails", rc == 1, out)
        check("T-A1 digest mismatch emits code", "GOV_AC_EVIDENCE_MISSING" in out, out)

        # T-A1: PENDING is NOT-RUN (bootstrap), never AC-VERIFIED and never business deferral.
        repo = tmp / "pending"
        governance_fixture(repo, row=governance_row("PENDING"), audit=False)
        rc, out = run_gates(repo)
        check("T-A1 PENDING is not verified", "G-3 NOT-RUN AC-69" in out and "G-3 AC-VERIFIED AC-69" not in out, out)
        check("T-A1 PENDING emits GOV_AC_EVIDENCE_MISSING", "GOV_AC_EVIDENCE_MISSING" in out, out)
        check("T-A1 PENDING does not use business ref/deferral", "AC-69 no test, no valid deferral" not in out, out)

        # T-A1: DEFERRED is NOT-RUN, never AC-VERIFIED.
        repo = tmp / "deferred"
        governance_fixture(repo, row=governance_row("DEFERRED"), audit=False)
        rc, out = run_gates(repo)
        check("T-A1 DEFERRED is not verified", "G-3 NOT-RUN AC-69" in out and "G-3 AC-VERIFIED AC-69" not in out, out)
        check("T-A1 DEFERRED emits GOV_AC_EVIDENCE_MISSING", "GOV_AC_EVIDENCE_MISSING" in out, out)

        # AC-99: business AC-01…68 keep AC-REF/deferral semantics.
        repo = tmp / "business-deferral"
        business_fixture(repo)
        write_ledger(repo, ["AC-02\tDeveloper\t2099-12-31\tbusiness fixture\tgov-test/thread\tdeferred\t-\tDEFERRED"])
        commit(repo, "business deferral", ["change_id: GOV-TEST-01", "Actor: dev", "Reviewed-by: reviewer",
                                           "Role: Product Owner, Developer"])
        rc, out = run_gates(repo)
        check("T-A1 business deferral still accepted", rc == 0 and "G-3 WARN AC-02 deferred" in out, out)

        repo = tmp / "business-missing"
        business_fixture(repo)
        rc, out = run_gates(repo)
        check("T-A1 business missing ref/deferral still fails", rc == 1 and "AC-02 no test, no valid deferral" in out, out)

        # T-A2: untracked _test.go is never AC-REF evidence (head-bound).
        repo = tmp / "untracked-test"
        business_fixture(repo)
        (repo / "internal/untracked_test.go").write_text(
            "package demo\nfunc TestAC02(t *testing.T) {}\n"
        )
        untracked = git(repo, "status", "--porcelain")
        rc, out = run_gates(repo)
        check("T-A2 untracked test file stays untracked", "?? internal/untracked_test.go" in untracked, untracked)
        check("T-A2 untracked test file is ignored", rc == 1 and "AC-02 no test, no valid deferral" in out, out)

        # T-A2: untracked evidence artifact is never AC-VERIFIED evidence.
        repo = tmp / "untracked-evidence"
        content = "worktree-only evidence artifact\n"
        governance_fixture(
            repo, row=governance_row("VERIFIED"), evidence=False,
            audit_digest=hashlib.sha1(content.encode()).hexdigest(),
        )
        (repo / EVIDENCE).parent.mkdir(parents=True, exist_ok=True)
        (repo / EVIDENCE).write_text(content)
        untracked = git(repo, "status", "--porcelain", "-uall")
        rc, out = run_gates(repo)
        check("T-A2 untracked evidence stays untracked", f"?? {EVIDENCE}" in untracked, untracked)
        check("T-A2 untracked evidence is not AC-VERIFIED", rc == 1 and "GOV_AC_EVIDENCE_MISSING" in out
              and "G-3 AC-VERIFIED AC-69" not in out, out)

        # T-A2: ledger is read from head, never from the working tree.
        repo = tmp / "ledger-head"
        governance_fixture(repo, row=governance_row("PENDING"), audit=False)
        (repo / "docs/collab/ac-coverage.tsv").write_text(
            LEDGER_HEADER + governance_row("VERIFIED") + "\n"
        )
        rc, out = run_gates(repo)
        check("T-A2 worktree ledger change is ignored",
              "G-3 NOT-RUN AC-69" in out and "G-3 AC-VERIFIED AC-69" not in out, out)

        # Fail-closed: missing baseline / unreadable range head never PASS.
        repo = tmp / "missing-baseline"
        init(repo)
        write_spec(repo, (1,))
        write_readme(repo)
        write_tests(repo, (1,))
        commit(repo, "missing baseline", ["Role: Product Owner, Developer"])
        rc, out = run_gates(repo)
        check("G-3 missing baseline is fail-closed", rc == 1 and "missing docs/collab/ac-baseline.txt" in out, out)

        repo = tmp / "bad-range"
        business_fixture(repo)
        rc, out = run_gates(repo, "--range=deadbeef..deadbeef")
        check("G-3 unreadable range head is fail-closed", rc == 1 and "cannot enumerate tracked files" in out, out)

        # T-A2: explicit --range drives baseline deletion/continuity and head reads.
        repo = tmp / "range"
        init(repo)
        write_spec(repo, (1, 2), "1.0")
        write_readme(repo, "1.0")
        write_baseline(repo, (1, 2))
        write_tests(repo, (1, 2))
        commit(repo, "range base", ["change_id: GOV-TEST-01", "Actor: dev", "Reviewed-by: reviewer",
                                    "Role: Product Owner, Developer"])
        base = git(repo, "rev-parse", "HEAD").strip()
        write_spec(repo, (1, 3), "1.1")
        write_readme(repo, "1.1")
        write_baseline(repo, (1, 3))
        write_tests(repo, (1, 3))
        commit(repo, "range head", ["change_id: GOV-TEST-01", "Actor: dev", "Reviewed-by: reviewer",
                                    "Role: Product Owner, Developer"])
        head = git(repo, "rev-parse", "HEAD").strip()
        rc, out = run_gates(repo)
        check("T-A2 default range detects baseline deletion", rc == 1 and "AC-02 removed from baseline" in out, out)
        check("T-A2 default range checks head continuity", "ac-baseline.txt is not contiguous" in out, out)
        rc, out = run_gates(repo, f"--range={head}..{head}")
        check("T-A2 explicit range skips stale deletion", "AC-02 removed from baseline" not in out, out)
        check("T-A2 explicit range still checks continuity", "ac-baseline.txt is not contiguous" in out, out)
        rc, out = run_gates(repo, f"--range={base}..{base}")
        check("T-A2 explicit range binds G-3 head", rc == 0, out)
        check("T-A2 explicit range has no deletion noise", "removed from baseline" not in out, out)

    if failures:
        print(f"test_collab_gates_p0a: {len(failures)} FAIL ({checks} checks)", file=sys.stderr)
        return 1
    print(f"test_collab_gates_p0a: {checks} checks PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
