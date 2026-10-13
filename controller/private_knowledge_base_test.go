package controller

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivateKnowledgePathStaysInUsernameDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PRIVATE_KNOWLEDGE_BASE_PATH", root)
	path, relative, err := privateKnowledgePath("ao.fu", "network/switch.md", true)
	require.NoError(t, err)
	require.Equal(t, "network/switch.md", relative)
	require.Equal(t, filepath.Join(root, "users", "ao.fu", "network", "switch.md"), path)
}

func TestPrivateKnowledgePathRejectsTraversal(t *testing.T) {
	t.Setenv("PRIVATE_KNOWLEDGE_BASE_PATH", t.TempDir())
	_, _, err := privateKnowledgePath("ao.fu", "../admin/secret.md", false)
	require.Error(t, err)
}
