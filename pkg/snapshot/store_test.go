package snapshot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsSnapshotID(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		snapshotID string
		expected   bool
	}{
		{
			name:       "valid snapshot key",
			key:        "snapshots/2026/04/08/scan-123.json",
			snapshotID: "scan-123",
			expected:   true,
		},
		{
			name:       "key too short - should not panic",
			key:        "latest.json",
			snapshotID: "scan-2026-04-08-10-00-00",
			expected:   false,
		},
		{
			name:       "empty key - should not panic",
			key:        "",
			snapshotID: "scan-123",
			expected:   false,
		},
		{
			name:       "key without .json suffix",
			key:        "snapshots/2026/04/08/scan-123",
			snapshotID: "scan-123",
			expected:   false,
		},
		{
			name:       "different snapshot ID",
			key:        "snapshots/2026/04/08/scan-123.json",
			snapshotID: "scan-456",
			expected:   false,
		},
		{
			name:       "snapshot ID at end of key",
			key:        "prefix/scan-xyz.json",
			snapshotID: "scan-xyz",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := containsSnapshotID(tt.key, tt.snapshotID)
			assert.Equal(t, tt.expected, result)
		})
	}
}
