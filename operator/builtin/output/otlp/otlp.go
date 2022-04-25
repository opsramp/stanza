package otlp

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"

	"github.com/opsramp/stanza/operator/flusher"

	"github.com/opsramp/stanza/entry"
	"github.com/opsramp/stanza/operator"
	"github.com/opsramp/stanza/operator/buffer"
	"github.com/opsramp/stanza/operator/helper"
	"go.opentelemetry.io/collector/model/otlpgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	operator.Register("otlp_output", func() operator.Builder { return NewOTLPConfig("") })
}

const authorization = "authorization"

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

	if !o.config.RetryDisabled {
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

	o.flush()
	return nil
}

// Process will write an entry to the endpoint.
func (o *OtlpOutput) Process(ctx context.Context, entry *entry.Entry) error {
	return o.buffer.Add(ctx, entry)
}

// Stop will close the connection.
func (o *OtlpOutput) Stop() error {
	o.cancel()
	o.wg.Wait()
	o.flusher.Stop()
	return o.clientConn.Close()
}

func (o *OtlpOutput) flush() {
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

			err := o.flushChunk()
			if err != nil {
				o.Errorw("failed to flush from buffer", zap.Error(err))
			}
		}
	}()
}

func (o *OtlpOutput) flushChunk() error {
	entries, clearer, err := o.buffer.ReadChunk(o.ctx)
	if err != nil {
		return fmt.Errorf("failed to read entries from buffer: %w", err)
	}

	entriesLen := len(entries)
	chunkID := uuid.New()
	o.Debugw("Read entries from buffer, ", "entries: ", entriesLen, ", chunk_id: ", chunkID)
	logRequest := buildProtoRequest(entries)
	o.Debugw("Created export requests ", "with ", entriesLen, " entries, chunk_id: ", chunkID)
	flushFunc := func(ctx context.Context) error {
		md := metadata.New(map[string]string{authorization: o.config.Authorization})
		ctx = metadata.NewOutgoingContext(ctx, md)
		_, err := o.logsClient.Export(ctx, logRequest)

		if err != nil {
			o.Debugw("Failed to send requests ", "chunk_id", chunkID, zap.Error(err))
			return err
		}

		o.Debugw("Marking entries as flushed,", "chunk_id: ", chunkID)
		return clearer.MarkAllAsFlushed()
	}
	o.flusher.Do(flushFunc)
	o.Debugw("Submitted requests to the flusher", "requests", entriesLen)

	return nil
}
