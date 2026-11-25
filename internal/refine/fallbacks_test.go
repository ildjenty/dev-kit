package refine

import (
	"testing"

	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

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
	r := setup(t, nil)
	r.begin()
	testutil.Must(round(r.ctx, r.artifacts, nil))
	testutil.Equal(t, r.finish()["outcome"], "stop")
	j := testutil.Must(record.Latest(r.ctx.Dir, "judgment"))
	testutil.Equal(t, obj(j["target"])["proposal_version"], any(nil))
	testutil.Equal(t, j["verdict"], "not_assessed")
	testutil.Contains(t, stringsOf(j["observations"])[0], "proposal")
}

func TestLaterProposalDoesNotFillEarlierUncommittedRound(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	testutil.Must(round(r.ctx, r.artifacts, nil))
	r.writeDraft()
	r.round(judgment())
	testutil.Equal(t, r.finish()["outcome"], "question")
	judgments := testutil.Must(record.List(r.ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 2)
	testutil.Equal(t, obj(judgments[0]["target"])["proposal_version"], any(nil))
	testutil.Equal(t, obj(judgments[1]["target"])["proposal_version"], any(float64(1)))
}

func TestFinishRejectsMissingOrInvalidRoundProposalVersion(t *testing.T) {
	for _, version := range []any{"missing", 0, "1", 1.5} {
		r := setup(t, nil)
		r.begin()
		r.writeDraft()
		r.round(judgment())
		entry := testutil.Must(readJSON(file(r.artifacts, "rounds/1.json")))
		if version == "missing" {
			delete(entry, "proposal_version")
		} else {
			entry["proposal_version"] = version
		}
		r.artifact("rounds/1.json", entry)
		_, err := finish(r.ctx, r.artifacts)
		testutil.Fails(t, err, file(r.artifacts, "rounds/1.json"))
		testutil.Fails(t, err, "proposal_version")
		testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "judgment")), []record.Document{})
		testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "result")), []record.Document{})
	}
}
