package plan

import (
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func jsLess(a, b string) bool {
	return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b))) < 0
}

func foldersOf(tracked []string) map[string]bool {
	folders := map[string]bool{".": true}
	for _, file := range tracked {
		for dir := path.Dir(file); dir != "."; dir = path.Dir(dir) {
			folders[dir] = true
		}
	}
	return folders
}
func pathBoundary(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || strings.ContainsRune("_./@-", r)
}
func named(text, candidate string) bool {
	for start := 0; start <= len(text); {
		n := strings.Index(text[start:], candidate)
		if n < 0 {
			return false
		}
		n += start
		end := n + len(candidate)
		left, right := true, true
		if n > 0 {
			r, _ := utf8.DecodeLastRuneInString(text[:n])
			left = !pathBoundary(r)
		}
		if end < len(text) {
			r, _ := utf8.DecodeRuneInString(text[end:])
			right = !pathBoundary(r)
		}
		if left && right {
			return true
		}
		start = n + 1
	}
	return false
}
func pathsIn(texts, tracked []string) []document {
	folders := foldersOf(tracked)
	candidates := slices.Clone(tracked)
	for folder := range folders {
		if folder != "." {
			candidates = append(candidates, folder)
		}
	}
	found := map[string]bool{}
	tokens := regexp.MustCompile("`([^`\\n]+)`|[\\p{L}\\p{N}_.@-]+(?:/[\\p{L}\\p{N}_.@\\[\\]-]+)+/?|[\\p{L}\\p{N}_@-]+\\.[A-Za-z][A-Za-z0-9_.-]*")
	for _, t := range texts {
		for _, candidate := range candidates {
			if named(t, candidate) {
				found[candidate] = true
			}
		}
		for _, m := range tokens.FindAllStringSubmatch(t, -1) {
			token := m[0]
			if m[1] != "" {
				token = m[1]
			}
			token = strings.TrimSuffix(strings.TrimPrefix(token, "./"), "/")
			token = strings.TrimRight(token, ".,;:")
			if token != "" && !strings.Contains(token, "://") && !strings.HasPrefix(token, "/") && !slices.Contains(strings.Split(token, "/"), "..") {
				found[token] = true
			}
		}
	}
	paths := []string{}
	for p := range found {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool { return jsLess(paths[i], paths[j]) })
	out := []document{}
	for _, p := range paths {
		kind := "missing"
		if slices.Contains(tracked, p) {
			kind = "file"
		} else if folders[p] {
			kind = "folder"
		}
		out = append(out, document{"path": p, "kind": kind})
	}
	return out
}
func factsOf(paths []document, tracked []string) document {
	folders := foldersOf(tracked)
	files := map[string]bool{}
	for _, f := range tracked {
		files[f] = true
	}
	children := func(dir string) []string {
		out := []string{}
		for p := range files {
			if path.Dir(p) == dir {
				out = append(out, p)
			}
		}
		for p := range folders {
			if p != "." && path.Dir(p) == dir {
				out = append(out, p)
			}
		}
		sort.Slice(out, func(i, j int) bool { return jsLess(out[i], out[j]) })
		return out
	}
	describe := func(p string) document {
		kind := "missing"
		if files[p] {
			kind = "file"
		} else if folders[p] {
			kind = "folder"
		}
		siblings := []string{}
		if p != "." {
			for _, child := range children(path.Dir(p)) {
				if child != p {
					siblings = append(siblings, child)
				}
			}
		}
		out := document{"exists": files[p] || folders[p], "kind": kind, "siblings": siblings}
		if folders[p] {
			tree := []document{}
			for _, child := range children(p) {
				row := document{"path": child, "kind": "file"}
				if folders[child] {
					row["kind"] = "folder"
					row["entries"] = children(child)
				}
				tree = append(tree, row)
			}
			out["tree"] = tree
		}
		return out
	}
	result := document{}
	for _, p := range paths {
		name := text(p["path"])
		d := describe(name)
		parent := describe(path.Dir(name))
		parent["path"] = path.Dir(name)
		d["parent"] = parent
		result[name] = d
	}
	return document{"description": "Facts from the tracked tree at the named commit, without interpretation.", "paths": result}
}
func requirementsTexts(input document) []string {
	r := object(input["requirements"])
	out := []string{text(r["goal"])}
	for _, c := range entries(r["completion_conditions"]) {
		out = append(out, text(c["text"]))
	}
	out = append(out, stringsOf(r["constraints"])...)
	for _, p := range entries(r["open_points"]) {
		out = append(out, text(p["text"]))
	}
	return out
}
func trackedAt(cwd, commit string) ([]string, error) {
	out, err := gitx.Run(cwd, "ls-tree", "-r", "--name-only", "-z", commit)
	if err != nil {
		return nil, err
	}
	return strings.FieldsFunc(out, func(r rune) bool { return r == 0 }), nil
}
func factsCommand(ctx record.ContextData, artifacts, display string) (document, error) {
	if display == "" {
		display = artifacts
	}
	input, err := read(filepath.Join(artifacts, "plan", "input.json"))
	if err != nil {
		return nil, err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return nil, err
	}
	commit, err := g.Head()
	if err != nil {
		return nil, err
	}
	tracked, err := trackedAt(ctx.Cwd, commit)
	if err != nil {
		return nil, err
	}
	paths := pathsIn(requirementsTexts(input), tracked)
	file := filepath.Join(artifacts, "plan", "facts-before.json")
	facts := factsOf(paths, tracked)
	facts["commit"] = commit
	if err = write(file, facts); err != nil {
		return nil, err
	}
	return document{"path": filepath.Join(display, "plan", "facts-before.json"), "paths": len(paths)}, nil
}
