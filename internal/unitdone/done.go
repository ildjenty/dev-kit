package unitdone

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed unit-done.json
var schema embed.FS

type Marker struct {
	Unit   string `json:"unit"`
	Branch string `json:"branch"`
}

func Parse(data []byte) (Marker, error) {
	var value, definition any
	if err := json.Unmarshal(data, &value); err != nil {
		return Marker{}, err
	}
	source, err := schema.ReadFile("unit-done.json")
	if err != nil {
		return Marker{}, err
	}
	if err = json.Unmarshal(source, &definition); err != nil {
		return Marker{}, err
	}
	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource("https://devkit.invalid/unit-done.json", definition); err != nil {
		return Marker{}, err
	}
	compiled, err := compiler.Compile("https://devkit.invalid/unit-done.json")
	if err != nil {
		return Marker{}, err
	}
	if err = compiled.Validate(value); err != nil {
		return Marker{}, err
	}
	var marker Marker
	err = json.Unmarshal(data, &marker)
	return marker, err
}

func Exists(cwd, folder, id string) (bool, error) {
	_, err := os.Lstat(filepath.Join(cwd, folder, id+".done.json"))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func RequestUnit(dir string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "request"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	numbers := []int{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			n, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
			if err == nil {
				numbers = append(numbers, n)
			}
		}
	}
	sort.Ints(numbers)
	unit := ""
	for _, n := range numbers {
		data, err := os.ReadFile(filepath.Join(dir, "request", strconv.Itoa(n)+".json"))
		if err != nil {
			return "", err
		}
		var request struct {
			Unit string `json:"unit"`
		}
		if err = json.Unmarshal(data, &request); err != nil {
			return "", err
		}
		if request.Unit != "" {
			unit = request.Unit
		}
	}
	return unit, nil
}
