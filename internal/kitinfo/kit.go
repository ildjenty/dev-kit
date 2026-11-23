package kitinfo

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/kit"
)

const Workflows = ".archon/workflows/dev"
const StampName = ".devkit-installed.json"
const StampPath = Workflows + "/" + StampName

type Stamp struct {
	DevkitVersion string `json:"devkit_version"`
	KitDigest     string `json:"kit_digest"`
}

func Digest(files fs.FS) (string, error) {
	hash := sha256.New()
	err := fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		for _, part := range [][]byte{[]byte(path), data} {
			if err := binary.Write(hash, binary.BigEndian, uint64(len(part))); err != nil {
				return err
			}
			if _, err := hash.Write(part); err != nil {
				return err
			}
		}
		return nil
	})
	return hex.EncodeToString(hash.Sum(nil)), err
}

func EmbeddedDigest() string {
	digest, err := Digest(kit.Files)
	if err != nil {
		panic(err)
	}
	return digest
}

func ReadStamp(folder string) (Stamp, error) {
	data, err := os.ReadFile(filepath.Join(folder, StampName))
	if err != nil {
		return Stamp{}, err
	}
	var stamp Stamp
	err = json.Unmarshal(data, &stamp)
	if err == nil && (stamp.DevkitVersion == "" || stamp.KitDigest == "") {
		err = fmt.Errorf("invalid install stamp: %s", filepath.Join(folder, StampName))
	}
	return stamp, err
}

var call = regexp.MustCompile(`\bdevkit[ \t]+([a-z][a-z0-9-]*)(?:[ \t]+([a-z][a-z0-9-]*))?`)

func Invocations(files fs.FS, root string, skills bool) ([]string, error) {
	calls := []string{}
	err := fs.WalkDir(files, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "fixtures" {
				return fs.SkipDir
			}
			return nil
		}
		if !skills && !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") || strings.Contains(path, "/commands/") && strings.HasSuffix(path, ".md")) {
			return nil
		}
		data, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		for _, match := range call.FindAllStringSubmatch(string(data), -1) {
			calls = append(calls, strings.TrimSpace(match[1]+" "+match[2]))
		}
		return nil
	})
	slices.Sort(calls)
	return slices.Compact(calls), err
}
