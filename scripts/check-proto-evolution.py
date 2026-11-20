#!/usr/bin/env python3
"""Guard proto field numbers. --write refreshes the baseline intentionally."""
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PROTO = ROOT / "api/proto/mall/v1/mall.proto"
BASELINE = ROOT / "api/proto/mall/v1/mall.proto.baseline.json"

MSG = re.compile(r"^message\s+(\w+)\s*\{")
RPC = re.compile(r"^\s*rpc\s+(\w+)\s*\(([^)]*)\)\s*returns\s*\(([^)]*)\)")
RESERVED = re.compile(r"^\s*reserved\s+(.+);")
FIELD = re.compile(r"^\s+(?:repeated\s+|optional\s+)?[\w.<>]+\s+(\w+)\s*=\s*(\d+)\s*;")


def snapshot() -> dict:
    out = {}
    message = None
    for line in PROTO.read_text().splitlines():
        m = MSG.match(line)
        if m:
            message = m.group(1)
            continue
        r = RPC.match(line)
        if r:
            out["rpc." + r.group(1)] = r.group(2).strip() + " -> " + r.group(3).strip()
            continue
        if message:
            reserved = RESERVED.match(line)
            if reserved:
                numbers = []
                for part in reserved.group(1).split(","):
                    part = part.strip()
                    if " to " in part:
                        lo, hi = part.split(" to ", 1)
                        numbers.extend(range(int(lo), int(hi) + 1))
                    elif part:
                        numbers.append(int(part))
                out["reserved." + message] = sorted(numbers)
                continue
            f = FIELD.match(line)
            if f and not line.strip().startswith("reserved"):
                out[message + "." + f.group(1)] = int(f.group(2))
    return out


def main() -> int:
    current = snapshot()
    if "--write" in sys.argv:
        BASELINE.write_text(json.dumps(current, indent=2, sort_keys=True) + "\n")
        print(f"[proto-guard] baseline written: {len(current)} entries")
        return 0
    if not BASELINE.exists():
        print("[proto-guard] missing baseline; run make proto-baseline", file=sys.stderr)
        return 1
    base = json.loads(BASELINE.read_text())
    by_msg = {}
    for key, number in current.items():
        if key.startswith("rpc."):
            continue
        message = key.rsplit(".", 1)[0]
        by_msg.setdefault(message, {}).setdefault(number, []).append(key)
    failed = False
    for key, number in base.items():
        if isinstance(number, list):
            if sorted(current.get(key, [])) != sorted(number):
                print(f"[proto-guard] reserved changed: {key} = {number}", file=sys.stderr)
                failed = True
            continue
        if current.get(key) != number:
            print(f"[proto-guard] removed/changed: {key} = {number}", file=sys.stderr)
            failed = True
    for message, numbers in by_msg.items():
        for number, keys in numbers.items():
            if len(keys) > 1 and any(k in base for k in keys):
                print(f"[proto-guard] {message} field {number} reused by {sorted(keys)}", file=sys.stderr)
                failed = True
    if failed:
        return 1
    print(f"[proto-guard] OK ({len(base)} baseline entries)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
