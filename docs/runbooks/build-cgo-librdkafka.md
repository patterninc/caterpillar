# Build fails on cgo or librdkafka

`github.com/confluentinc/confluent-kafka-go/v2` needs cgo.

| Where | Symptom | Fix |
| --- | --- | --- |
| Local | `cgo: C compiler "gcc" not found` or `clang: not found` | Install a C compiler: `xcode-select --install` (macOS), `apt-get install build-essential` (Debian/Ubuntu). |
| Local | `undefined: kafka.` errors | `CGO_ENABLED=0` is set; unset it. |
| Docker build | `confluent-kafka-go requires librdkafka v2.14.0 or later` | `build/Dockerfile` installs librdkafka from Alpine `edge/community` because Alpine 3.20 ships 2.4.0. Check that edge still has a new enough package. |
| Docker build | `apk` fetch errors from `dl-cdn.alpinelinux.org` | Mirror outage; re-run the job. |
| CI arm64 | Linker errors | arm64 builds run natively on `ubuntu-24.04-arm` with the bundled `glibc_linux_arm64` librdkafka; check the runner label is unchanged. |

When bumping `confluent-kafka-go`, check its required librdkafka version against the Alpine edge package before merging, and run `docker build -f build/Dockerfile .` locally.
