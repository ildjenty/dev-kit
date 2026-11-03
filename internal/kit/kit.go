package kit

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	devkit "github.com/ildjenty/dev-kit"
)

// Version はバイナリに埋め込んだ固定物のバージョンである。
const Version = "v1"

// Kit はリポジトリ内にある単一バージョンの固定物のコピーを表す。
// fs.FS としても利用でき、埋め込みへの読み出しの切り替えは行わない。
type Kit struct {
	dir     string
	version string
}

// Open は指定したバージョンのコピーを開く。読み取り可能なのは Version のみである。
// Work がある場合は、読み出しに先立って CheckVersion で照合する。
func Open(repoRoot, version string) (*Kit, error) {
	if version != Version {
		return nil, fmt.Errorf("kit: 読み取り可能なバージョンは %q、指定は %q", Version, version)
	}
	dir := filepath.Join(repoRoot, ".devkit", "kit", version)
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("kit: コピーを開く: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("kit: コピーがディレクトリではない: %s", dir)
	}
	return &Kit{dir: dir, version: version}, nil
}

// Dir は固定物のコピーのディレクトリを返す。
func (k *Kit) Dir() string {
	return k.dir
}

// Version は開いたコピーのバージョンを返す。
func (k *Kit) Version() string {
	return k.version
}

// CheckVersion は Work に記録したバージョンとコピーの一致を照合する。
func (k *Kit) CheckVersion(workVersion string) error {
	if workVersion != k.version {
		return fmt.Errorf("kit: Work のバージョン %q とコピーのバージョン %q が一致しない", workVersion, k.version)
	}
	return nil
}

// Open はコピー内の相対パスを開く。パスは fs.FS の形式に従う。
func (k *Kit) Open(name string) (fs.File, error) {
	return os.DirFS(k.dir).Open(name)
}

// ReadFile はコピー内の相対パスから固定物を読み出す。
func (k *Kit) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(os.DirFS(k.dir), name)
}

// Copy は init 用に埋め込みを .devkit/kit/<Version>/ にコピーする。
// 同内容の既存ファイルは維持し、内容が異なるファイルは上書きせずエラーにする。
func Copy(repoRoot string) error {
	dir := filepath.Join(repoRoot, ".devkit", "kit", Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("kit: コピー先を作る: %w", err)
	}
	return walkEmbedded(func(source, name string, entry fs.DirEntry) error {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("kit: コピー先を作る: %w", err)
			}
			return nil
		}
		data, err := fs.ReadFile(devkit.FS(), source)
		if err != nil {
			return fmt.Errorf("kit: 埋め込みを読む: %w", err)
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			existing, err := os.ReadFile(target)
			if err != nil {
				return fmt.Errorf("kit: 既存のコピーを読む: %w", err)
			}
			if !bytes.Equal(existing, data) {
				return fmt.Errorf("kit: 既存のコピーの内容が異なる: %s", target)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("kit: コピー先を開く: %w", err)
		}
		_, writeErr := file.Write(data)
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return fmt.Errorf("kit: コピーを書き出す %s: %w", target, err)
		}
		return nil
	})
}

// Verify は doctor 用にコピーと埋め込みの配置・内容の一致を照合する。
// 欠落や余分な項目、種類や内容の違いはエラーにし、コピーは修正しない。
func (k *Kit) Verify() error {
	expected := make(map[string]fs.DirEntry)
	if err := walkEmbedded(func(source, name string, entry fs.DirEntry) error {
		expected[name] = entry
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(devkit.FS(), source)
		if err != nil {
			return fmt.Errorf("kit: 埋め込みを読む: %w", err)
		}
		copied, err := k.ReadFile(name)
		if err != nil {
			return fmt.Errorf("kit: コピーを読む: %w", err)
		}
		if !bytes.Equal(copied, data) {
			return fmt.Errorf("kit: コピーと埋め込みの内容が異なる: %s", name)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := fs.WalkDir(k, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("kit: コピーをたどる: %w", err)
		}
		if name == "." {
			return nil
		}
		original, ok := expected[name]
		if !ok {
			return fmt.Errorf("kit: 埋め込みにない項目がコピーにある: %s", name)
		}
		if entry.Type() != original.Type() {
			return fmt.Errorf("kit: コピーと埋め込みの項目の種類が異なる: %s", name)
		}
		delete(expected, name)
		return nil
	}); err != nil {
		return err
	}
	for name := range expected {
		return fmt.Errorf("kit: コピーに項目がない: %s", name)
	}
	return nil
}

func walkEmbedded(visit func(source, name string, entry fs.DirEntry) error) error {
	return fs.WalkDir(devkit.FS(), ".", func(source string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("kit: 埋め込みをたどる: %w", err)
		}
		if source == "." || source == "kit" {
			return nil
		}
		return visit(source, strings.TrimPrefix(source, "kit/"), entry)
	})
}
