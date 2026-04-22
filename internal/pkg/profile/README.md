# profile

Optional profiling for the caterpillar binary. Supports file-based pprof dumps
(local disk or S3) and/or a live `net/http/pprof` server.

## Flags

| Flag              | Description                                                                 |
|-------------------|-----------------------------------------------------------------------------|
| `-profile-dump`   | Destination for pprof files written on exit. Local directory or `s3://bucket/prefix`. |
| `-profile-serve` | Address for the live pprof HTTP server (e.g. `:6060`).                      |

Either flag (or both) may be set; profiling is skipped entirely when neither
is provided.

## `-profile-dump`

When set, the process:

1. Enables block and mutex sampling at startup (adds some runtime overhead).
2. Starts a CPU profile that runs for the lifetime of the pipeline.
3. On clean exit, on `bail`, or on `SIGINT`/`SIGTERM`, flushes these files to
   the destination:
   - `cpu.pprof`
   - `heap.pprof`
   - `goroutine.pprof`
   - `block.pprof`
   - `mutex.pprof`

The flush is idempotent — signal handlers, deferred flushes, and error-path
`bail` all converge on the same single write.

### Local destination

```bash
./caterpillar -conf pipeline.yaml -profile-dump /tmp/caterpillar-profile
```

The directory is created if it does not exist. CPU profile data streams
directly to disk.

### S3 destination

```bash
./caterpillar -conf pipeline.yaml -profile-dump s3://my-bucket/profiles/run-123
```

- AWS credentials and region are picked up from the default AWS SDK config
  chain (env vars, shared config, IMDS, etc.). Set `AWS_REGION` if needed.
- Each pprof file is buffered in memory and uploaded with a single
  `PutObject` on flush. For very long runs with large CPU profiles, prefer
  a local destination or `-server`.
- The prefix path segment is joined with the profile name, so the example
  above writes keys like `profiles/run-123/cpu.pprof`.

## `-profile-serve`

Starts `net/http/pprof` on the given address in a background goroutine:

```bash
./caterpillar -conf pipeline.yaml -profile-server :6060
```

Then pull profiles on demand:

```bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
go tool pprof http://localhost:6060/debug/pprof/heap
```

Unlike `-profile-dump`, this does not enable block or mutex sampling
automatically; combine with `-profile-dump` if you want those profiles
populated.

## Analyzing profiles

```bash
go tool pprof /tmp/caterpillar-profile/cpu.pprof
go tool pprof -http=: /tmp/caterpillar-profile/heap.pprof
```

For S3-hosted profiles, download first (`aws s3 cp`) and run `go tool pprof`
against the local copy.
