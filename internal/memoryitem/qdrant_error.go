package memoryitem

import "errors"

// QdrantDataError marks a successful Qdrant HTTP response whose returned data
// cannot be trusted. Callers must not degrade this class like a network outage.
type QdrantDataError struct {
	Err error
}

func (e *QdrantDataError) Error() string {
	if e == nil || e.Err == nil {
		return "Qdrant returned invalid data"
	}
	return e.Err.Error()
}

func (e *QdrantDataError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsQdrantDataError(err error) bool {
	var target *QdrantDataError
	return errors.As(err, &target) && target != nil && target.Err != nil
}

func qdrantDataError(err error) error {
	return &QdrantDataError{Err: err}
}
