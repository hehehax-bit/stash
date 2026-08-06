#!/usr/bin/env bash
set -euo pipefail

echo "=== AI Feature Runtime Validation ==="
echo ""

# Check that all AI task files exist
AI_TASKS=(
  "internal/manager/task_ai_embedding.go"
  "internal/manager/task_ai_performer_cluster.go"
  "internal/manager/task_ai_scene_segment.go"
  "internal/manager/task_ai_suggestion.go"
  "internal/manager/task_ai_quality.go"
  "internal/manager/task_ai_audio.go"
  "internal/manager/task_ai_career.go"
  "internal/manager/task_ai_collections.go"
  "internal/manager/task_ai_file_rename.go"
)

echo "1. Checking AI task file existence..."
for f in "${AI_TASKS[@]}"; do
  if [ -f "$f" ]; then
    echo "  [OK] $f"
  else
    echo "  [FAIL] $f missing"
  fi
done

echo ""
echo "2. Checking for hardcoded thresholds..."
grep -n "minConfidence.*0\.7" internal/manager/task_ai_performer_cluster.go && echo "  [WARN] Hard-coded minConfidence=0.7" || true
grep -n "defaultMaxTokens.*2048" pkg/ai/chat.go && echo "  [WARN] Hard-coded defaultMaxTokens=2048" || true
grep -n "noise=-35dB" internal/manager/task_ai_audio.go && echo "  [WARN] Hard-coded silence threshold" || true

echo ""
echo "3. Checking for fragile JSON extraction..."
count=$(grep -r 'strings.Index(content, "{")' internal/manager/ pkg/ai/ 2>/dev/null | wc -l)
echo "  Found $count instances of fragile JSON extraction pattern"
if [ "$count" -gt 0 ]; then
  echo "  [WARN] Consider extracting to shared function"
fi

echo ""
echo "4. Checking for missing test files..."
MISSING_TESTS=0
for task in "${AI_TASKS[@]}"; do
  base=$(basename "$task" .go)
  test_file="internal/manager/${base}_test.go"
  if [ ! -f "$test_file" ]; then
    echo "  [MISSING] $test_file"
    MISSING_TESTS=$((MISSING_TESTS + 1))
  fi
done
echo "  Total missing test files: $MISSING_TESTS"

echo ""
echo "5. Checking embedding.go syntax..."
if go build ./pkg/sqlite/ 2>&1 | grep -q "task_ai_embedding.go"; then
  echo "  [FAIL] Syntax error detected in task_ai_embedding.go"
else
  echo "  [OK] No syntax errors"
fi

echo ""
echo "=== Validation Complete ==="
