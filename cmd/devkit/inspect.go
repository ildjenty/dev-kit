package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
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
	norms, err := engine.NormativeReferences(ctx, w, tree)
	if err != nil {
		return err
	}
	targets := []form.TargetVersion{{Kind: form.TargetTree, Tree: tree}}
	if status.Position.Plan != nil {
		targets = append(targets, form.TargetVersion{Kind: form.TargetPlan, Plan: status.Position.Plan.Plan.Version})
	}
	var structure *view.Structure
	source, err := w.ReadFile(ctx, tree, "devkit.toml")
	if err != nil {
		return err
	}
	settings, err := config.Parse(source)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(e.Root(), settings.RepositoryPaths().Structure))
	if err == nil {
		structure, err = parseStructure(data, fmt.Sprintf("%x", sha256.Sum256(data)))
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return err
	}
	if structure != nil {
		targets = append(targets, form.TargetVersion{Kind: form.TargetStructure, Structure: structure.Version})
	}
	history, err := w.StructureHistory(ctx, "HEAD", settings.RepositoryPaths().Structure)
	if err != nil {
		return err
	}
	var revisions []view.StructureRevision
	for _, item := range history {
		revision := view.StructureRevision{Commit: item.Commit}
		if !item.Deleted {
			revision.Structure, err = parseStructure(item.Content, fmt.Sprintf("%x", sha256.Sum256(item.Content)))
			if err != nil {
				return err
			}
		}
		revisions = append(revisions, revision)
	}
	html, err := view.Generate(view.Input{Ledger: state, Records: records, Decisions: decisions, Targets: targets, Norms: norms, Structure: structure, StructureHistory: revisions})
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

func parseStructure(data []byte, version string) (*view.Structure, error) {
	var structure view.Structure
	if err := json.Unmarshal(data, &structure); err != nil {
		return nil, err
	}
	structure.Version = version
	return &structure, nil
}
