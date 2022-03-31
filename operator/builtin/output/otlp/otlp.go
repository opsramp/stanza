package otlp

import (
	"context"
	"fmt"
	"sync"

	"github.com/observiq/stanza/operator/flusher"
	"go.uber.org/zap"

	"github.com/observiq/stanza/entry"
	"github.com/observiq/stanza/operator"
	"github.com/observiq/stanza/operator/buffer"
	"github.com/observiq/stanza/operator/helper"
	"go.opentelemetry.io/collector/model/otlpgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	operator.Register("otlp", func() operator.Builder { return NewOTLPConfig("") })
}

const authorization = "authorization"

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

// OtlpOutput is an operator that writes logs to a service.
type OtlpOutput struct {
	helper.OutputOperator
	config     OtlpConfig
	buffer     buffer.Buffer
	flusher    *flusher.Flusher
	logsClient otlpgrpc.LogsClient
	clientConn *grpc.ClientConn

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start will open the connection.
func (o *OtlpOutput) Start() error {
	var opts []grpc.DialOption
	var err error
	ctx := context.Background()

	if !o.config.RetrySettings.Enabled {
		opts = append(opts, grpc.WithDisableRetry())
	}

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if o.config.Timeout > 0 {
		ctx, _ = context.WithTimeout(ctx, o.config.Timeout)
	}

	o.clientConn, err = grpc.DialContext(ctx, o.config.Endpoint, opts...)
	if err != nil {
		return err
	}
	o.logsClient = otlpgrpc.NewLogsClient(o.clientConn)

	o.wg.Add(1)
	o.flush(ctx)
	return nil
}

// Stop will close the connection.
func (o *OtlpOutput) Stop() error {
	o.cancel()
	o.wg.Wait()
	o.flusher.Stop()
	return o.clientConn.Close()
}

// Process will write an entry to the endpoint.
func (o *OtlpOutput) Process(ctx context.Context, entry *entry.Entry) error {

	return o.buffer.Add(ctx, entry)

	/*
		md := metadata.New(map[string]string{authorization: o.config.Authorization})

		ctx = metadata.NewOutgoingContext(ctx, md)

		logRequest := otlpgrpc.NewLogsRequest()
		logRequest.SetLogs(convert(entry))
		_, err := o.logsClient.Export(ctx, logRequest)

		return err
	*/
}

func (o *OtlpOutput) flush(ctx context.Context) {
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		defer o.Debug("flusher stopped")
		for {
			select {
			case <-o.ctx.Done():
				o.Debug("context completed while flushing")
				return
			default:
			}

			err := o.flushChunk(ctx)
			if err != nil {
				o.Errorw("failed to flush from buffer", zap.Error(err))
			}
		}
	}()
}

func (o *OtlpOutput) flushChunk(ctx context.Context) error {
	return nil
}
