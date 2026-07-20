#!/bin/sh

set -eu

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
repo_root=$(CDPATH= cd "$script_dir/../.." && pwd)
cd "$repo_root"

pass() {
	printf '[PASS] %s\n' "$1"
}

fail() {
	printf '[FAIL] %s\n' "$1" >&2
	exit 1
}

command -v go >/dev/null 2>&1 || fail "Go is not installed or not in PATH"
command -v curl >/dev/null 2>&1 || fail "curl is not installed or not in PATH"

tokenizer_path=${RECENT_CHAT_TOKENIZER_PATH:-assets/tokenizers/qwen2/tokenizer.json}
[ -f "$tokenizer_path" ] || fail "missing tokenizer asset: $tokenizer_path; run scripts/bootstrap/tokenizer-asset.sh SOURCE first"
tokenizer_path=$(CDPATH= cd "$(dirname "$tokenizer_path")" && pwd)/$(basename "$tokenizer_path")
QWEN_TOKENIZER_PATH=$tokenizer_path
export QWEN_TOKENIZER_PATH

[ -f sql/session_summaries.sql ] || fail "missing sql/session_summaries.sql"
grep -F 'PRIMARY KEY (session_id, user_id)' sql/session_summaries.sql >/dev/null || fail "session summary ownership key is missing"
pass "lesson 13 session summary schema is present"

GOCACHE=${GOCACHE:-$repo_root/.cache/go-build}
export GOCACHE
mkdir -p "$GOCACHE"

expected_tokenizer_sha=${RECENT_CHAT_TOKENIZER_SHA256:-b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8}
go run ./cmd/tokenizer-inspect --tokenizer "$tokenizer_path" --expect-sha256 "$expected_tokenizer_sha" >/dev/null
pass "tokenizer fingerprint matches $expected_tokenizer_sha"

go test ./internal/sessionsummary ./internal/recentchat -run 'Test(Trigger|SelectPrefix|FormattedMessageCounter|BuildUpdatePrompt|Generator|HTTPOllamaClientGenerateText)' -count=1
pass "lesson 13-15 focused tests"

assert_trigger() {
	expected_summary=$1
	expected_reason=$2
	shift 2
	output=$(go run ./cmd/summary-trigger-demo "$@")
	printf '%s\n' "$output" | grep -F "Should summarize: $expected_summary" >/dev/null || fail "trigger summary decision mismatch: $expected_reason"
	printf '%s\n' "$output" | grep -F "Reason: $expected_reason" >/dev/null || fail "trigger reason mismatch: $expected_reason"
}

assert_trigger false no_evicted_messages --messages 10 --tokens 5000 --evicted 0
assert_trigger true message_threshold --messages 8 --tokens 1000 --evicted 2
assert_trigger true token_threshold --messages 3 --tokens 3000 --evicted 1
assert_trigger false below_threshold --messages 3 --tokens 1000 --evicted 1
pass "lesson 13 trigger reasons cover all branches"

invalid_file="$repo_root/.cache/lessons-13-15-invalid-trigger.txt"
trap 'rm -f "$invalid_file"' 0 HUP INT TERM
if go run ./cmd/summary-trigger-demo --messages 2 --tokens 1000 --evicted 3 >"$invalid_file" 2>&1; then
	fail "invalid trigger statistics unexpectedly succeeded"
fi
grep -F 'cannot exceed unsummarized messages' "$invalid_file" >/dev/null || fail "invalid trigger failed without expected diagnostic"
pass "lesson 13 invalid trigger statistics are rejected"

selection=$(go run ./cmd/summary-selection-demo \
	--tokenizer "$tokenizer_path" \
	--ids '19,20,21,22,23,24,25,26' \
	--watermark 20 \
	--recent-start 25)
printf '%s\n' "$selection" | grep -F 'Unsummarized IDs: 21,22,23,24,25,26' >/dev/null || fail "lesson 14 unsummarized IDs mismatch"
printf '%s\n' "$selection" | grep -F 'Evicted IDs: 21,22,23,24' >/dev/null || fail "lesson 14 evicted prefix mismatch"
printf '%s\n' "$selection" | grep -F 'Unsummarized tokens: 129' >/dev/null || fail "lesson 14 unsummarized token golden mismatch"
printf '%s\n' "$selection" | grep -F 'Evicted tokens: 86' >/dev/null || fail "lesson 14 evicted token golden mismatch"
printf '%s\n' "$selection" | grep -F 'Next watermark: 24' >/dev/null || fail "lesson 14 watermark mismatch"

hole_selection=$(go run ./cmd/summary-selection-demo \
	--tokenizer "$tokenizer_path" \
	--ids '19,20,21,23,30' \
	--watermark 20 \
	--recent-start 30)
printf '%s\n' "$hole_selection" | grep -F 'Evicted IDs: 21,23' >/dev/null || fail "lesson 14 ID-hole prefix mismatch"
printf '%s\n' "$hole_selection" | grep -F 'Next watermark: 23' >/dev/null || fail "lesson 14 ID-hole watermark mismatch"
pass "lesson 14 prefix selection and ID holes"

ollama_url=${OLLAMA_BASE_URL:-http://127.0.0.1:11434}
model=${OLLAMA_MODEL:-qwen:7b}
tags=$(curl -sS --max-time 10 "$ollama_url/api/tags") || fail "cannot reach Ollama at $ollama_url"
printf '%s\n' "$tags" | grep -F '"name":"'"$model"'"' >/dev/null || fail "Ollama model is missing: $model"

summary_output=$(go run ./cmd/summary-generate-demo \
	--base-url "$ollama_url" \
	--model "$model" \
	--previous '用户叫小黄，代码示例使用 Go。' \
	--max-tokens 128)
generated=$(printf '%s\n' "$summary_output" | awk 'found {print} /^Updated summary:$/ {found=1}')
[ -n "$(printf '%s' "$generated" | tr -d '[:space:]')" ] || fail "lesson 15 generated an empty summary"
printf '%s\n' "$generated" | grep -F '<updated_summary>' >/dev/null && fail "lesson 15 leaked updated_summary wrapper"
pass "lesson 15 Ollama returned a non-empty wrapper-free summary"

printf '\nLessons 13-15 cross-environment regression passed.\n'
