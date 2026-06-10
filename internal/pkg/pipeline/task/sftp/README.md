# SFTP Task

The `sftp` task transfers files to and from SFTP servers. It reads credentials from AWS SSM, verifies the server's host key, and retries on unstable connections.

## Function

The `sftp` task handles SFTP only. It does not talk to S3 directly. Instead, it is combined with the existing [`file`](../file) task:

- **Upload (S3 → SFTP)**: the `file` task reads from `s3://…` and the `sftp` task writes the files to the server.
- **Download (SFTP → S3)**: the `sftp` task reads files from the server and the `file` task writes them to `s3://…`.

This reuses Caterpillar's existing S3 code. On download, each file's sanitized base name is stored in a record context value; on upload you reference it in `path` (`{{ context "CATERPILLAR_FILE_NAME_WRITE" }}`) to keep consistent names.

## Behavior

Like the `file` task, the role is **inferred from the channels**:

| The task has… | Role | What it does |
|---------------|------|--------------|
| **no input** (it is the first task) | **source — download** | Reads file(s) at `path` (a single file or a glob; doublestar `**` and `{a,b}` are supported, like the file task) and emits one record per file. The base name is stored in the record context (`CATERPILLAR_FILE_NAME_WRITE`), and the full sanitized source path in `CATERPILLAR_FILE_PATH_WRITE`, so a downstream task can name what it writes. |
| **an input** | **sink — upload** | Writes each incoming record's data to `path`, used as-is per record. To name files from the source, template `path` — e.g. `{{ context "CATERPILLAR_FILE_NAME_WRITE" }}`. |

It cannot be both: configuring the task with both an input and an output is an error.

For non-file sources that don't set a file name (Kafka, HTTP, …), template `path` yourself — with a macro like `{{ macro "uuid" }}`, a value extracted from the record via a `context:` jq map, or `{{ context "CATERPILLAR_ARCHIVE_FILE_NAME_WRITE" }}` for an archive-unpack source.

## Authentication

Set **exactly one** of `password` or `private_key`. Read credentials from SSM with the `{{ secret }}` template. Do not write them directly in the YAML.

```yaml
# Password authentication
username: '{{ secret "/data/sftp/clientX/username" }}'
password: '{{ secret "/data/sftp/clientX/password" }}'
```

```yaml
# SSH private key authentication. The key is multi-line, so use a YAML block
# scalar with the indent helper. Set the indent value to match the block's
# indentation -- 6 here, because task fields sit under a list item (`- name:`),
# as in the examples below.
    username: '{{ secret "/data/sftp/clientX/username" }}'
    private_key: |
      {{ indent 6 (secret "/data/sftp/clientX/private_key") }}
    passphrase: '{{ secret "/data/sftp/clientX/passphrase" }}'   # only if the key is encrypted
```

## Host key verification

By default, the task verifies the server's identity to prevent man-in-the-middle attacks. Provide one of:

- `host_key`: a single authorized-key line, for example `ssh-ed25519 AAAA...` (the key part of a `known_hosts` entry).
- `known_hosts_path`: the path to a `known_hosts` file.

If you set neither, the task refuses to connect (it fails closed). You can obtain a server's host key with `ssh-keyscan -t ed25519 -p <port> <host>`.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name |
| `type` | string | - | Must be `sftp` |
| `host` | string | - | SFTP server hostname (required) |
| `port` | int | `22` | SFTP server port |
| `username` | string | - | SFTP username (required) |
| `password` | string | - | Password (use `{{ secret }}`); set this OR `private_key` |
| `private_key` | string | - | PEM private key (use `{{ secret }}`); set this OR `password` |
| `passphrase` | string | - | Passphrase for an encrypted `private_key` |
| `host_key` | string | - | Authorized-key line used to verify the server |
| `known_hosts_path` | string | - | Path to a `known_hosts` file |
| `path` | string | - | Remote file path (required; supports per-record templating). On download it may be a glob (`**`/`{a,b}` supported); a bare directory is not expanded. Used as-is — template a context value such as `{{ context "CATERPILLAR_FILE_NAME_WRITE" }}` to name uploaded files from the source. |
| `timeout` | duration | `30s` | SSH connection timeout (for example `15s`, `1m`) |
| `max_retries` | int | `3` | Attempts per connect or transfer operation |
| `retry_delay` | duration | `1s` | Delay between retries |
| `fail_on_error` | bool | `false` | Stop the pipeline if this task fails |
| `task_concurrency` | int | `1` | Number of parallel workers. Each worker opens its own connection (see Notes and limitations) |
| `context` | map | - | jq expressions that copy values from each record into its context for later tasks |

## Examples

### Upload files from S3 to an SFTP server
The `sftp` task has an input (from the `file` task), so it acts as a sink and uploads.
```yaml
tasks:
  - name: read_from_s3
    type: file
    path: s3://my-bucket/outbound/*.csv
  - name: push_to_client
    type: sftp
    host: sftp.client.example.com
    username: '{{ secret "/data/sftp/clientX/username" }}'
    private_key: |
      {{ indent 6 (secret "/data/sftp/clientX/private_key") }}
    host_key: '{{ secret "/data/sftp/clientX/host_key" }}'
    path: '/incoming/{{ context "CATERPILLAR_FILE_NAME_WRITE" }}'
```

### Download files from an SFTP server to S3
The `sftp` task has no input, so it acts as a source and downloads.
```yaml
tasks:
  - name: pull_from_client
    type: sftp
    host: sftp.client.example.com
    username: '{{ secret "/data/sftp/clientX/username" }}'
    password: '{{ secret "/data/sftp/clientX/password" }}'
    known_hosts_path: /etc/ssh/known_hosts
    path: /outgoing/*.csv
  - name: write_to_s3
    type: file
    # The file task writes to `path` exactly as given. The sftp download stores
    # each file's name in CATERPILLAR_FILE_NAME_WRITE, so add it to the key.
    # Without this, every file would write to the same key.
    path: s3://my-bucket/inbound/{{ context "CATERPILLAR_FILE_NAME_WRITE" }}
```

## Notes and limitations

**Files are held in memory.** In the `file → sftp` flow, the `file` task reads each file fully into memory before the `sftp` task sends it. Memory use is therefore set by the size of the largest single file, not the total of all files. This is fine for small and medium files. For very large files (several GB), the task may use too much memory. End-to-end streaming is not supported yet.

**`task_concurrency` opens multiple connections.** Setting `task_concurrency` above `1` makes the task open one SSH connection per worker and transfer files in parallel. This is faster, but it increases memory use (more files in memory at once), and some servers limit the number of connections per user.

**Nested directories are not preserved.** If you template the destination with `CATERPILLAR_FILE_NAME_WRITE`, each file is identified by its base name only: download files from nested folders with a recursive glob (for example `/data/**/*.csv`) and they all land in the single destination directory, where files with the same name overwrite each other. To avoid collisions, template with `CATERPILLAR_FILE_PATH_WRITE` instead — it encodes the full source path into the (flat) filename, so same-named files from different folders stay distinct. To keep an actual folder structure on the target, use a separate `file → sftp` pair for each folder, with the target path set for each one.
