package config

import "testing"

func TestDetectionRequired(t *testing.T) {
	for _, body := range []string{"", "\n[detection]\nrequired = true"} {
		settings, err := Parse([]byte("kit_version = 'v1'" + body))
		if err != nil || !settings.DetectionRequired() {
			t.Fatalf("detection default: %+v, %v", settings, err)
		}
	}
	settings, err := Parse([]byte("kit_version = 'v1'\n[detection]\nrequired = false"))
	if err != nil || settings.DetectionRequired() {
		t.Fatalf("detection override: %+v, %v", settings, err)
	}
	if _, err := Parse([]byte("kit_version = 'v1'\n[detection]\nrequired = 'false'")); err == nil {
		t.Fatal("accepted a string detection setting")
	}
}
