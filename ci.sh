#!/bin/bash
# Tony's CI gate — runs before every merge
# Usage: ./ci.sh [file.py] or ./ci.sh (runs on all .py files)

set -e
PASS=0
FAIL=0
REPORT=""

echo "================================================"
echo "  TONY CI GATE — $(date -u '+%Y-%m-%d %H:%M UTC')"
echo "================================================"

TARGET=${1:-"*.py"}

# 1. Ruff linting
echo ""
echo "[1/3] RUFF — Style & lint check"
if ruff check $TARGET 2>&1; then
    echo "✓ Ruff passed"
    PASS=$((PASS+1))
    REPORT="$REPORT\n✅ Ruff: PASS"
else
    echo "✗ Ruff failed"
    FAIL=$((FAIL+1))
    REPORT="$REPORT\n❌ Ruff: FAIL"
fi

# 2. Bandit security scan
echo ""
echo "[2/3] BANDIT — Security vulnerability scan"
if bandit -r . -ll -q 2>&1; then
    echo "✓ Bandit passed"
    PASS=$((PASS+1))
    REPORT="$REPORT\n✅ Bandit: PASS"
else
    echo "⚠ Bandit found warnings (non-blocking)"
    REPORT="$REPORT\n⚠️  Bandit: warnings (non-blocking)"
    PASS=$((PASS+1))
fi

# 3. Pytest
echo ""
echo "[3/3] PYTEST — Unit tests"
if [ -d "tests" ] && [ "$(ls tests/test_*.py 2>/dev/null)" ]; then
    if pytest tests/ -v --tb=short 2>&1; then
        echo "✓ Pytest passed"
        PASS=$((PASS+1))
        REPORT="$REPORT\n✅ Pytest: PASS"
    else
        echo "✗ Pytest failed"
        FAIL=$((FAIL+1))
        REPORT="$REPORT\n❌ Pytest: FAIL"
    fi
else
    echo "⚠ No tests found — skipping"
    REPORT="$REPORT\n⚠️  Pytest: NO TESTS"
fi

echo ""
echo "================================================"
echo "  RESULTS: $PASS passed, $FAIL failed"
echo "================================================"
printf "$REPORT\n"

if [ $FAIL -gt 0 ]; then
    echo ""
    echo "❌ CI GATE FAILED — fix issues before merging"
    exit 1
else
    echo ""
    echo "✅ CI GATE PASSED — safe to merge"
    exit 0
fi
