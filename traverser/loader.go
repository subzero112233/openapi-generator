package traverser

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

func LoadYAML(path string) (doc Map, err error) {
	file, err := os.Open(path)
	if err != nil {
		return doc, fmt.Errorf("failed opening %q: %w", path, err)
	}

	defer file.Close() // nolint: errcheck

	err = yaml.NewDecoder(file).Decode(&doc)
	if err != nil {
		return doc, fmt.Errorf("failed parsing %q: %w", path, err)
	}

	return doc, nil
}

func LoadJSON(path string) (doc Map, err error) {
	file, err := os.Open(path)
	if err != nil {
		return doc, fmt.Errorf("failed opening %q: %w", path, err)
	}

	defer file.Close() // nolint: errcheck

	err = json.NewDecoder(file).Decode(&doc)
	if err != nil {
		return doc, fmt.Errorf("failed parsing %q: %w", path, err)
	}

	return doc, nil
}
