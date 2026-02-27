package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPatternTypes(t *testing.T) {
	patterns := []PatternType{
		PatternIgnoredError,
		PatternGoroutineLaunch,
		PatternNetworkCall,
		PatternDatabaseCall,
	}
	for _, p := range patterns {
		assert.NotEmpty(t, string(p))
	}
}
