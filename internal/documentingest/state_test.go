package documentingest

import (
	"strings"
	"testing"
)

func TestValidateTransitionAllowsDocumentBuildLifecycle(t *testing.T) {
	tests := []struct {
		from VersionStatus
		to   VersionStatus
	}{
		{StatusPending, StatusBuilding},
		{StatusBuilding, StatusReady},
		{StatusBuilding, StatusFailed},
		{StatusFailed, StatusBuilding},
		{StatusReady, StatusActive},
	}
	for _, tt := range tests {
		if err := ValidateTransition(tt.from, tt.to); err != nil {
			t.Errorf("ValidateTransition(%q, %q) error = %v", tt.from, tt.to, err)
		}
	}
}

func TestValidateTransitionRejectsSkippedRepeatedAndActiveMutation(t *testing.T) {
	tests := []struct {
		from VersionStatus
		to   VersionStatus
		want string
	}{
		{StatusPending, StatusReady, "pending"},
		{StatusPending, StatusFailed, "pending"},
		{StatusBuilding, StatusActive, "building"},
		{StatusReady, StatusBuilding, "ready"},
		{StatusActive, StatusFailed, "active"},
		{StatusFailed, StatusReady, "failed"},
		{StatusPending, StatusPending, "pending"},
		{"unknown", StatusBuilding, "unknown"},
		{StatusPending, "unknown", "unknown"},
	}
	for _, tt := range tests {
		_, _ = tt.from, tt.to
		if err := ValidateTransition(tt.from, tt.to); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ValidateTransition(%q, %q) error = %v, want %q", tt.from, tt.to, err, tt.want)
		}
	}
}

func TestVersionStatusValid(t *testing.T) {
	for _, status := range []VersionStatus{StatusPending, StatusBuilding, StatusReady, StatusActive, StatusFailed} {
		if !status.Valid() {
			t.Errorf("status %q should be valid", status)
		}
	}
	if VersionStatus("unknown").Valid() {
		t.Fatal("unknown status should be invalid")
	}
}
