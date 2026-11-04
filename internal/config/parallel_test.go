package config

import "testing"

func TestParallelWorkerLimit(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int
	}{
		{"", 3},
		{"[parallel]\nmax_workers = 1\n", 1},
		{"[parallel]\nmax_workers = 5\n", 5},
	} {
		settings, err := Parse([]byte("kit_version = 'v2'\n" + tc.body))
		if err != nil || settings.MaxWorkers() != tc.want {
			t.Fatalf("worker limit = %d, error = %v; want %d", settings.MaxWorkers(), err, tc.want)
		}
	}
	for _, body := range []string{"max_workers = 0", "max_workers = -1"} {
		if _, err := Parse([]byte("kit_version = 'v2'\n[parallel]\n" + body)); err == nil {
			t.Fatal("accepted a disabled worker limit")
		}
	}
}
