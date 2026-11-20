package docs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecIsValidJSON(t *testing.T) {
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(Spec()), &m))
	paths := m["paths"].(map[string]interface{})
	assert.Contains(t, paths, "/api/posts/ai-generate")
	assert.Contains(t, paths, "/api/admin/users")
	assert.Equal(t, Spec(), spec{}.ReadDoc())
}
