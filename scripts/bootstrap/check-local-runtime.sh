#!/bin/sh

set -u

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
config_path=${RECENT_CHAT_CONFIG:-$repo_root/config/recent-chat.env}

failures=0
warnings=0

pass() {
	printf '[OK] %s\n' "$1"
}

warn() {
	warnings=$((warnings + 1))
	printf '[WARN] %s\n' "$1"
}

fail() {
	failures=$((failures + 1))
	printf '[FAIL] %s\n' "$1"
}

have_command() {
	command -v "$1" >/dev/null 2>&1
}

config_value() {
	key=$1
	file=$2
	sed -n "s#^$key=##p" "$file" | tail -n 1
}

printf '== offline-rag-go-lab local runtime check ==\n'
printf 'repo: %s\n' "$repo_root"
printf '\n'

printf '[1/5] 基础命令\n'
if have_command git; then
	pass "git: $(git --version)"
else
	fail "git is missing"
fi
if have_command go; then
	pass "go: $(go version)"
else
	fail "go is missing"
fi
if have_command curl; then
	pass "curl: available"
else
	warn "curl is missing; HTTP probes will be skipped"
fi
printf '\n'

printf '[2/5] Docker\n'
if have_command docker; then
	pass "docker: $(docker --version)"
	if docker compose version >/tmp/offline-rag-compose-version.$$ 2>/tmp/offline-rag-compose-version.err.$$; then
		pass "docker compose: $(cat /tmp/offline-rag-compose-version.$$)"
	else
		fail "docker compose is missing or unavailable"
	fi
	rm -f /tmp/offline-rag-compose-version.$$ /tmp/offline-rag-compose-version.err.$$

	if docker info >/dev/null 2>&1; then
		pass "Docker daemon: running"
		(
			cd "$repo_root"
			docker compose ps
		)
	else
		warn "Docker daemon is not running; start Docker Desktop, then rerun this script"
	fi
else
	fail "docker is missing; install Docker Desktop for Mac"
fi
printf '\n'

printf '[3/5] 本地配置\n'
if [ -f "$config_path" ]; then
	pass "config file exists: $config_path"
	mysql_dsn=$(config_value RECENT_CHAT_MYSQL_DSN "$config_path")
	if [ -n "$mysql_dsn" ] && [ "$mysql_dsn" != "YOUR_MYSQL_DSN" ]; then
		pass "RECENT_CHAT_MYSQL_DSN is configured"
	else
		fail "RECENT_CHAT_MYSQL_DSN is not configured"
	fi
	qdrant_url=$(config_value QDRANT_BASE_URL "$config_path")
	ollama_url=$(config_value OLLAMA_BASE_URL "$config_path")
	tokenizer_path=$(config_value RECENT_CHAT_TOKENIZER_PATH "$config_path")
else
	warn "config file is missing: $config_path"
	printf '      next: cp config/recent-chat.env.docker.example config/recent-chat.env\n'
	qdrant_url=http://127.0.0.1:6333
	ollama_url=http://127.0.0.1:11434
	tokenizer_path=assets/tokenizers/qwen2/tokenizer.json
fi
[ -n "${qdrant_url:-}" ] || qdrant_url=http://127.0.0.1:6333
[ -n "${ollama_url:-}" ] || ollama_url=http://127.0.0.1:11434
[ -n "${tokenizer_path:-}" ] || tokenizer_path=assets/tokenizers/qwen2/tokenizer.json
printf '\n'

printf '[4/5] 本地资产\n'
case "$tokenizer_path" in
	/*) tokenizer_abs=$tokenizer_path ;;
	*) tokenizer_abs=$repo_root/$tokenizer_path ;;
esac
if [ -f "$tokenizer_abs" ]; then
	pass "tokenizer asset exists: $tokenizer_path"
else
	fail "missing tokenizer asset: $tokenizer_path"
	printf '      next: sh scripts/bootstrap/tokenizer-asset.sh /path/to/tokenizer.json\n'
fi
printf '\n'

printf '[5/5] 本地服务探测\n'
if have_command curl; then
	if curl -fsS --max-time 3 "$qdrant_url/collections" >/dev/null 2>&1; then
		pass "Qdrant HTTP is reachable: $qdrant_url"
	else
		warn "Qdrant is not reachable at $qdrant_url"
		printf '      next: docker compose up -d mysql qdrant\n'
	fi
	if curl -fsS --max-time 3 "$ollama_url/api/tags" >/dev/null 2>&1; then
		pass "Ollama is reachable: $ollama_url"
	else
		warn "Ollama is not reachable at $ollama_url"
		printf '      next: ollama serve\n'
	fi
else
	warn "skipped HTTP service probes because curl is missing"
fi
printf '\n'

if [ "$failures" -eq 0 ]; then
	printf 'Result: OK with %d warning(s).\n' "$warnings"
	printf 'Next dependency check: go run ./cmd/rag-real-loop-demo check --config config/recent-chat.env\n'
	exit 0
fi

printf 'Result: %d failure(s), %d warning(s).\n' "$failures" "$warnings"
exit 1
