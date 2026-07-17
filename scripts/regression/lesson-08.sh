#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

pass() {
	printf '[PASS] %s\n' "$1"
}

fail() {
	printf '[FAIL] %s\n' "$1" >&2
	exit 1
}

command -v go >/dev/null 2>&1 || fail "Go is not installed or not in PATH"
pass "Go command: $(go version)"

gomod=$(go env GOMOD)
[ "$gomod" = "$repo_root/go.mod" ] || fail "wrong module: GOMOD=$gomod"
pass "module root: $gomod"

tokenizer_path=${RECENT_CHAT_TOKENIZER_PATH:-assets/tokenizers/qwen2/tokenizer.json}
[ -f "$tokenizer_path" ] || fail "missing tokenizer asset: $tokenizer_path; run scripts/bootstrap/tokenizer-asset.sh SOURCE first"
QWEN_TOKENIZER_PATH=$(CDPATH= cd -- "$(dirname -- "$tokenizer_path")" && pwd)/$(basename -- "$tokenizer_path")
export QWEN_TOKENIZER_PATH
pass "Qwen2 tokenizer asset exists: $QWEN_TOKENIZER_PATH"

[ -f third_party/github.com/sugarme/tokenizer/go.mod ] || fail "missing local sugarme/tokenizer replacement"
replacement=$(go list -m -f '{{if .Replace}}{{.Replace.Dir}}{{end}}' github.com/sugarme/tokenizer)
[ "$replacement" = "$repo_root/third_party/github.com/sugarme/tokenizer" ] || fail "sugarme/tokenizer does not resolve to the repository replacement: $replacement"
pass "sugarme/tokenizer resolves to local compatibility code"

GOCACHE=${GOCACHE:-$repo_root/.cache/go-build}
export GOCACHE
mkdir -p "$GOCACHE"

go test ./internal/tokenizerdemo ./internal/chatprompt
pass "Tokenizer and Qwen message-format tests"

tokenizer_output=$(go run ./cmd/tokenizer-demo --tokenizer "$QWEN_TOKENIZER_PATH" --text '我叫小黄，这个项目是 Go 写的。')
printf '%s\n' "$tokenizer_output" | grep -F 'Token count: 15' >/dev/null || {
	printf '%s\n' "$tokenizer_output" >&2
	fail "Chinese golden text did not produce 15 tokens"
}
pass "Chinese golden token count is 15"

message_output=$(go run ./cmd/message-format-demo --role user --content '你好，解释 token。')
printf '%s\n' "$message_output" | grep -F '<|im_start|>user' >/dev/null || fail "formatted user boundary is missing"
printf '%s\n' "$message_output" | grep -F '你好，解释 token。<|im_end|>' >/dev/null || fail "formatted content or end boundary is missing"
pass "valid Qwen message boundaries"

invalid_output_file="$repo_root/.cache/lesson-08-invalid-role.txt"
trap 'rm -f "$invalid_output_file"' EXIT HUP INT TERM
if go run ./cmd/message-format-demo --role unknown --content '测试' >"$invalid_output_file" 2>&1; then
	fail "unknown role unexpectedly succeeded"
fi
grep -F 'unsupported message role "unknown"' "$invalid_output_file" >/dev/null || fail "unknown role failed without the expected diagnostic"
rm -f "$invalid_output_file"
trap - EXIT HUP INT TERM
pass "invalid role is rejected"

printf '\nLesson 08 cross-environment regression passed.\n'
