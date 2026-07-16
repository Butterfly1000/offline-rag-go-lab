package contextretrieval

import (
	"errors"
	"strings"
	"testing"
)

func TestSourceErrorClassifiesInfrastructureAndIntegrity(t *testing.T) {
	cause := errors.New("timeout")
	infra := InfrastructureFailure(SourceMemory, cause)
	if !IsInfrastructureFailure(infra) {
		t.Fatal("infrastructure failure was not classified")
	}
	if !errors.Is(infra, cause) {
		t.Fatal("source error must unwrap its cause")
	}
	if !strings.Contains(infra.Error(), "memory infrastructure failure") {
		t.Fatalf("infrastructure error = %q", infra)
	}

	integrity := IntegrityFailure(SourceDocument, errors.New("wrong scope"))
	if IsInfrastructureFailure(integrity) {
		t.Fatal("integrity failure must remain hard")
	}
	if !strings.Contains(integrity.Error(), "document integrity failure") {
		t.Fatalf("integrity error = %q", integrity)
	}
}

func TestSourceErrorRejectsInvalidConstruction(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "unknown source", err: InfrastructureFailure("cache", errors.New("down"))},
		{name: "nil cause", err: IntegrityFailure(SourceMemory, nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil || !strings.Contains(tt.err.Error(), "invalid source failure") {
				t.Fatalf("error = %v", tt.err)
			}
			if IsInfrastructureFailure(tt.err) {
				t.Fatal("invalid failure must not be downgraded")
			}
		})
	}
}

func TestIsInfrastructureFailureRejectsMalformedSourceError(t *testing.T) {
	malformed := &SourceError{
		Source: "cache",
		Kind:   FailureInfrastructure,
		Err:    errors.New("down"),
	}
	if IsInfrastructureFailure(malformed) {
		t.Fatal("malformed source error must not be downgraded")
	}
}
