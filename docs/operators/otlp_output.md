## `otlp_output` operator

The `otlp_output` operator will send entries to OTLP API service (e.g. ingestion service of Tracing project)

### Configuration Fields

| Field              | Default       | Description                                                                              |
|--------------------|---------------|------------------------------------------------------------------------------------------|
| `id`               | `otlp_output` | A unique identifier for the operator                                                     |
| `endpoint`         |               | Endpoint for the OTLP API service                                                        |
| `headers`          |               | Authentication tokes (see eamples)                                                       |
| `insecure`         |               | Not used now (token authentication instead, see examples)                                |
| `buffer`           |               | A [buffer](/docs/types/buffer.md) block indicating how to buffer entries before flushing |
| `flusher`          |               | A [flusher](/docs/types/flusher.md) block configuring flushing behavior                  |
| `retry_on_failure` | true          | Enables retry attempt for DialOption at time of establishing gRPC connection             |

### Example Configurations

#### Simple configuration (minimal)

Configuration:
```yaml
 - type: otlp_output
     id: otlp
     endpoint: localhost:8086
   headers:
     authorization: 5b42b134-1111-1111-1111-b550fecf90551
```

#### Configuration with non-default buffer and flusher params

Configuration:
```yaml
- type: otlp_output
    id: otlp
    endpoint: localhost:8086
    insecure: true
    buffer:
      type: memory
      max_chunk_size: 5
      max_delay: 10s
    flusher:
      max_retry_interval: 5s
      max_elapsed_time: 20s
    headers:
      authorization: 5b42b134-1111-1111-1111-b550fecf90551
    retry_on_failure: true
```
