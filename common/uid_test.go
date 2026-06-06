package common

import "testing"

func TestGroupUIDStart(t *testing.T) {
	tests := []struct {
		name    string
		groupID uint32
		want    uint64
	}{
		{name: "group 1", groupID: 1, want: 1000000000001},
		{name: "group 2", groupID: 2, want: 2000000000001},
		{name: "group 3", groupID: 3, want: 3000000000001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GroupUIDStart(tt.groupID); got != tt.want {
				t.Fatalf("GroupUIDStart(%d) = %d, want %d", tt.groupID, got, tt.want)
			}
		})
	}
}
