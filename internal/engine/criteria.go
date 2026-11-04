package engine

import (
	"context"
	"fmt"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/source"
)

// criteria resolves a check's criteria selections from the norms fixed for ref. Only norm
// sources supply criteria, and a selection that matches no documents stops the judgment.
func (e *Engine) criteria(ctx context.Context, ref string, selections []plan.Selection) ([]check.Criterion, error) {
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return nil, err
	}
	resolved := []check.CriteriaSelection{}
	for _, selection := range selections {
		item, ok := sources[selection.Source]
		if !ok {
			return nil, fmt.Errorf("unknown criteria source: %q", selection.Source)
		}
		if !item.Norm {
			return nil, fmt.Errorf("criteria source %q must be a norm source", selection.Source)
		}
		documents, err := e.selectDocuments(ctx, ref, selection.Source, selection.Select)
		if err != nil {
			return nil, err
		}
		entry := check.CriteriaSelection{Source: selection.Source, Select: selection.Select}
		for _, document := range documents {
			entry.Documents = append(entry.Documents, check.Criterion{Source: selection.Source, ID: document.ID, ContentHash: source.ContentHash(document.Body), Content: document.Body})
		}
		resolved = append(resolved, entry)
	}
	return check.ResolveCriteria(resolved)
}

// criteriaDocuments returns the fixed bodies of criteria recorded in a request.
func (e *Engine) criteriaDocuments(ctx context.Context, ref string, criteria []check.Criterion) ([]brief.Document, error) {
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return nil, err
	}
	docs := []brief.Document{}
	for _, criterion := range criteria {
		body, err := e.criterionBody(ctx, ref, criterion, sources[criterion.Source])
		if err != nil {
			return nil, err
		}
		docs = append(docs, brief.Document{Path: criterion.Source + "/" + criterion.ID, Content: body, Exists: true})
	}
	return docs, nil
}

func (e *Engine) criterionBody(ctx context.Context, ref string, criterion check.Criterion, item config.Source) (string, error) {
	if _, inside := e.repositoryLocation(item); !inside && e.workRoot() != "" {
		if body, err := e.blob(criterion.ContentHash); err == nil {
			return body, nil
		}
	}
	document, err := e.document(ctx, ref, criterion.Source, criterion.ID)
	if err != nil {
		return "", err
	}
	if source.ContentHash(document.Body) != criterion.ContentHash {
		return "", fmt.Errorf("criteria document %s/%s changed after the request was issued", criterion.Source, criterion.ID)
	}
	return document.Body, nil
}

func planSelections(selections []config.Selection) []plan.Selection {
	result := []plan.Selection{}
	for _, selection := range selections {
		result = append(result, plan.Selection{Source: selection.Source, Select: selection.Select})
	}
	return result
}

// inputSchemas supplies the input contracts selected by a check's schemas; output schemas are
// already bundled with their references.
func (e *Engine) inputSchemas(ctx context.Context, ref string, names []string) ([]brief.Document, error) {
	documents := []brief.Document{}
	for _, name := range names {
		body, err := e.asset(ctx, ref, "schemas/"+name+".json")
		if err != nil {
			return nil, err
		}
		documents = append(documents, brief.Document{Path: "schemas/" + name + ".json", Content: string(body), Exists: true})
	}
	return documents, nil
}
