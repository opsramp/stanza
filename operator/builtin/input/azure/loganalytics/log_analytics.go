package loganalytics

import (
	"context"

	jsoniter "github.com/json-iterator/go"
	"github.com/opsramp/stanza/operator"
	"github.com/opsramp/stanza/operator/builtin/input/azure"
	"github.com/opsramp/stanza/operator/helper"
)

const operatorName = "azure_log_analytics_input"

func init() {
	operator.Register(operatorName, func() operator.Builder { return NewLogAnalyticsConfig("") })
}

// NewLogAnalyticsConfig creates a new Azure Log Analytics input config with default values
func NewLogAnalyticsConfig(operatorID string) *LogAnalyticsInputConfig {
	return &LogAnalyticsInputConfig{
		InputConfig: helper.NewInputConfig(operatorID, operatorName),
		AzureConfig: azure.AzureConfig{
			PrefetchCount: 1000,
			StartAt:       "end",
		},
	}
}

// LogAnalyticsInputConfig is the configuration of a Azure Log Analytics input operator.
type LogAnalyticsInputConfig struct {
	helper.InputConfig `yaml:",inline"`
	azure.AzureConfig  `yaml:",inline"`
}

// Build will build a Azure Log Analytics input operator.
func (c *LogAnalyticsInputConfig) Build(buildContext operator.BuildContext) ([]operator.Operator, error) {
	if err := c.AzureConfig.Build(buildContext, c.InputConfig); err != nil {
		return nil, err
	}

	logAnalyticsInput := &LogAnalyticsInput{
		EventHub: azure.EventHub{
			AzureConfig: c.AzureConfig,
			Persist: &azure.Persister{
				DB: helper.NewScopedDBPersister(buildContext.Database, c.ID()),
			},
		},
		json: jsoniter.ConfigFastest,
	}
	return []operator.Operator{logAnalyticsInput}, nil
}

// LogAnalyticsInput is an operator that reads Azure Log Analytics input from Azure Event Hub.
type LogAnalyticsInput struct {
	azure.EventHub
	json jsoniter.API
}

// Start will start generating log entries.
func (l *LogAnalyticsInput) Start() error {
	l.Handler = l.handleBatchedEvents
	return l.StartConsumers(context.Background())
}

// Stop will stop generating logs.
func (l *LogAnalyticsInput) Stop() error {
	return l.StopConsumers()
}
