package model

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// LoadKnowledge reads and parses an operator knowledge YAML file from the
// given path, returning the populated OperatorKnowledge struct.
func LoadKnowledge(path string) (*OperatorKnowledge, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading knowledge file %s: %w", path, err)
	}

	var k OperatorKnowledge
	if err := yaml.Unmarshal(data, &k); err != nil {
		return nil, fmt.Errorf("parsing knowledge file %s: %w", path, err)
	}

	return &k, nil
}
