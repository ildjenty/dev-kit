package config

import "testing"

func TestParallelConcurrentLimit(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int
	}{
		{"", 3},
		{"[parallel]\nmax_concurrent = 1\n", 1},
		{"[parallel]\nmax_concurrent = 5\n", 5},
	} {
		settings, err := Parse([]byte("kit_version = 'v2'\n" + tc.body))
		if err != nil || settings.MaxConcurrent() != tc.want {
			t.Fatalf("concurrency limit = %d, error = %v; want %d", settings.MaxConcurrent(), err, tc.want)
		}
	}
	for _, body := range []string{"max_concurrent = 0", "max_concurrent = -1"} {
		if _, err := Parse([]byte("kit_version = 'v2'\n[parallel]\n" + body)); err == nil {
			t.Fatal("accepted a disabled concurrency limit")
		}
	}
}
