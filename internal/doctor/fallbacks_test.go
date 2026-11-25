package doctor

import (
	"errors"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

type failingOutput struct{ calls int }

func (w *failingOutput) Write(b []byte) (int, error) {
	w.calls++
	if w.calls == 2 {
		return 0, errors.New("diagnostic output unavailable")
	}
	return len(b), nil
}

func TestDoctorReportsOutputFailure(t *testing.T) {
	env, _, _ := environment(t, "claude")
	env.Stdout = &failingOutput{}
	err := Commands()[0].Run(nil, env)
	testutil.Fails(t, err, "diagnostic output unavailable")
	env.Stdout = &failingOutput{}
	testutil.Equal(t, Report(env), false)
}
