package otlp

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/opsramp/stanza/entry"
	"go.opentelemetry.io/collector/model/otlpgrpc"
	"go.opentelemetry.io/collector/model/pdata"
)

func buildProtoRequest(entries []*entry.Entry) otlpgrpc.LogsRequest {

	// partitioning entries based on their resource labels
	// TODO: add the partitioning logic before it reaches the OTEL output handler
	partitionedEntries := map[string][]*entry.Entry{}
	for _, value := range entries {
		// building a unique key based on resource label values
		keys := []string{}
		for labelKey := range value.Resource {
			keys = append(keys, labelKey)
		}
		sort.Strings(keys)
		uniqueKey := ""
		for _, key := range keys {
			uniqueKey = fmt.Sprintf("%s|%s:%s", uniqueKey, key, value.Resource[key])
		}

		if entryList, ok := partitionedEntries[uniqueKey]; ok {
			partitionedEntries[uniqueKey] = append(entryList, value)
		} else {
			partitionedEntries[uniqueKey] = []*entry.Entry{value}
		}
	}

	logRequest := otlpgrpc.NewLogsRequest()
	pLogs := pdata.NewLogs()
	rl := pLogs.ResourceLogs()

	for _, entryList := range partitionedEntries {
		resourceLog := rl.AppendEmpty()
		ill := resourceLog.InstrumentationLibraryLogs().AppendEmpty()

		for index, entry := range entryList {
			// assigning the resource labels during the first iteration
			// (will only be done once since all the records have same resource labels)
			if index == 0 {
				for key, value := range entry.Resource {
					resourceLog.Resource().Attributes().InsertString(key, value)
				}
			}

			logRec := ill.LogRecords().AppendEmpty()
			convertEntryToLogRecord(entry, logRec)
		}
	}

	logRequest.SetLogs(pLogs)
	return logRequest

}

// Convert converts entry.Entry into provided pdata.LogRecord.
func convertEntryToLogRecord(entry *entry.Entry, dest pdata.LogRecord) {
	dest.SetTimestamp(pdata.NewTimestampFromTime(entry.Timestamp))
	dest.SetSeverityNumber(sevMap[entry.Severity])
	dest.SetSeverityText(sevTextMap[entry.Severity])

	insertToAttributeVal(entry.Record, entry.Resource, entry.Labels, dest.Body())
	for key, value := range entry.Labels {
		dest.Attributes().InsertString(key, value)
	}
}

func insertToAttributeVal(record interface{}, resourceLabels, logLabels map[string]string, dest pdata.AttributeValue) {
	bodyJSON := map[string]interface{}{}

	// adding resourceLabels and logLabels to Body
	for key, value := range resourceLabels {
		bodyJSON[key] = value
	}
	for key, value := range logLabels {
		bodyJSON[key] = value
	}

	switch t := record.(type) {
	case []byte:
		bodyJSON["message"] = string(t)
	case map[string]interface{}:
		b, _ := json.Marshal(t)
		bodyJSON["message"] = string(b)
	default:
		bodyJSON["message"] = fmt.Sprintf("%v", t)
	}

	b, _ := json.Marshal(bodyJSON)
	dest.SetStringVal(string(b))
}

var sevMap = map[entry.Severity]pdata.SeverityNumber{
	entry.Default: pdata.SeverityNumberUNDEFINED,
	entry.Trace:   pdata.SeverityNumberTRACE,
	entry.Trace2:  pdata.SeverityNumberTRACE2,
	entry.Trace3:  pdata.SeverityNumberTRACE3,
	entry.Trace4:  pdata.SeverityNumberTRACE4,
	entry.Debug:   pdata.SeverityNumberDEBUG,
	entry.Debug2:  pdata.SeverityNumberDEBUG2,
	entry.Debug3:  pdata.SeverityNumberDEBUG3,
	entry.Debug4:  pdata.SeverityNumberDEBUG4,
	entry.Info:    pdata.SeverityNumberINFO,
	entry.Info2:   pdata.SeverityNumberINFO2,
	entry.Info3:   pdata.SeverityNumberINFO3,
	entry.Info4:   pdata.SeverityNumberINFO4,
	entry.Error2:  pdata.SeverityNumberERROR2,
	entry.Error3:  pdata.SeverityNumberERROR3,
	entry.Error4:  pdata.SeverityNumberERROR4,
}

var sevTextMap = map[entry.Severity]string{
	entry.Default: "",
	entry.Trace:   "Trace",
	entry.Trace2:  "Trace2",
	entry.Trace3:  "Trace3",
	entry.Trace4:  "Trace4",
	entry.Debug:   "Debug",
	entry.Debug2:  "Debug2",
	entry.Debug3:  "Debug3",
	entry.Debug4:  "Debug4",
	entry.Info:    "Info",
	entry.Info2:   "Info2",
	entry.Info3:   "Info3",
	entry.Info4:   "Info4",
	entry.Error:   "Error",
	entry.Error2:  "Error2",
	entry.Error3:  "Error3",
	entry.Error4:  "Error4",
}
