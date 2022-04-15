# Buffers

Buffers are used to temporarily store log entries until they can be flushed to their final destination.

There are two types of buffers: `memory` buffers and `disk` buffers.

## Memory Buffers

Memory buffers keep log entries in memory until they are flushed, which makes them very fast. However, because
entries are only stored in memory, they will be lost if the agent is shut down uncleanly. If the agent is shut down
cleanly, they will be saved to the agent's database.

### Memory Buffer Configuration

Memory buffers are configured by setting the `type` field of the `buffer` block on an output to `memory`. 

| Field            | Default          | Description                                                                      |
|------------------| ---              | ---                                                                              |
| `max_entries`    | `1048576` (2^20) | The maximum number of entries stored in the memory buffer                        |
| `max_chunk_size` | 1000             | The maximum number of entries that are read from the buffer by default           |
| `max_delay`      | 1s               | The maximum amount of time that a reader will wait to batch entries into a chunk |

Example:
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


