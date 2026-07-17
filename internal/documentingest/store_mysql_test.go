package documentingest

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

type fakeSQLResult struct {
	id  int64
	err error
}

func (r fakeSQLResult) LastInsertId() (int64, error) { return r.id, r.err }
func (r fakeSQLResult) RowsAffected() (int64, error) { return 0, nil }

func TestPositiveLastInsertIDRejectsZeroWithoutFormattingNilAsWrappedError(t *testing.T) {
	_, err := positiveLastInsertID(fakeSQLResult{}, "document source")
	if err == nil || strings.Contains(err.Error(), "%!w") {
		t.Fatalf("error=%v", err)
	}
	want := errors.New("driver failure")
	_, err = positiveLastInsertID(fakeSQLResult{err: want}, "document source")
	if !errors.Is(err, want) {
		t.Fatalf("error=%v, want wrapped driver failure", err)
	}
	var _ sql.Result = fakeSQLResult{}
}

func TestBoundDocumentBuildErrorUsesUTF8ByteLimit(t *testing.T) {
	value := boundDocumentBuildError(strings.Repeat("错", 1000))
	if len(value) > 2048 || !strings.Contains(value, "...") {
		t.Fatalf("bounded length=%d value suffix=%q", len(value), value[len(value)-8:])
	}
}

func TestValidateBuildIdentityRejectsUnsafeCollection(t *testing.T) {
	build := BuildIdentity{KnowledgeScope: "course", DocumentID: "intro", SourceRef: "docs/intro.md", ContentHash: strings.Repeat("a", 64), ParserVersion: "v1", ChunkPolicyHash: strings.Repeat("b", 64), TargetCollection: "other_collection"}
	if err := validateBuildIdentity(build); err == nil || !strings.Contains(err.Error(), "collection") {
		t.Fatalf("error=%v", err)
	}
}
