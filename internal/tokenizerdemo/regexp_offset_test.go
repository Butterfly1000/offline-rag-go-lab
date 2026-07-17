package tokenizerdemo

import (
	"testing"

	"github.com/sugarme/tokenizer/normalizer"
)

func TestRegexp2BackreferenceOffsetsUseUTF8Bytes(t *testing.T) {
	pattern := normalizer.NewRegexpPattern(`([!])\1*`)
	matches := pattern.FindMatches("中文!!尾")
	if len(matches) != 3 {
		t.Fatalf("matches = %#v, want unmatched/matched/unmatched", matches)
	}
	want := [][2]int{{0, 6}, {6, 8}, {8, 11}}
	for index, offsets := range want {
		if matches[index].Offsets[0] != offsets[0] || matches[index].Offsets[1] != offsets[1] {
			t.Fatalf("match %d offsets = %#v, want %v", index, matches[index].Offsets, offsets)
		}
	}
}
