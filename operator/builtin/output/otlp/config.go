package otlp

import (
	"context"
	"fmt"
	"github.com/observiq/stanza/operator"
	"time"

	"github.com/observiq/stanza/operator/buffer"
	"github.com/observiq/stanza/operator/flusher"

	"github.com/observiq/stanza/operator/helper"
)

// GRPCClientSettings defines common settings for a gRPC client configuration.
type GRPCClientSettings struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`
}

// Headers defines headers settings for a gRPC client configuration.
type Headers struct {
	Authorization string `json:"authorization" yaml:"authorization"`
}

// OtlpConfig is the configuration of an otlp output operation.
type OtlpConfig struct {
	helper.OutputConfig `yaml:",inline"`
	BufferConfig        buffer.Config  `json:"buffer,omitempty" yaml:"buffer,omitempty"`
	FlusherConfig       flusher.Config `json:"flusher,omitempty" yaml:"flusher,omitempty"`
	Endpoint            string         `json:"endpoint" yaml:"endpoint"`
	Insecure            string         `json:"insecure" yaml:"insecure"`
	Headers             `json:"headers" yaml:"headers"`
	RetryDisabled       bool          `json:"retry_on_failure" yaml:"retry_on_failure"`
	Timeout             time.Duration `json:"timeout" yaml:"timeout"`
}

// NewOTLPConfig creates a new otlp output config with default values
func NewOTLPConfig(operatorID string) *OtlpConfig {
	return &OtlpConfig{
		OutputConfig:  helper.NewOutputConfig(operatorID, "otlp_output"),
		BufferConfig:  buffer.NewConfig(),
		FlusherConfig: flusher.NewConfig(),
	}
}

// Build will build a otlp output operator.
func (c OtlpConfig) Build(bc operator.BuildContext) ([]operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(bc)
	if err != nil {
		return nil, err
	}

	if c.Endpoint == "" {
		return nil, fmt.Errorf("operator must provide an endpoint")
	}

	buffer, err := c.BufferConfig.Build(bc, c.ID())
	if err != nil {
		return nil, err
	}

	flusher := c.FlusherConfig.Build(bc.Logger.SugaredLogger)
	ctx, cancel := context.WithCancel(context.Background())

	otlpOutput := &OtlpOutput{
		OutputOperator: outputOperator,
		config:         c,
		buffer:         buffer,
		flusher:        flusher,
		ctx:            ctx,
		cancel:         cancel,
	}

	return []operator.Operator{otlpOutput}, nil
}
