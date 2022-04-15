# Flusher

Flusher handle reading entries from buffers in chunks, flushing them to their final destination, and retrying on failure.


## Flusher configuration

Flushers are configured with the `flusher` block on output plugins.

| Field              | Default | Description                                                        |
|--------------------|---------|--------------------------------------------------------------------|
| `max_elapsed_time`  | 60m     | Maximum time for retry atempts for one batch, see explanation below|
| `max_retry_interval`| 60s     | Maximum time between retry attempts for one batch                  |
| `max_concurrent`    | `16`    | The maximum number of goroutines flushing entries concurrently     |


Flusher uses github.com/cenkalti/backoff/v4 library, see explanation:

ExponentialBackOff is a backoff implementation that increases the backoff
period for each retry attempt using a randomization function that grows exponentially.

NextBackOff() is calculated using the following formula:

randomized interval =
RetryInterval * (random value in range [1 - RandomizationFactor, 1 + RandomizationFactor])

In other words NextBackOff() will range between the randomization factor
percentage below and above the retry interval.

For example, given the following parameters:

RetryInterval = 2
RandomizationFactor = 0.5
Multiplier = 2

the actual backoff period used in the next retry attempt will range between 1 and 3 seconds,
multiplied by the exponential, that is, between 2 and 6 seconds.

Note: MaxInterval caps the RetryInterval and not the randomized interval.

If the time elapsed since an ExponentialBackOff instance is created goes past the
MaxElapsedTime, then the method NextBackOff() starts returning backoff.Stop.

The elapsed time can be reset by calling Reset().

Example: Given the following default arguments, for 10 tries the sequence will be,
and assuming we go over the MaxElapsedTime on the 10th try:

Request #  RetryInterval (seconds)  Randomized Interval (seconds)
```
1          0.5                     [0.25,   0.75]
2          0.75                    [0.375,  1.125]
3          1.125                   [0.562,  1.687]
4          1.687                   [0.8435, 2.53]
5          2.53                    [1.265,  3.795]
6          3.795                   [1.897,  5.692]
7          5.692                   [2.846,  8.538]
8          8.538                   [4.269, 12.807]
9         12.807                   [6.403, 19.210]
10         19.210                   backoff.Stop
```
Note: Implementation is not thread-safe.
