# SFTP Task

The `sftp` task transfers files to and from SFTP servers. It reads credentials from AWS SSM, verifies the server's host key, and retries on unstable connections.

## Function

The `sftp` task handles SFTP only. It does not talk to S3 directly. Instead, it is combined with the existing [`file`](../file) task:

- **Upload (S3 → SFTP)**: the `file` task reads from `s3://…` and the `sftp` task (`operation: upload`) writes the files to the server.
- **Download (SFTP → S3)**: the `sftp` task (`operation: download`) reads files from the server and the `file` task writes them to `s3://…`.

This reuses Caterpillar's existing S3 code. The file name passes from one task to the next through a record context value, so `file` and `sftp` work together without extra configuration.

## Behavior

The `operation` field sets what the task does:

| Operation | Role | Input | Output | What it does |
|-----------|------|-------|--------|--------------|
| `upload` | sink | required | optional (pass-through) | Writes each record's data to the server. If `remote_path` is a directory, the source file name is added to it. |
| `download` | source | none | required | Reads file(s) at `remote_path` (a single file, a glob, or a directory) and emits one record per file. |
| `list` | source | none | required | Emits one JSON record per directory entry: `{name, size, mod_time, is_dir}`. |
| `move` | action | optional | optional (pass-through) | Renames `remote_path` to `destination_path`. |
| `delete` | action | optional | optional (pass-through) | Deletes `remote_path`. |

For `move` and `delete`: if the task has an input, it runs once for each record (the paths can use record values). If it has no input, it runs once using the configured paths.

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
| `operation` | string | - | One of `upload`, `download`, `list`, `move`, `delete` (required) |
| `host` | string | - | SFTP server hostname (required) |
| `port` | int | `22` | SFTP server port |
| `username` | string | - | SFTP username (required) |
| `password` | string | - | Password (use `{{ secret }}`); set this OR `private_key` |
| `private_key` | string | - | PEM private key (use `{{ secret }}`); set this OR `password` |
| `passphrase` | string | - | Passphrase for an encrypted `private_key` |
| `host_key` | string | - | Authorized-key line used to verify the server |
| `known_hosts_path` | string | - | Path to a `known_hosts` file |
| `remote_path` | string | - | Remote file or directory (supports templating). On upload, a trailing `/` or an existing directory is treated as a directory. |
| `destination_path` | string | - | Target path for `move` (supports templating) |
| `timeout` | duration | `30s` | SSH connection timeout (for example `15s`, `1m`) |
| `max_retries` | int | `3` | Attempts per connect or transfer operation |
| `retry_delay` | duration | `1s` | Delay between retries |
| `fail_on_error` | bool | `false` | Stop the pipeline if this task fails |
| `task_concurrency` | int | `1` | Number of parallel workers. For SFTP, each worker opens its own connection (see Notes and limitations) |
| `context` | map | - | jq expressions that copy values from each record into its context for later tasks |

## Examples

### Upload files from S3 to an SFTP server
```yaml
tasks:
  - name: read_from_s3
    type: file
    path: s3://my-bucket/outbound/*.csv
  - name: push_to_client
    type: sftp
    operation: upload
    host: sftp.client.example.com
    username: '{{ secret "/data/sftp/clientX/username" }}'
    private_key: |
      {{ indent 6 (secret "/data/sftp/clientX/private_key") }}
    host_key: '{{ secret "/data/sftp/clientX/host_key" }}'
    remote_path: /incoming/
```

### Download files from an SFTP server to S3
```yaml
tasks:
  - name: pull_from_client
    type: sftp
    operation: download
    host: sftp.client.example.com
    username: '{{ secret "/data/sftp/clientX/username" }}'
    password: '{{ secret "/data/sftp/clientX/password" }}'
    known_hosts_path: /etc/ssh/known_hosts
    remote_path: /outgoing/*.csv
  - name: write_to_s3
    type: file
    # The file task writes to `path` exactly as given. The sftp download stores
    # each file's name in CATERPILLAR_FILE_NAME_WRITE, so add it to the key.
    # Without this, every file would write to the same key.
    path: s3://my-bucket/inbound/{{ context "CATERPILLAR_FILE_NAME_WRITE" }}
```

### List, move, and delete
```yaml
tasks:
  - name: list_incoming
    type: sftp
    operation: list
    host: sftp.client.example.com
    username: '{{ secret "/data/sftp/clientX/username" }}'
    password: '{{ secret "/data/sftp/clientX/password" }}'
    host_key: '{{ secret "/data/sftp/clientX/host_key" }}'
    remote_path: /incoming
  - name: echo
    type: echo
    only_data: true
```

## Notes and limitations

**Files are held in memory.** In the `file → sftp` flow, the `file` task reads each file fully into memory before the `sftp` task sends it. Memory use is therefore set by the size of the largest single file, not the total of all files. This is fine for small and medium files. For very large files (several GB), the task may use too much memory. End-to-end streaming is not supported yet.

**`task_concurrency` opens multiple connections.** Setting `task_concurrency` above `1` makes the task open one SSH connection per worker and transfer files in parallel. This is faster, but it increases memory use (more files in memory at once), and some servers limit the number of connections per user.

**Nested directories are not preserved.** Each file is identified by its base name only. If you upload files from nested S3 folders with a recursive glob (for example `s3://bucket/data/**/*.csv`), they all land in the single `remote_path` directory, and files with the same name overwrite each other. To keep a folder structure, use a separate `file → sftp` pair for each folder, with the target path set for each one.
