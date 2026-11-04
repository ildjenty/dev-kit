package engine

import (
	"context"
	"encoding/json"

	"github.com/ildjenty/dev-kit/internal/form"
)

// sourceIdentity identifies fixed outside product documents.
func (e *Engine) sourceIdentity() (string, error) {
	state, err := e.load()
	if err != nil {
		return "", err
	}
	ref, err := e.reference(context.Background(), state)
	if err != nil {
		return "", err
	}
	return e.sourceIdentityAt(context.Background(), ref)
}

func (e *Engine) sourceIdentityAt(ctx context.Context, ref string) (string, error) {
	external, err := e.externalSources(ctx, ref)
	if err != nil || len(external) == 0 {
		return "", err
	}
	pin, err := e.pin(ctx, ref, false)
	if err != nil {
		return "", err
	}
	manifests := sourcePin{}
	for name, item := range external {
		if !item.Norm && len(pin[name]) > 0 {
			manifests[name] = pin[name]
		}
	}
	if len(manifests) == 0 {
		return "", nil
	}
	data, err := json.Marshal(manifests)
	return hash(data), err
}

// treeTarget identifies a git tree together with fixed outside product documents.
func (e *Engine) treeTarget(tree string) (form.TargetVersion, error) {
	sources, err := e.sourceIdentity()
	return form.TargetVersion{Kind: form.TargetTree, Tree: tree, Sources: sources}, err
}

// approvalTree returns the approval target for a tree target version.
func approvalTree(target form.TargetVersion) form.ApprovalTarget {
	return form.ApprovalTarget{Kind: form.ApprovalTree, Tree: target.Tree, Sources: target.Sources}
}

// CurrentIdentity returns the tree target and its fixed outside products and norms.
func (e *Engine) CurrentIdentity(ctx context.Context, tree string) (form.TargetVersion, []form.NormReference, error) {
	ref, err := e.refreshSources(ctx, tree)
	if err != nil {
		return form.TargetVersion{}, nil, err
	}
	sources, err := e.sourceIdentityAt(ctx, ref)
	target := form.TargetVersion{Kind: form.TargetTree, Tree: tree, Sources: sources}
	if err != nil {
		return form.TargetVersion{}, nil, err
	}
	norms, err := e.norms(ctx, ref)
	return target, norms, err
}
