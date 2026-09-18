package syncer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/julienbreux/agy-sync/internal/syncer"
)

func TestAdaptWorkspaceURIs(t *testing.T) {
	tests := []struct {
		name     string
		uris     []string
		destHome string
		expected []string
	}{
		{
			name:     "empty input",
			uris:     nil,
			destHome: "/Users/bob",
			expected: nil,
		},
		{
			name:     "empty destHome",
			uris:     []string{"file:///Users/alice/project"},
			destHome: "",
			expected: []string{"file:///Users/alice/project"},
		},
		{
			name:     "macOS to macOS different user",
			uris:     []string{"file:///Users/alice/Projects/agy-sync"},
			destHome: "/Users/julienbreux",
			expected: []string{"file:///Users/julienbreux/Projects/agy-sync"},
		},
		{
			name:     "Linux to macOS",
			uris:     []string{"file:///home/developer/workspace/app"},
			destHome: "/Users/julienbreux",
			expected: []string{"file:///Users/julienbreux/workspace/app"},
		},
		{
			name:     "macOS to Linux",
			uris:     []string{"file:///Users/alice/go/src/app"},
			destHome: "/home/ubuntu",
			expected: []string{"file:///home/ubuntu/go/src/app"},
		},
		{
			name:     "destHome with trailing slash",
			uris:     []string{"file:///Users/alice/workspace"},
			destHome: "/Users/bob/",
			expected: []string{"file:///Users/bob/workspace"},
		},
		{
			name:     "bare user home directory URI",
			uris:     []string{"file:///Users/alice"},
			destHome: "/Users/bob",
			expected: []string{"file:///Users/bob"},
		},
		{
			name:     "non-file URI preserved",
			uris:     []string{"vscode-vfs://remote/project", "file:///Users/alice/project"},
			destHome: "/Users/bob",
			expected: []string{"vscode-vfs://remote/project", "file:///Users/bob/project"},
		},
		{
			name:     "system directory without user home preserved",
			uris:     []string{"file:///opt/project", "file:///var/tmp/app"},
			destHome: "/Users/bob",
			expected: []string{"file:///opt/project", "file:///var/tmp/app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.AdaptWorkspaceURIs(tt.uris, tt.destHome)
			assert.Equal(t, tt.expected, result)
		})
	}
}
