package main

import (
	// Load packages when importing input operators
	_ "github.com/opsramp/stanza/operator/builtin/input/aws/cloudwatch"
	_ "github.com/opsramp/stanza/operator/builtin/input/azure/eventhub"
	_ "github.com/opsramp/stanza/operator/builtin/input/azure/loganalytics"
	_ "github.com/opsramp/stanza/operator/builtin/input/file"
	_ "github.com/opsramp/stanza/operator/builtin/input/filecheck"
	_ "github.com/opsramp/stanza/operator/builtin/input/forward"
	_ "github.com/opsramp/stanza/operator/builtin/input/generate"
	_ "github.com/opsramp/stanza/operator/builtin/input/goflow"
	_ "github.com/opsramp/stanza/operator/builtin/input/http"
	_ "github.com/opsramp/stanza/operator/builtin/input/journald"
	_ "github.com/opsramp/stanza/operator/builtin/input/k8sevent"
	_ "github.com/opsramp/stanza/operator/builtin/input/stanza"
	_ "github.com/opsramp/stanza/operator/builtin/input/stdin"
	_ "github.com/opsramp/stanza/operator/builtin/input/tcp"
	_ "github.com/opsramp/stanza/operator/builtin/input/udp"

	_ "github.com/opsramp/stanza/operator/builtin/parser/csv"
	_ "github.com/opsramp/stanza/operator/builtin/parser/json"
	_ "github.com/opsramp/stanza/operator/builtin/parser/keyvalue"
	_ "github.com/opsramp/stanza/operator/builtin/parser/regex"
	_ "github.com/opsramp/stanza/operator/builtin/parser/severity"
	_ "github.com/opsramp/stanza/operator/builtin/parser/syslog"
	_ "github.com/opsramp/stanza/operator/builtin/parser/time"
	_ "github.com/opsramp/stanza/operator/builtin/parser/uri"
	_ "github.com/opsramp/stanza/operator/builtin/parser/xml"

	_ "github.com/opsramp/stanza/operator/builtin/transformer/add"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/copy"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/filter"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/flatten"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/hostmetadata"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/k8smetadata"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/metadata"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/move"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/noop"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/ratelimit"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/recombine"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/remove"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/restructure"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/retain"
	_ "github.com/opsramp/stanza/operator/builtin/transformer/router"

	_ "github.com/opsramp/stanza/operator/builtin/output/count"
	_ "github.com/opsramp/stanza/operator/builtin/output/drop"
	_ "github.com/opsramp/stanza/operator/builtin/output/elastic"
	_ "github.com/opsramp/stanza/operator/builtin/output/file"
	_ "github.com/opsramp/stanza/operator/builtin/output/forward"
	_ "github.com/opsramp/stanza/operator/builtin/output/googlecloud"
	_ "github.com/opsramp/stanza/operator/builtin/output/newrelic"
	_ "github.com/opsramp/stanza/operator/builtin/output/otlp"
	_ "github.com/opsramp/stanza/operator/builtin/output/stdout"
)
