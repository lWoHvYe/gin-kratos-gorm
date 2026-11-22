#!/usr/bin/env bash
# ADR 0006 A1…A5: synthetic repositories only; never touches real history.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
gates="$root/scripts/check-collab-gates.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
pass=0

gitinit() {
  local dir="$1"
  mkdir -p "$dir/docs/specs" "$dir/docs/collab" "$dir/internal"
  git -C "$dir" init -q
  git -C "$dir" config user.email dev@example.test
  git -C "$dir" config user.name Dev
  cat > "$dir/docs/specs/0001-demo.md" <<'EOF'
```yaml
id: SPEC-0001
version: 1.0
```
# Demo
## 13. 变更记录
| 1.0 | init |
**AC-01** one
**AC-02** two
EOF
  cat > "$dir/docs/specs/README.md" <<'EOF'
| 编号 | 版本 |
|---|---|
| [0001](0001-demo.md) | 1.0 |
EOF
  printf 'AC-01\nAC-02\n' > "$dir/docs/collab/ac-baseline.txt"
  cat > "$dir/internal/demo_test.go" <<'EOF'
package demo
func TestAC01(t *testing.T) {}
func TestAC02(t *testing.T) {}
EOF
  git -C "$dir" add -A
  git -C "$dir" commit -qm 'base' --trailer 'Role: Product Owner, Developer'
}

expect_fail() {
  local dir="$1" label="$2" needle="$3"
  if out="$("$gates" --mode=strict --repo="$dir" 2>&1)"; then
    echo "A? FAIL $label: expected strict failure" >&2
    echo "$out" >&2
    exit 1
  fi
  if ! grep -q "$needle" <<<"$out"; then
    echo "A? FAIL $label: missing '$needle'" >&2
    echo "$out" >&2
    exit 1
  fi
  echo "PASS $label"
  pass=$((pass + 1))
}

# A1: Developer touches docs/specs.
d="$tmp/a1"; gitinit "$d"
printf '\nedit\n' >> "$d/docs/specs/0001-demo.md"
git -C "$d" add -A
git -C "$d" commit -qm 'wrong owner' --trailer 'Role: Developer'
expect_fail "$d" "A1 ownership" "expected=Product Owner"

# A2a: Spec body changes without version bump.
d="$tmp/a2a"; gitinit "$d"
printf '\nedit\n' >> "$d/docs/specs/0001-demo.md"
git -C "$d" add -A
git -C "$d" commit -qm 'no bump' --trailer 'Role: Product Owner'
expect_fail "$d" "A2a no bump" "no bump"

# A2b: version bump without README index sync.
d="$tmp/a2b"; gitinit "$d"
sed -i.bak 's/version: 1.0/version: 1.1/' "$d/docs/specs/0001-demo.md"
sed -i.bak 's/| 1.0 |/| 1.1 |/' "$d/docs/specs/0001-demo.md"
printf '\n| 1.1 | bump |\n' >> "$d/docs/specs/0001-demo.md"
rm -f "$d/docs/specs/0001-demo.md.bak"
git -C "$d" add -A
git -C "$d" commit -qm 'index drift' --trailer 'Role: Product Owner'
expect_fail "$d" "A2b index drift" "index=1.0 yaml=1.1"

# A3: unique AC test reference removed.
d="$tmp/a3"; gitinit "$d"
python3 - "$d/internal/demo_test.go" <<'PY'
import sys
p = sys.argv[1]
text = open(p).read().replace("func TestAC02(t *testing.T) {}\n", "")
open(p, "w").write(text)
PY
git -C "$d" add -A
git -C "$d" commit -qm 'drop AC-02 test' --trailer 'Role: Developer'
expect_fail "$d" "A3 AC coverage" "AC-02"

# A4: a valid commit passes strict.
d="$tmp/a4"; gitinit "$d"
sed -i.bak 's/version: 1.0/version: 1.1/' "$d/docs/specs/0001-demo.md"
sed -i.bak 's/| 1.0 |/| 1.1 |/' "$d/docs/specs/README.md"
printf '\n| 1.1 | valid |\n' >> "$d/docs/specs/0001-demo.md"
rm -f "$d/docs/specs/0001-demo.md.bak" "$d/docs/specs/README.md.bak"
printf '\n// AC-02 note\n' >> "$d/internal/demo_test.go"
git -C "$d" add -A
git -C "$d" commit -qm 'valid bump' --trailer 'Role: Product Owner, Developer'
if ! out="$("$gates" --mode=strict --repo="$d" 2>&1)"; then
  echo "A4 FAIL: valid commit rejected" >&2
  echo "$out" >&2
  exit 1
fi
echo "PASS A4 valid commit"
pass=$((pass + 1))

# A5: strict runtime < 5s.
start="$(python3 -c 'import time;print(time.time())')"
"$gates" --mode=strict --repo="$d" >/dev/null
end="$(python3 -c 'import time;print(time.time())')"
elapsed="$(python3 -c "print($end-$start)")"
if python3 -c "raise SystemExit(0 if $elapsed < 5 else 1)"; then
  echo "PASS A5 runtime ${elapsed}s"
  pass=$((pass + 1))
else
  echo "A5 FAIL runtime ${elapsed}s" >&2
  exit 1
fi

echo "gates-test: $pass/5 PASS"
