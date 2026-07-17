package documentingest

import "fmt"

type VersionStatus string

const (
	StatusPending  VersionStatus = "pending"
	StatusBuilding VersionStatus = "building"
	StatusReady    VersionStatus = "ready"
	StatusActive   VersionStatus = "active"
	StatusFailed   VersionStatus = "failed"
)

func (s VersionStatus) Valid() bool {
	switch s {
	case StatusPending, StatusBuilding, StatusReady, StatusActive, StatusFailed:
		return true
	default:
		return false
	}
}

func ValidateTransition(from, to VersionStatus) error {
	if !from.Valid() {
		return fmt.Errorf("unknown document version status %q", from)
	}
	if !to.Valid() {
		return fmt.Errorf("unknown document version status %q", to)
	}
	allowed := map[VersionStatus]VersionStatus{
		StatusPending: StatusBuilding,
		StatusFailed:  StatusBuilding,
		StatusReady:   StatusActive,
	}
	if to == allowed[from] {
		return nil
	}
	if from == StatusBuilding && (to == StatusReady || to == StatusFailed) {
		return nil
	}
	return fmt.Errorf("document version transition %q -> %q is not allowed", from, to)
}
