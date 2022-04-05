package otlp

import (
	"fmt"
	"testing"
	"time"

	"github.com/observiq/stanza/database"
	"github.com/observiq/stanza/entry"
	"github.com/observiq/stanza/operator"
	"github.com/observiq/stanza/operator/buffer"
	"github.com/observiq/stanza/operator/flusher"
	"github.com/observiq/stanza/operator/helper"
	"github.com/observiq/stanza/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestOtlpOperator(t *testing.T) {
	cfg := OtlpConfig{
		OutputConfig: helper.OutputConfig{
			BasicConfig: helper.BasicConfig{
				OperatorID:   "test_operator_id",
				OperatorType: "otlp",
			}},
		Endpoint:      "test:80",
		Insecure:      "",
		Headers:       Headers{Authorization: "test"},
		RetrySettings: RetrySettings{Enabled: true},
		Timeout:       5,
	}

	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NotNil(t, ops)
	require.NoError(t, err)

	//op := ops[0]

	entry := entry.New()
	entry.Timestamp = time.Now()
	entry.Resource = map[string]string{"test": "test"}
	entry.Record = "test message"

	//TODO mock logsClient.Export call
	//err = op.(*OtlpOutput).Process(context.Background(), entry)
	require.NoError(t, err)
}

func TestOtlpConfig_Build(t *testing.T) {
	type fields struct {
		OutputConfig  helper.OutputConfig
		BufferConfig  buffer.Config
		FlusherConfig flusher.Config
		Endpoint      string
		Insecure      string
		Headers       Headers
		RetrySettings RetrySettings
		Timeout       time.Duration
	}
	type args struct {
		bc operator.BuildContext
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    error
		wantErr bool
	}{
		{
			name: "Missed endpoint error",
			fields: fields{
				OutputConfig: helper.OutputConfig{
					BasicConfig: helper.BasicConfig{OperatorID: "test", OperatorType: "otlp"},
				},
				BufferConfig:  buffer.NewConfig(),
				FlusherConfig: flusher.NewConfig(),
				Endpoint:      "",
			},
			args: args{
				bc: operator.NewBuildContext(nil, zap.NewNop().Sugar()),
			},
			want:    fmt.Errorf("operator must provide an endpoint"),
			wantErr: true,
		},
		{
			name: "Missed logger error",
			fields: fields{
				OutputConfig: helper.OutputConfig{
					BasicConfig: helper.BasicConfig{OperatorID: "test", OperatorType: "otlp"},
				},
				BufferConfig:  buffer.NewConfig(),
				FlusherConfig: flusher.NewConfig(),
				Endpoint:      "test",
			},

			want:    fmt.Errorf("operator build context is missing a logger.: {\"operator_id\":\"test\",\"operator_type\":\"otlp\"}"),
			wantErr: true,
		},
		{
			name: "No error",
			fields: fields{
				OutputConfig: helper.OutputConfig{
					BasicConfig: helper.BasicConfig{OperatorID: "test", OperatorType: "otlp"},
				},
				BufferConfig:  buffer.NewConfig(),
				FlusherConfig: flusher.NewConfig(),
				Endpoint:      "test",
			},
			args: args{
				bc: operator.NewBuildContext(database.NewStubDatabase(), zap.NewNop().Sugar()),
			},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := OtlpConfig{
				OutputConfig:  tt.fields.OutputConfig,
				BufferConfig:  tt.fields.BufferConfig,
				FlusherConfig: tt.fields.FlusherConfig,
				Endpoint:      tt.fields.Endpoint,
				Timeout:       tt.fields.Timeout,
			}
			_, err := c.Build(tt.args.bc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				require.Equal(t, err.Error(), tt.want.Error())
			}

		})
	}
}