package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Workspace は1つのチェックアウトで git CLI を実行する。
type Workspace struct {
	root string
}

// Snapshot はブランチに含めないコミットと、そのツリーで表す対象のバージョンである。
type Snapshot struct {
	Tree   string
	Commit string
}

// CommitResult は照合したツリーと、実際のコミットおよび hook の出力を保持する。
// Commit が空でなければ、エラーがあってもコミットは作成済みである。
type CommitResult struct {
	BeforeTree string
	Commit     string
	Tree       string
	Output     string
}

// ErrTargetChanged は commit 前後に判定済みのツリーとの不一致を検出したことを表す。
var ErrTargetChanged = errors.New("判定済みの対象とツリーが一致しません")

// StructureRevision は structure.json の履歴を新しいコミットから順に表す。
type StructureRevision struct {
	Commit  string
	Blob    string
	Content []byte
	Deleted bool
}

// Open は cwd が属するチェックアウトのルートを解決する。
func Open(ctx context.Context, cwd string) (*Workspace, error) {
	w := &Workspace{root: cwd}
	root, err := w.run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	w.root = strings.TrimSuffix(string(root), "\n")
	return w, nil
}

// Root はチェックアウトのルートの絶対パスを返す。
func (w *Workspace) Root() string {
	return w.root
}

// Branch は現在のブランチ名を返す。detached HEAD では git のエラーを返す。
func (w *Workspace) Branch(ctx context.Context) (string, error) {
	return w.line(ctx, nil, "symbolic-ref", "--quiet", "--short", "HEAD")
}

// Head は現在の HEAD のコミット ID を返す。
func (w *Workspace) Head(ctx context.Context) (string, error) {
	return w.line(ctx, nil, "rev-parse", "--verify", "HEAD^{commit}")
}

// CreateBranch は現在の HEAD からブランチを作成して切り替える。
// 作業のブランチ名には devkit/<作業 ID> を渡す。
func (w *Workspace) CreateBranch(ctx context.Context, name string) error {
	_, err := w.run(ctx, nil, "switch", "-c", name)
	return err
}

// SwitchBranch は既存のブランチに切り替える。
func (w *Workspace) SwitchBranch(ctx context.Context, name string) error {
	_, err := w.run(ctx, nil, "switch", "--", name)
	return err
}

// SnapshotTree は追跡ファイルと ignore されていない未追跡ファイルのツリーを作る。
// 通常の index と HEAD は変更しない。
func (w *Workspace) SnapshotTree(ctx context.Context) (string, error) {
	index, cleanup, err := w.workingIndex(ctx)
	if err != nil {
		return "", err
	}
	defer cleanup()
	return w.line(ctx, index, "write-tree")
}

// Snapshot は作業ツリーの対象を取得し、refs/devkit/<作業 ID>/<ツリー ID> で保持する。
func (w *Workspace) Snapshot(ctx context.Context, workID string) (Snapshot, error) {
	tree, err := w.SnapshotTree(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	commit, err := w.line(ctx, nil, "commit-tree", tree, "-m", "devkit snapshot")
	if err != nil {
		return Snapshot{}, err
	}
	return w.SnapshotFromCommit(ctx, workID, commit)
}

// SnapshotFromCommit は既存のコミットを対象として保持する。hook が変更した commit にも使える。
func (w *Workspace) SnapshotFromCommit(ctx context.Context, workID, commit string) (Snapshot, error) {
	commitID, err := w.line(ctx, nil, "rev-parse", "--verify", commit+"^{commit}")
	if err != nil {
		return Snapshot{}, err
	}
	tree, err := w.CommitTree(ctx, commitID)
	if err != nil {
		return Snapshot{}, err
	}
	if _, err := w.run(ctx, nil, "update-ref", "refs/devkit/"+workID+"/"+tree, commitID); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Tree: tree, Commit: commitID}, nil
}

// DeleteSnapshots は完了・中止と成果物の永続化がそろった作業の refs を削除する。
func (w *Workspace) DeleteSnapshots(ctx context.Context, workID string) error {
	refs, err := w.run(ctx, nil, "for-each-ref", "--format=%(refname)", "refs/devkit/"+workID+"/")
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(string(refs)) {
		if _, err := w.run(ctx, nil, "update-ref", "-d", ref); err != nil {
			return err
		}
	}
	return nil
}

// CommitTree はコミットのツリー ID を返す。
func (w *Workspace) CommitTree(ctx context.Context, commit string) (string, error) {
	return w.line(ctx, nil, "rev-parse", "--verify", commit+"^{tree}")
}

// ReadFile は指定した規範の参照元から git show でファイルを読む。
func (w *Workspace) ReadFile(ctx context.Context, ref, path string) ([]byte, error) {
	return w.run(ctx, nil, "show", ref+":"+path)
}

// NormativeIDs は規範のルートごとのツリー ID と blob ID を返す。存在しないものは含めない。
func (w *Workspace) NormativeIDs(ctx context.Context, ref string) (map[string]string, error) {
	return w.treeEntries(ctx, ref, ".devkit", "devkit.toml", ".claude/agents", ".codex/agents", "AGENTS.md", "CLAUDE.md")
}

