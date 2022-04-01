package otlp

import (
	"github.com/observiq/stanza/entry"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func Test_buildProtoRequest(t *testing.T) {
	logRequest := buildProtoRequest(generateTestEntries())
	require.Equal(t, logRequest.Logs().LogRecordCount(), 3)
	for i := 0; i < 3; i++ {
		lr := logRequest.Logs().ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).LogRecords().At(i)
		require.Equal(t, lr.Body().StringVal(), "test record"+strconv.Itoa(i))
	}
}

func generateTestEntries() []*entry.Entry {

	entries := make([]*entry.Entry, 0, 3)
	sev := []int{10, 12, 20}

	for i := 0; i < 3; i++ {
		ent := entry.New()
		ent.Severity = entry.Severity(sev[i])
		ent.SeverityText = entry.Severity(i).String()
		ent.Record = "test record" + strconv.Itoa(i)
		entries = append(entries, ent)
	}

	return entries
}
