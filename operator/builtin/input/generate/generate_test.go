package generate

import (
	"testing"

	"github.com/opsramp/stanza/entry"
	"github.com/opsramp/stanza/operator"
	"github.com/opsramp/stanza/testutil"
	"github.com/stretchr/testify/require"
)

func TestInputGenerate(t *testing.T) {
	cfg := NewGenerateInputConfig("test_operator_id")
	cfg.OutputIDs = []string{"fake"}
	cfg.Count = 5
	cfg.Entry = entry.Entry{
		Record: "test message",
	}

	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	op := ops[0]

	fake := testutil.NewFakeOutput(t)
	err = op.SetOutputs([]operator.Operator{fake})
	require.NoError(t, err)

	require.NoError(t, op.Start())
	defer op.Stop()

	for i := 0; i < 5; i++ {
		fake.ExpectRecord(t, "test message")
	}
}
