# Run never finishes, no summary printed

**Why it happens:** a task stopped (returned an error) while its upstream was still producing. The channel between them fills to `channel_size` (default 10,000) and the upstream blocks on send forever, so no output channel closes and `Pipeline.Run` never returns (README, "Backpressure caveat"). A source with `exit_on_empty: false` (SQS) or no end condition (Kafka, `http_server`) also runs until killed, by design.

## Steps

1. Look for an `error in <task>:` line on stdout near where output stopped. That task is the one that stopped consuming.
2. Get a goroutine dump to see who is blocked: send `SIGQUIT` (`kill -QUIT <pid>`) to the process; Go prints every goroutine's stack. Goroutines stuck in `chan send` inside `SendRecord` are the blocked upstream.
3. If no task errored, check whether the source is meant to end: SQS needs `exit_on_empty: true`, `end_after` or `max_records`; Kafka needs an end condition from its README.
4. Kill the run. For SQS and Kafka sources nothing unsettled is lost: unacknowledged messages are redelivered (see [redelivery-and-data-loss.md](redelivery-and-data-loss.md)).
5. Fix the failing task, or make it non-blocking for that error class, and re-run.
