package middlewares

import "testing"

func TestIsAllowedOrigin(t *testing.T) {
	tests := []struct {
		origin string
		want   bool
	}{
		{origin: "http://localhost:5173", want: true},
		{origin: "http://localhost:3000"},
		{origin: "https://localhost:5173"},
		{origin: "https://leaderboard.leoding.com"},
	}

	for _, tt := range tests {
		if got := isAllowedOrigin(tt.origin); got != tt.want {
			t.Errorf("isAllowedOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}
