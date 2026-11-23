package config

import (
	"github.com/ildjenty/dev-kit/internal/testutil"
	"testing"
)

func TestProviderAndSessionDefaultsAndValidation(t *testing.T) {
	for _, tc := range []struct {
		json, provider string
		sessions       []string
	}{
		{`{}`, "claude", []string{"claude"}},
		{`{"provider":"codex"}`, "codex", []string{"codex"}},
		{`{"provider":"codex","sessions":["claude","codex"]}`, "codex", []string{"claude", "codex"}},
		{`{"sessions":[]}`, "claude", []string{}},
		{`{"provider":"claude","sessions":["codex"]}`, "claude", []string{"codex"}},
	} {
		c := testutil.Must(Parse([]byte(tc.json)))
		testutil.Equal(t, c.Provider, tc.provider)
		testutil.Equal(t, c.Sessions, tc.sessions)
	}
	for _, input := range []string{`{"provider":"other"}`, `{"provider":null}`, `{"sessions":null}`, `{"sessions":"codex"}`, `{"sessions":["other"]}`, `{"sessions":["claude","claude"]}`, `{"sessions":[1]}`} {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}
