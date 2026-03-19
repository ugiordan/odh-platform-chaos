package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const maxModelFileSize = 1 * 1024 * 1024 // 1 MB

// LoadKnowledge reads and parses an operator knowledge YAML file from the
// given path, returning the populated OperatorKnowledge struct.
func LoadKnowledge(path string) (*OperatorKnowledge, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Size() > maxModelFileSize {
		return nil, fmt.Errorf("file %s (%d bytes) exceeds maximum size of %d bytes", path, info.Size(), maxModelFileSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading knowledge file %s: %w", path, err)
	}

	var k OperatorKnowledge
	if err := yaml.UnmarshalStrict(data, &k); err != nil {
		return nil, fmt.Errorf("parsing knowledge file %s: %w", path, err)
	}

	return &k, nil
}

// LoadKnowledgeDir reads all YAML files from the specified directory and
// returns a slice of OperatorKnowledge structs. Non-YAML files and subdirectories
// are skipped. Returns error if any file fails to load.
func LoadKnowledgeDir(dir string) ([]*OperatorKnowledge, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading knowledge dir %s: %w", dir, err)
	}

	var models []*OperatorKnowledge
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		k, err := LoadKnowledge(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", name, err)
		}
		models = append(models, k)
	}
	return models, nil
}