// Expand は対象を新しいディレクトリへ展開する。export-ignore と export-subst は適用しない。
// 展開先は対象のバージョンごとに一度だけ作成し、判定者には読み取り用として渡す。
func (w *Workspace) Expand(ctx context.Context, tree, destination string) error {
	destination, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	index, cleanup, err := w.emptyIndex()
	if err != nil {
		return err
	}
	defer cleanup()
	if _, err := w.run(ctx, index, "read-tree", tree); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		return err
	}
	_, err = w.run(ctx, index, "checkout-index", "--all", "--prefix="+destination+string(filepath.Separator))
	if err != nil {
		return errors.Join(err, os.RemoveAll(destination))
	}
	return nil
}

// Commit は直前の作業ツリーを照合し、その一時 index で hook を有効にして commit する。
// 成功後は通常の index を新しい HEAD に reset し、作業ツリーには触れない。
// 前後の不一致では ErrTargetChanged を返し、作成済みのコミットは取り消さない。
func (w *Workspace) Commit(ctx context.Context, expectedTree, workID, stage, message string) (CommitResult, error) {
	var result CommitResult
	index, cleanup, err := w.workingIndex(ctx)
	if err != nil {
		return result, err
	}
	defer cleanup()
	result.BeforeTree, err = w.line(ctx, index, "write-tree")
	if err != nil {
		return result, err
	}
	if result.BeforeTree != expectedTree {
		return result, ErrTargetChanged
	}
	env := append(index, "DEVKIT_STAGE="+stage, "DEVKIT_WORK="+workID)
	args := []string{"commit", "--allow-empty", "-m", message}
	output, err := w.command(ctx, env, args...).CombinedOutput()
	result.Output = string(output)
	if err != nil {
		return result, fmt.Errorf("git commit: %w: %s", err, output)
	}
	result.Commit, err = w.Head(ctx)
	if err != nil {
		return result, err
	}
	result.Tree, err = w.CommitTree(ctx, result.Commit)
	if err != nil {
		return result, err
	}
	_, err = w.run(ctx, nil, "reset", "--mixed", "HEAD")
	if result.Tree != expectedTree {
		err = errors.Join(ErrTargetChanged, err)
	}
	return result, err
}

// ChangedFiles は二つの対象間で変更されたパスを返す。改名は削除と追加として返す。
func (w *Workspace) ChangedFiles(ctx context.Context, before, after string) ([]string, error) {
	output, err := w.run(ctx, nil, "diff", "--name-only", "--no-renames", "-z", before, after, "--")
	if err != nil {
		return nil, err
	}
	if len(output) == 0 {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00"), nil
}

// StructureHistory は指定した参照から structure.json の追加・変更・削除の履歴を読む。
func (w *Workspace) StructureHistory(ctx context.Context, ref string) ([]StructureRevision, error) {
	const path = ".devkit/structure.json"
	output, err := w.run(ctx, nil, "log", "--format=%H", ref, "--", path)
	if err != nil {
		return nil, err
	}
	var history []StructureRevision
	for _, commit := range strings.Fields(string(output)) {
		entries, err := w.treeEntries(ctx, commit, path)
		if err != nil {
			return nil, err
		}
		revision := StructureRevision{Commit: commit, Blob: entries[path]}
		revision.Deleted = revision.Blob == ""
		if !revision.Deleted {
			revision.Content, err = w.ReadFile(ctx, commit, path)
			if err != nil {
				return nil, err
			}
		}
		history = append(history, revision)
	}
	return history, nil
}

func (w *Workspace) treeEntries(ctx context.Context, ref string, paths ...string) (map[string]string, error) {
	args := append([]string{"ls-tree", "-z", ref, "--"}, paths...)
	output, err := w.run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]string)
	for _, entry := range bytes.Split(output, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		metadata, path, ok := strings.Cut(string(entry), "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("git ls-tree の出力が不正です: %q", entry)
		}
		entries[path] = fields[2]
	}
	return entries, nil
}

func (w *Workspace) emptyIndex() ([]string, func(), error) {
	directory, err := os.MkdirTemp("", "devkit-index-")
	if err != nil {
		return nil, nil, err
	}
	return []string{"GIT_INDEX_FILE=" + filepath.Join(directory, "index")}, func() { _ = os.RemoveAll(directory) }, nil
}

func (w *Workspace) workingIndex(ctx context.Context) ([]string, func(), error) {
	env, cleanup, err := w.emptyIndex()
	if err != nil {
		return nil, nil, err
	}
	indexPath, err := w.line(ctx, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	data, err := os.ReadFile(indexPath)
	if err == nil {
		err = os.WriteFile(strings.TrimPrefix(env[0], "GIT_INDEX_FILE="), data, 0o600)
	} else if errors.Is(err, os.ErrNotExist) {
		_, err = w.run(ctx, env, "read-tree", "--empty")
	}
	if err == nil {
		_, err = w.run(ctx, env, "add", "--all", "--", ".")
	}
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return env, cleanup, nil
}

func (w *Workspace) command(ctx context.Context, env []string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = w.root
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func (w *Workspace) run(ctx context.Context, env []string, args ...string) ([]byte, error) {
	output, err := w.command(ctx, env, args...).Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return output, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, exitError.Stderr)
		}
		return output, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func (w *Workspace) line(ctx context.Context, env []string, args ...string) (string, error) {
	output, err := w.run(ctx, env, args...)
	return strings.TrimSuffix(string(output), "\n"), err
}
