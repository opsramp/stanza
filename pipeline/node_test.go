package pipeline

import (
	"testing"

	_ "github.com/opsramp/stanza/operator/builtin/input/generate"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/noop"
	"github.com/opsramp/stanza/testutil"
	"github.com/stretchr/testify/require"
)

func TestNodeDOTID(t *testing.T) {
	operator := testutil.NewMockOperator("test")
	operator.On("Outputs").Return(nil)
	node := createOperatorNode(operator)
	require.Equal(t, operator.ID(), node.DOTID())
}

func TestCreateNodeID(t *testing.T) {
	nodeID := createNodeID("test_id")
	require.Equal(t, int64(5795108767401590291), nodeID)
}
