package contextretrieval

import (
	"errors"
	"fmt"
)

type FailureKind string

const (
	FailureInfrastructure FailureKind = "infrastructure"
	FailureIntegrity      FailureKind = "integrity"
)

// SourceError distinguishes failures that chat may safely degrade from data
// integrity failures that must stop prompt construction.
type SourceError struct {
	Source Source
	Kind   FailureKind
	Err    error
}

func (e *SourceError) Error() string {
	return fmt.Sprintf("%s %s failure: %v", e.Source, e.Kind, e.Err)
}

func (e *SourceError) Unwrap() error {
	return e.Err
}

func InfrastructureFailure(source Source, err error) error {
	return newSourceError(source, FailureInfrastructure, err)
}

func IntegrityFailure(source Source, err error) error {
	return newSourceError(source, FailureIntegrity, err)
}

func IsInfrastructureFailure(err error) bool {
	var sourceErr *SourceError
	return errors.As(err, &sourceErr) &&
		knownSource(sourceErr.Source) &&
		sourceErr.Kind == FailureInfrastructure &&
		sourceErr.Err != nil
}

func newSourceError(source Source, kind FailureKind, err error) error {
	if !knownSource(source) || err == nil {
		return fmt.Errorf("invalid source failure: source=%q kind=%q cause=%v", source, kind, err)
	}
	return &SourceError{Source: source, Kind: kind, Err: err}
}

func knownSource(source Source) bool {
	return source == SourceMemory || source == SourceDocument
}
