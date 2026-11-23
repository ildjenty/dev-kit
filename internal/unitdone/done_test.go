package unitdone

import (
	"github.com/ildjenty/dev-kit/internal/testutil"
	"testing"
)

func TestMarkerSchema(t *testing.T) {
	for _, body := range []string{`{}`, `{"unit":"U-abcdef","branch":""}`, `{"unit":"bad","branch":"feature/x"}`, `{"unit":"U-abcdef","branch":"feature/x","extra":true}`, `null`, `{`} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	marker := testutil.Must(Parse([]byte(`{"unit":"U-abcdef","branch":"feature/x"}`)))
	testutil.Equal(t, marker, Marker{Unit: "U-abcdef", Branch: "feature/x"})
}

func TestRequestUnitKeepsTheLastNamedUnitAcrossSupplements(t *testing.T) {
	dir := t.TempDir()
	testutil.Equal(t, testutil.Must(RequestUnit(dir)), "")
	for path, body := range map[string]string{"request/2.json": `{"unit":"U-abcdef"}`, "request/10.json": `{"unit":"U-fedcba"}`, "request/11.json": `{"unit":""}`, "request/notes.json": `invalid`} {
		testutil.Put(t, dir, path, body)
	}
	testutil.Equal(t, testutil.Must(RequestUnit(dir)), "U-fedcba")
	testutil.Equal(t, testutil.Must(Exists(dir, "units", "U-fedcba")), false)
	testutil.Put(t, dir, "units/U-fedcba.done.json", "{}")
	testutil.Equal(t, testutil.Must(Exists(dir, "units", "U-fedcba")), true)
}
