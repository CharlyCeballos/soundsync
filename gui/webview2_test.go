package main

import "testing"

func TestHasRuntime(t *testing.T) {
	cases := []struct {
		versions []string
		want     bool
	}{
		{nil, false},
		{[]string{""}, false},
		{[]string{"0.0.0.0"}, false},
		{[]string{"0.0.0.0", "141.0.3537.71"}, true},
		{[]string{"141.0.3537.71"}, true},
	}
	for _, c := range cases {
		if got := hasRuntime(c.versions); got != c.want {
			t.Errorf("hasRuntime(%q) = %v, want %v", c.versions, got, c.want)
		}
	}
}
