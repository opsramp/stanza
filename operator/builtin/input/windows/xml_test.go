// +build windows

package windows

import (
	"testing"
	"time"

	"github.com/observiq/carbon/entry"
	"github.com/stretchr/testify/require"
)

func TestParseValidTimestamp(t *testing.T) {
	xml := EventXML{
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
	}
	timestamp := xml.parseTimestamp()
	expected, _ := time.Parse(time.RFC3339Nano, "2020-07-30T01:01:01.123456789Z")
	require.Equal(t, expected, timestamp)
}

func TestParseInvalidTimestamp(t *testing.T) {
	xml := EventXML{
		TimeCreated: TimeCreated{
			SystemTime: "invalid",
		},
	}
	timestamp := xml.parseTimestamp()
	require.Equal(t, time.Now().Year(), timestamp.Year())
	require.Equal(t, time.Now().Month(), timestamp.Month())
	require.Equal(t, time.Now().Day(), timestamp.Day())
}

func TestParseSeverity(t *testing.T) {
	xmlCritical := EventXML{Level: "Critical"}
	xmlError := EventXML{Level: "Error"}
	xmlWarning := EventXML{Level: "Warning"}
	xmlInformation := EventXML{Level: "Information"}
	xmlUnknown := EventXML{Level: "Unknown"}
	require.Equal(t, entry.Critical, xmlCritical.parseSeverity())
	require.Equal(t, entry.Error, xmlError.parseSeverity())
	require.Equal(t, entry.Warning, xmlWarning.parseSeverity())
	require.Equal(t, entry.Info, xmlInformation.parseSeverity())
	require.Equal(t, entry.Default, xmlUnknown.parseSeverity())
}

func TestParseRecord(t *testing.T) {
	xml := EventXML{
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Message:  "message",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
	}

	expected := map[string]interface{}{
		"provider_name": "provider",
		"provider_id":   "guid",
		"event_source":  "event source",
		"system_time":   "2020-07-30T01:01:01.123456789Z",
		"computer":      "computer",
		"channel":       "application",
		"record_id":     uint64(1),
		"level":         "Information",
		"message":       "message",
		"task":          "task",
		"opcode":        "opcode",
		"keywords":      []string{"keyword"},
	}

	require.Equal(t, expected, xml.parseRecord())
}

func TestToEntry(t *testing.T) {
	xml := EventXML{
		Provider: Provider{
			Name:            "provider",
			GUID:            "guid",
			EventSourceName: "event source",
		},
		TimeCreated: TimeCreated{
			SystemTime: "2020-07-30T01:01:01.123456789Z",
		},
		Computer: "computer",
		Channel:  "application",
		RecordID: 1,
		Level:    "Information",
		Message:  "message",
		Task:     "task",
		Opcode:   "opcode",
		Keywords: []string{"keyword"},
	}
	timestamp, _ := time.Parse(time.RFC3339Nano, "2020-07-30T01:01:01.123456789Z")
	expected := entry.Entry{
		Record: map[string]interface{}{
			"provider_name": "provider",
			"provider_id":   "guid",
			"event_source":  "event source",
			"system_time":   "2020-07-30T01:01:01.123456789Z",
			"computer":      "computer",
			"channel":       "application",
			"record_id":     uint64(1),
			"level":         "Information",
			"message":       "message",
			"task":          "task",
			"opcode":        "opcode",
			"keywords":      []string{"keyword"},
		},
		Timestamp: timestamp,
		Severity:  entry.Info,
	}
	e := xml.ToEntry()

	require.Equal(t, expected.Record, e.Record)
	require.Equal(t, expected.Timestamp, e.Timestamp)
	require.Equal(t, expected.Severity, e.Severity)
}