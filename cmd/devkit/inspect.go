package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/source"
	"github.com/ildjenty/dev-kit/internal/view"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

func workValidator(ctx context.Context, w *workspace.Workspace, workRoot string) (*ledger.Validator, error) {
	raw, err := fsstore.New(workRoot).List()
	if err != nil {
		return nil, err
	}
	for _, record := range raw {
		if record.Kind != string(ledger.KindWork) {
			continue
		}
		var work ledger.Work
		if err := fsstore.DecodePayload(record.Payload, &work); err != nil {
			return nil, err
		}
		if work.KitVersion != kit.Version {
			return nil, errors.New("work and binary kit versions do not match")
		}
		sources := make(map[string][]byte)
		for _, name := range []string{"decision", "evidence", "progress", "question", "judgment", "finding", "reference", "instruction", "plan-template", "plan"} {
			sources[name], err = kit.Schema(name)
			if err != nil {
				return nil, err
			}
		}
		return ledger.NewValidator(sources)
	}
	return nil, errors.New("work record is missing")
}

func inspect(ctx context.Context, e *engine.Engine, name string, f *flag.FlagSet, args []string, output io.Writer) error {
	all, recordID := false, ""
	if name == "status" {
		f.BoolVar(&all, "records", false, "Query all records")
		f.StringVar(&recordID, "record", "", "Query one record ID")
	}
	if err := parse(f, args, 0, 0); err != nil {
		return err
	}
	if all && recordID != "" {
		return errors.New("status accepts either --records or --record")
	}
	status, err := e.Status(ctx)
	if err != nil {
		return err
	}
	root := filepath.Join(e.Root(), ".devkit/run/work", status.WorkID)
	if name == "status" && !all && recordID == "" {
		return writeOutput(output, root, "status.json", status)
	}
	w, err := workspace.Open(ctx, e.Root())
	if err != nil {
		return err
	}
	validator, err := workValidator(ctx, w, root)
	if err != nil {
		return err
	}
	store := fsstore.New(root)
	if recordID != "" {
		record, err := ledger.Get(store, recordID, validator)
		if err != nil {
			return err
		}
		return writeOutput(output, root, "record.json", record)
	}
	records, err := ledger.List(store, validator)
	if err != nil {
		return err
	}
	if all {
		return writeOutput(output, root, "records.json", records)
	}
	stops, err := hook.ReadStops(root)
	if err != nil {
		return err
	}
	state, err := ledger.Load(store, stops, validator)
	if err != nil {
		return err
	}
	decisions, err := state.ResolveDecisions(ledger.DecisionContext{})
	if err != nil {
		return err
	}
	tree, err := w.SnapshotTree(ctx)
	if err != nil {
		return err
	}
	current, norms, err := e.CurrentIdentity(ctx, tree)
	if err != nil {
		return err
	}
	targets := []form.TargetVersion{current}
	if status.Position.Plan != nil {
		targets = append(targets, form.TargetVersion{Kind: form.TargetPlan, Plan: status.Position.Plan.Plan.Version})
	}
	structure, text, revisions, err := viewStructures(ctx, w, e.Root(), tree)
	if err != nil {
		return err
	}
	if structure != nil {
		targets = append(targets, form.TargetVersion{Kind: form.TargetStructure, Structure: structure.Version})
	}
	html, err := view.Generate(view.Input{Ledger: state, Records: records, Decisions: decisions, Targets: targets, Norms: norms, Structure: structure, StructureText: text, StructureHistory: revisions})
	if err != nil {
		return err
	}
	if err := store.Publish("view.html", []byte(html)); err != nil {
		return err
	}
	if status.Warning != "" {
		if _, err := fmt.Fprintln(output, "Warning:", status.Warning); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(output, filepath.Join(root, "view.html"))
	return err
}

// parseStructure returns a drawable structure, or nil when the body does not follow the structure JSON.
func parseStructure(data []byte, version string) *view.Structure {
	schema, err := kit.Schema("structure")
	if err != nil {
		return nil
	}
	validator, err := form.CompileSchema(schema)
	if err != nil {
		return nil
	}
	var structure view.Structure
	if err := validator.Decode(data, &structure); err != nil {
		return nil
	}
	structure.Version = version
	if len(structure.Modules) == 0 || view.StructureMermaid(structure) == "" {
		return nil
	}
	return &structure
}

// viewStructures reads the structure source from the working checkout or its outside location.
// A body that is not a drawable structure JSON is shown as text.
func viewStructures(ctx context.Context, w *workspace.Workspace, root, tree string) (*view.Structure, *view.StructureText, []view.StructureRevision, error) {
	data, err := w.ReadFile(ctx, tree, "devkit.toml")
	if err != nil {
		return nil, nil, nil, err
	}
	settings, err := config.Parse(data)
	if err != nil {
		return nil, nil, nil, err
	}
	item := settings.KnowledgeSources()["structure"]
	adapter, err := source.NewFiles(root, item.Path, os.DirFS(root), "")
	if err != nil {
		return nil, nil, nil, err
	}
	documents, err := adapter.Select("")
	if err != nil {
		return nil, nil, nil, err
	}
	structure, text := currentStructure(documents)
	location, err := filepath.Rel(root, filepath.Join(root, filepath.FromSlash(item.Path)))
	if filepath.IsAbs(filepath.FromSlash(item.Path)) {
		location, err = filepath.Rel(root, filepath.FromSlash(item.Path))
	}
	if err != nil || !filepath.IsLocal(location) {
		return structure, text, nil, nil
	}
	history, err := w.StructureHistory(ctx, "HEAD", filepath.ToSlash(location))
	if err != nil {
		return nil, nil, nil, err
	}
	var revisions []view.StructureRevision
	for _, entry := range history {
		revision := view.StructureRevision{Commit: entry.Commit}
		if !entry.Deleted {
			revision.Structure, revision.Text = currentStructure([]source.Document{{Body: string(entry.Content)}})
		}
		revisions = append(revisions, revision)
	}
	return structure, text, revisions, nil
}

func currentStructure(documents []source.Document) (*view.Structure, *view.StructureText) {
	if len(documents) == 0 {
		return nil, nil
	}
	var body strings.Builder
	for _, document := range documents {
		if len(documents) > 1 {
			fmt.Fprintf(&body, "## %s\n\n", document.ID)
		}
		body.WriteString(document.Body)
	}
	version := fmt.Sprintf("%x", sha256.Sum256([]byte(body.String())))
	if len(documents) == 1 {
		if structure := parseStructure([]byte(documents[0].Body), version); structure != nil {
			return structure, nil
		}
	}
	return nil, &view.StructureText{Version: version, Body: body.String()}
}
