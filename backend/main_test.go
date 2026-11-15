package main

import "testing"

func TestValidEmail(t *testing.T) {
	for _, tc := range []struct {
		email string
		want  bool
	}{
		{"agent@example.com", true}, {"invalid", false}, {"x@y", false}, {"", false},
	} {
		if got := validEmail(tc.email); got != tc.want {
			t.Fatalf("validEmail(%q)=%v want %v", tc.email, got, tc.want)
		}
	}
}
