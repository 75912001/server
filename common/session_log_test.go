package common

import "testing"

func TestShortSession(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: ""},
		{name: "short", value: "123456789012", want: "123456789012"},
		{name: "long", value: "1234567890abcdef", want: "123456...abcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShortSession(tt.value); got != tt.want {
				t.Fatalf("ShortSession(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
