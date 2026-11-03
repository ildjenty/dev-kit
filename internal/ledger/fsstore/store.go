package fsstore

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/oklog/ulid/v2"
)

// Store は呼び出し元から渡されたルート配下のファイルを扱う。
// 記録とロックには作業ルートを渡す。セッションの生成物には実行ルートを渡せる。
type Store struct {
	root string
}

// Record はファイル名から得た ID と、ledger が解釈する JSON 本文を持つ。
type Record struct {
	ID   string
	Data json.RawMessage
}

// New は保存先を設定する。ルートの解決やファイルの作成は行わない。
func New(root string) *Store {
	return &Store{root: root}
}

// Append は records/<ID>.json を追記する。既存の記録は上書きしない。
// 作業ロックは取得せず、通常の操作での取得は呼び出し元に委ねる。
func (s *Store) Append(id string, data json.RawMessage) error {
	path, err := s.recordPath(id)
	if err != nil {
		return err
	}
	if err := validateJSON(path, data); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// 同じ ID の書き手も、一時ファイルの排他的な作成で直列化する。
	temp := filepath.Join(filepath.Dir(path), "."+id+".tmp")
	f, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			os.Remove(temp)
		}
	}()
	if err := writeAndClose(f, data); err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		return &fs.PathError{Op: "append", Path: path, Err: fs.ErrExist}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		return err
	}
	published = true
	return nil
}

// List は記録を ID 順で全件取得する。一時ファイルは読まない。
// 壊れた記録があれば、そのファイルを残したままエラーを返す。
func (s *Store) List() ([]Record, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "records"))
	if os.IsNotExist(err) {
		return []Record{}, nil
	}
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		data, err := s.Get(id)
		if err != nil {
			return nil, err
		}
		records = append(records, Record{ID: id, Data: data})
	}
	return records, nil
}

// Get は記録を取得し、JSON の破損をファイル名と理由付きで報告する。
// スキーマと kit の互換性の照合は、本文を解釈する呼び出し元が行う。
func (s *Store) Get(id string) (json.RawMessage, error) {
	path, err := s.recordPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := validateJSON(path, data); err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// Publish はルートからの相対パスに生成物を一時ファイルから rename で公開する。
// 追記する観測や結果の名前の一意性は呼び出し元が決める。
// 再生成できる view.html だけは通常の書き込みで保存する。
func (s *Store) Publish(name string, data []byte) error {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("生成物のパスはルート配下の相対パスが必要です: %q", name)
	}
	path := filepath.Join(s.root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if filepath.Clean(name) == "view.html" {
		return os.WriteFile(path, data, 0o600)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".publish-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := writeAndClose(f, data); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Lock は run.lock のプロセス間排他ロックを取得するまで待つ。
// 呼び出し元は処理の終了時に Close する。ロックファイルは削除しない。
func (s *Store) Lock() (*Lock, error) {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(s.root, "run.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("作業ロック %s: %w", f.Name(), err)
	}
	return &Lock{file: f}, nil
}

// Lock は作業ロックの生存期間を表す。
type Lock struct {
	file *os.File
}

// Close はファイルを閉じて作業ロックを解放する。
func (l *Lock) Close() error {
	return l.file.Close()
}

func (s *Store) recordPath(id string) (string, error) {
	path := filepath.Join(s.root, "records", id+".json")
	if _, err := ulid.ParseStrict(id); err != nil {
		return "", fmt.Errorf("記録 %s: 記録 ID が ULID ではありません: %w", path, err)
	}
	return path, nil
}

func validateJSON(path string, data []byte) error {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("記録 %s: JSON を読めません: %w", path, err)
	}
	return nil
}

func writeAndClose(f *os.File, data []byte) error {
	_, err := f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
