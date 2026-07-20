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
tokenizer_path=${RECENT_CHAT_TOKENIZER_PATH:-assets/tokenizers/qwen2/tokenizer.json}
[ -f "$tokenizer_path" ] || fail "missing tokenizer asset: $tokenizer_path; run scripts/bootstrap/tokenizer-asset.sh SOURCE first"
tokenizer_path=$(CDPATH= cd "$(dirname "$tokenizer_path")" && pwd)/$(basename "$tokenizer_path")
QWEN_TOKENIZER_PATH=$tokenizer_path
export QWEN_TOKENIZER_PATH

GOCACHE=${GOCACHE:-$repo_root/.cache/go-build}
export GOCACHE
mkdir -p "$GOCACHE"

expected_tokenizer_sha=${RECENT_CHAT_TOKENIZER_SHA256:-b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8}
go run ./cmd/tokenizer-inspect --tokenizer "$tokenizer_path" --expect-sha256 "$expected_tokenizer_sha" >/dev/null
pass "tokenizer fingerprint matches $expected_tokenizer_sha"

go test ./internal/chatprompt ./internal/recentchat -run 'Test(TokenCounter|FormattedTokenWindow)' -count=1
pass "lesson 09-10 focused tests"

full_output=$(go run ./cmd/conversation-token-demo \
	--tokenizer "$tokenizer_path" \
	--system '你是 Go 助手。' \
	--history-user '我叫小黄。' \
	--history-assistant '记住了。' \
	--prompt '我叫什么？')
full_tokens=$(printf '%s\n' "$full_output" | awk -F': ' '/^Total prompt tokens:/ {print $2}')
[ "$full_tokens" = "100" ] || {
	printf '%s\n' "$full_output" >&2
	fail "lesson 09 full conversation tokens=$full_tokens, want 100"
}
last_rendered_line=$(printf '%s\n' "$full_output" | awk '/^Total prompt tokens:/ {print previous} {previous=$0}')
[ "$last_rendered_line" = '<|im_start|>assistant' ] || fail "assistant generation prefix is not the final rendered line"
pass "lesson 09 full conversation golden count is 100"

short_output=$(go run ./cmd/conversation-token-demo \
	--tokenizer "$tokenizer_path" \
	--history-user '' \
	--history-assistant '')
short_tokens=$(printf '%s\n' "$short_output" | awk -F': ' '/^Total prompt tokens:/ {print $2}')
[ "$short_tokens" = "56" ] || {
	printf '%s\n' "$short_output" >&2
	fail "lesson 09 no-history tokens=$short_tokens, want 56"
}
[ $((full_tokens - short_tokens)) -eq 44 ] || fail "history token delta is not 44"
pass "lesson 09 removing history reduces the prompt by 44 tokens"

go test ./internal/recentchat -run 'TestFormattedTokenWindowDoesNotForceOversizedNewestMessage' -count=1
pass "lesson 10 strict window rejects an oversized newest message"

printf '\nLessons 09-10 cross-environment regression passed.\n'
