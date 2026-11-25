package retro

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestStartRejectsInvalidRestartLimits(t *testing.T) {
	for _, value := range []any{0, -1, "1"} {
		r := setup(t, backlog())
		r.json(".devkit/config.json", doc{"limits": doc{"restarts": value}})
		r.commit()
		_, err := start(r.ctx, envVars(nil))
		testutil.Fails(t, err, "limits.restarts must be an integer of at least 1")
		testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "run"))), 0)
	}
}

func TestBacklogEnumerationDistinguishesMissingEmptyAndUnreadable(t *testing.T) {
	for _, path := range []string{".devkit", ".devkit/backlog", ".devkit/backlog/escapes"} {
		t.Run(path, func(t *testing.T) {
			cwd := t.TempDir()
			testutil.Put(t, cwd, path, "not a directory")
			_, _, err := readBacklog(cwd)
			testutil.Fails(t, err, filepath.Join(cwd, path))
		})
	}
	cwd := t.TempDir()
	for _, path := range []string{"", ".devkit/backlog", ".devkit/backlog/escapes"} {
		testutil.OK(t, os.MkdirAll(filepath.Join(cwd, path), 0755))
		entries, files, err := readBacklog(cwd)
		testutil.OK(t, err)
		testutil.Equal(t, entries, []doc{})
		testutil.Equal(t, files, doc{})
	}
	path := filepath.Join(cwd, ".devkit/backlog/escapes")
	testutil.OK(t, os.Chmod(path, 0000))
	t.Cleanup(func() { _ = os.Chmod(path, 0755) })
	_, _, err := readBacklog(cwd)
	testutil.Fails(t, err, path)
}

func TestReadDraftReportsSyntaxAndKeepsAbsence(t *testing.T) {
	a := t.TempDir()
	d, err := readDraft(a)
	testutil.OK(t, err)
	testutil.Equal(t, d, any(nil))
	testutil.OK(t, put(file(a, "draft.json"), "{"))
	_, err = readDraft(a)
	testutil.Fails(t, err, file(a, "draft.json"))
	testutil.Fails(t, err, "unexpected end")
}

func TestFinishDoesNotInventProposalVersionBeforeCommit(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	testutil.Must(round(r.ctx, r.artifacts, nil))
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "stop")
	j := testutil.Must(record.Latest(r.ctx.Dir, "judgment"))
	testutil.Equal(t, obj(j["target"])["proposal_version"], any(nil))
	testutil.Equal(t, j["verdict"], "not_assessed")
	testutil.Contains(t, str(stringsOf(j["observations"])[0]), "proposal")
}

func TestLaterProposalDoesNotFillEarlierUncommittedRound(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	testutil.Must(round(r.ctx, r.artifacts, nil))
	r.write(r.draft())
	testutil.Must(check(r.ctx, r.artifacts))
	testutil.Must(round(r.ctx, r.artifacts, judgment()))
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "question")
	judgments := testutil.Must(record.List(r.ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 2)
	testutil.Equal(t, obj(judgments[0]["target"])["proposal_version"], any(nil))
	testutil.Equal(t, obj(judgments[1]["target"])["proposal_version"], any(float64(1)))
}

func TestFinishRejectsMissingOrInvalidRoundProposalVersion(t *testing.T) {
	for _, version := range []any{"missing", 0, "1", 1.5} {
		r := setup(t, backlog())
		r.begin()
		r.write(r.draft())
		testutil.Must(check(r.ctx, r.artifacts))
		testutil.Must(round(r.ctx, r.artifacts, judgment()))
		entry := testutil.Must(readJSON(file(r.artifacts, "rounds/1.json")))
		if version == "missing" {
			delete(entry, "proposal_version")
		} else {
			entry["proposal_version"] = version
		}
		testutil.OK(t, writeJSON(file(r.artifacts, "rounds/1.json"), entry))
		_, err := finish(r.ctx, r.artifacts)
		testutil.Fails(t, err, file(r.artifacts, "rounds/1.json"))
		testutil.Fails(t, err, "proposal_version")
		testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "judgment")), []record.Document{})
		testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "result")), []record.Document{})
	}
}
