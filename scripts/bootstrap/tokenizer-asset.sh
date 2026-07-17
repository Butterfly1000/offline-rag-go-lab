#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
source_path=${1:-${RECENT_CHAT_TOKENIZER_SOURCE:-}}
destination=${2:-$repo_root/assets/tokenizers/qwen2/tokenizer.json}

if [ -z "$source_path" ]; then
	printf 'usage: sh scripts/bootstrap/tokenizer-asset.sh SOURCE [DESTINATION]\n' >&2
	exit 2
fi
if [ ! -f "$source_path" ]; then
	printf 'tokenizer source does not exist: %s\n' "$source_path" >&2
	exit 1
fi

mkdir -p "$(dirname -- "$destination")"
cp "$source_path" "$destination"
printf 'Tokenizer asset initialized: %s\n' "$destination"
printf 'Next: sh scripts/regression/lesson-08.sh\n'
