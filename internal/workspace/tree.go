package workspace

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"testing/fstest"
)

// TreeFS returns a read-only file system rooted at the repository root holding the
// regular files under paths in the fixed reference. Missing paths are omitted.
func (w *Workspace) TreeFS(ctx context.Context, ref string, paths ...string) (fs.FS, error) {
	args := append([]string{"ls-tree", "-r", "-z", ref, "--"}, paths...)
	output, err := w.run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	names, blobs := []string{}, []string{}
	for _, entry := range bytes.Split(output, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		metadata, name, ok := strings.Cut(string(entry), "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid git ls-tree output: %q", entry)
		}
		if fields[1] == "blob" && (fields[0] == "100644" || fields[0] == "100755") {
			names, blobs = append(names, name), append(blobs, fields[2])
		}
	}
	contents, err := w.readBlobs(ctx, blobs)
	if err != nil {
		return nil, err
	}
	files := fstest.MapFS{}
	for i, name := range names {
		files[name] = &fstest.MapFile{Data: contents[i], Mode: 0o444}
	}
	return files, nil
}

func (w *Workspace) readBlobs(ctx context.Context, blobs []string) ([][]byte, error) {
	if len(blobs) == 0 {
		return nil, nil
	}
	cmd := w.command(ctx, nil, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(blobs, "\n") + "\n")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w", err)
	}
	reader := bufio.NewReader(bytes.NewReader(output))
	contents := make([][]byte, 0, len(blobs))
	for range blobs {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid git cat-file output: %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, err
		}
		data := make([]byte, size+1)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		contents = append(contents, data[:size])
	}
	return contents, nil
}
