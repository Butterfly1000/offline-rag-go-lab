#!/bin/sh

set -eu

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
repo_root=$(CDPATH= cd "$script_dir/../.." && pwd)
cd "$repo_root"

live=false
if [ "${1:-}" = "--live" ]; then
	live=true
elif [ "$#" -ne 0 ]; then
	printf 'usage: sh scripts/regression/lessons-11-12.sh [--live]\n' >&2
	exit 2
fi

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

ollama_url=${OLLAMA_BASE_URL:-http://127.0.0.1:11434}
model=${OLLAMA_MODEL:-qwen:7b}
tags=$(curl -sS --max-time 10 "$ollama_url/api/tags") || fail "cannot reach Ollama at $ollama_url"
printf '%s\n' "$tags" | grep -F '"name":"'"$model"'"' >/dev/null || fail "Ollama model is missing: $model"
pass "lesson 11 Ollama model is available: $model"

GOCACHE=${GOCACHE:-$repo_root/.cache/go-build}
export GOCACHE
mkdir -p "$GOCACHE"

expected_tokenizer_sha=${RECENT_CHAT_TOKENIZER_SHA256:-b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8}
go run ./cmd/tokenizer-inspect --tokenizer "$tokenizer_path" --expect-sha256 "$expected_tokenizer_sha" >/dev/null
pass "tokenizer fingerprint matches $expected_tokenizer_sha"

go test ./internal/promptbudget ./internal/recentchat -run 'Test(Automatic|HTTPOllamaClientContextLength|ServiceAutomatic)' -count=1
pass "lesson 11-12 focused tests"

budget_output=$(go run ./cmd/automatic-budget-demo \
	--base-url "$ollama_url" \
	--model "$model" \
	--system '你是 Go 助手。' \
	--prompt '解释 recent window。' \
	--output-reserve 2048 \
	--tokenizer "$tokenizer_path")
context_limit=$(printf '%s\n' "$budget_output" | awk -F': ' '/^Context limit:/ {print $2}')
fixed_tokens=$(printf '%s\n' "$budget_output" | awk -F': ' '/^Fixed input tokens:/ {print $2}')
output_reserve=$(printf '%s\n' "$budget_output" | awk -F': ' '/^Output reserve tokens:/ {print $2}')
available=$(printf '%s\n' "$budget_output" | awk -F': ' '/^Available recent history tokens:/ {print $2}')
[ -n "$context_limit" ] && [ -n "$fixed_tokens" ] && [ -n "$output_reserve" ] && [ -n "$available" ] || fail "automatic budget output is incomplete"
[ $((fixed_tokens + output_reserve + available)) -eq "$context_limit" ] || fail "automatic budget equation does not balance"
if [ "$model" = "qwen:7b" ]; then
	[ "$context_limit" = "32768" ] || fail "qwen:7b context limit=$context_limit, want 32768"
	[ "$fixed_tokens" = "56" ] || fail "lesson 11 fixed input tokens=$fixed_tokens, want 56"
	[ "$available" = "30664" ] || fail "lesson 11 available history tokens=$available, want 30664"
fi
pass "lesson 11 automatic budget equation balances"

overflow_file="$repo_root/.cache/lessons-11-12-overflow.txt"
trap 'rm -f "$overflow_file" "$repo_root/.cache/lessons-11-12-live.txt" "$repo_root/.cache/lessons-11-12-conflict.txt"' 0 HUP INT TERM
if go run ./cmd/automatic-budget-demo --base-url "$ollama_url" --model "$model" --output-reserve "$context_limit" --tokenizer "$tokenizer_path" >"$overflow_file" 2>&1; then
	fail "context overflow unexpectedly succeeded"
fi
grep -F 'exceeds context limit' "$overflow_file" >/dev/null || fail "overflow failed without the expected diagnostic"
pass "lesson 11 impossible fixed/output budget is rejected"

if [ "$live" = false ]; then
	printf '\nLessons 11-12 no-business-write regression passed.\n'
	printf 'Live lesson 12 skipped. Start recent-chat and rerun with --live.\n'
	exit 0
fi

chat_url=${RECENT_CHAT_URL:-http://127.0.0.1:18093}
session_id="regression-lesson-12-$(date +%s)-$$"
live_file="$repo_root/.cache/lessons-11-12-live.txt"
http_code=$(curl -sS --max-time 180 -o "$live_file" -w '%{http_code}' -X POST "$chat_url/chat" \
	-H 'Content-Type: application/json' \
	-d '{"session_id":"'"$session_id"'","user_id":"regression-user","message":"只回复 Go。","model":"'"$model"'","recent_limit":10,"auto_token_budget":true,"output_token_reserve":64,"store_user_turn":true,"store_assistant_turn":true}') || fail "cannot reach recent-chat at $chat_url; start cmd/recent-chat and its local dependencies"
[ "$http_code" = "200" ] || {
	cat "$live_file" >&2
	fail "lesson 12 automatic request HTTP=$http_code"
}
grep -F '"budget_mode":"automatic"' "$live_file" >/dev/null || fail "lesson 12 response lacks automatic budget mode"
grep -F '"output_token_reserve":64' "$live_file" >/dev/null || fail "lesson 12 response lacks output reserve"
pass "lesson 12 live automatic /chat request"

conflict_file="$repo_root/.cache/lessons-11-12-conflict.txt"
conflict_code=$(curl -sS --max-time 30 -o "$conflict_file" -w '%{http_code}' -X POST "$chat_url/chat" \
	-H 'Content-Type: application/json' \
	-d '{"session_id":"'"$session_id"'-conflict","user_id":"regression-user","message":"参数冲突。","model":"'"$model"'","auto_token_budget":true,"recent_token_budget":10,"output_token_reserve":64,"store_user_turn":false,"store_assistant_turn":false}') || fail "conflict request could not reach recent-chat"
[ "$conflict_code" = "400" ] || {
	cat "$conflict_file" >&2
	fail "lesson 12 conflicting budget request HTTP=$conflict_code, want 400"
}
grep -F 'auto_token_budget' "$conflict_file" >/dev/null || fail "conflict response does not identify auto_token_budget"
grep -F 'recent_token_budget' "$conflict_file" >/dev/null || fail "conflict response does not identify recent_token_budget"
pass "lesson 12 conflicting automatic/manual budget is rejected"

printf '\nLessons 11-12 live regression passed.\n'
