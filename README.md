# Sonar Insights

[![CI](https://github.com/sonar-solutions/sonar-insights/actions/workflows/ci.yml/badge.svg)](https://github.com/sonar-solutions/sonar-insights/actions/workflows/ci.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=sonar-insights&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=sonar-insights)
[![Go Version](https://img.shields.io/github/go-mod/go-version/sonar-solutions/sonar-insights)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/github/license/sonar-solutions/sonar-insights)](LICENSE.txt)
[![Latest Release](https://img.shields.io/github/v/release/sonar-solutions/sonar-insights)](https://github.com/sonar-solutions/sonar-insights/releases/latest)

`sonar-insights` is a command-line tool that turns raw data from a SonarQube
instance into self-contained HTML reports. It is designed to help platform
owners, performance engineers, and SonarQube administrators understand how
their server is being used — without standing up additional infrastructure.

It works against both **SonarQube Cloud** (`sonarcloud.io`, `sonarcloud.us`)
and self-hosted **SonarQube Server**.

## How it works

The tool is split into two stages so that data collection and report
generation can be run independently:

```
SonarQube  ──collect──▶  ./sonar-data/      (raw JSON, on disk)
                              │
                              └──analyze──▶  ./sonar-reports/*.html
```

- **`collect`** authenticates against SonarQube, paginates the relevant API
  endpoints in parallel, and writes the raw responses to disk. It never
  produces reports.
- **`analyze`** reads previously collected data from disk and renders a
  single, self-contained HTML report. It never talks to SonarQube.
- **`run`** is a convenience wrapper that performs `collect` followed by
  `analyze` in one invocation.

Splitting the two stages means you can re-run analysis (different date
range, different report filename) over an already-collected dataset, share
the raw data with someone else for analysis, or schedule collection
separately from reporting.

## Installation

### From a GitHub release (recommended)

Pre-built binaries for Linux, macOS, and Windows on `amd64` and `arm64` are
published on the [releases page](https://github.com/sonar-solutions/sonar-insights/releases/latest).

1. Download the archive that matches your OS and CPU architecture, e.g.
   `sonar-insights_Darwin_arm64.tar.gz` for an Apple Silicon Mac or
   `sonar-insights_Linux_x86_64.tar.gz` for a 64-bit Linux host.
2. Extract the archive — it contains a single binary named `sonar-insights`.
3. Move the binary somewhere on your `PATH`, for example:

   ```sh
   tar -xzf sonar-insights_Darwin_arm64.tar.gz
   sudo mv sonar-insights /usr/local/bin/
   ```

4. Verify the install:

   ```sh
   sonar-insights --help
   ```

> **macOS note:** if Gatekeeper blocks the binary the first time you run it,
> remove the quarantine flag with `xattr -d com.apple.quarantine $(which sonar-insights)`,
> or right-click the binary in Finder and choose **Open**.

### With `go install`

If you already have a Go toolchain installed (see `go.mod` for the required
version), you can build and install directly from the module proxy:

```sh
go install github.com/sonar-solutions/sonar-insights@latest
```

The binary will be placed in `$(go env GOBIN)` — typically `$HOME/go/bin` —
which you may want to add to your `PATH`.

### From source

```sh
git clone https://github.com/sonar-solutions/sonar-insights.git
cd sonar-insights
go build -o sonar-insights .
./sonar-insights --help
```

## Configuration

Authentication and the SonarQube endpoint can be supplied either via flags
or via environment variables. Flags take precedence.

| Setting              | Flag       | Environment variable | Default                  |
| -------------------- | ---------- | -------------------- | ------------------------ |
| SonarQube base URL   | `--url`    | `SONAR_HOST_URL`     | `https://sonarcloud.io`  |
| Authentication token | `--token`  | `SONAR_TOKEN`        | _(required)_             |

Generate a user token from your SonarQube account under
**My Account → Security → Generate Tokens**. The token needs read access to
the data you intend to collect.

## Usage

### Quickstart: collect and report in one shot

```sh
export SONAR_TOKEN=squ_xxxxxxxxxxxxxxxxxxxxxxxxxxxx
sonar-insights run bgtasks
```

This fetches the data, writes it under `./sonar-data/`, and produces
`./sonar-reports/report-bgtasks.html`. Open that file in any browser — it
is fully self-contained and works offline.

### Collecting data only

```sh
sonar-insights collect bgtasks \
  --url https://sonarqube.example.com \
  --token "$SONAR_TOKEN" \
  --out-dir ./sonar-data/ \
  --parallel 8
```

The collector wipes and re-creates `--out-dir` on each run so that stale
files cannot mix with a new collection. A small `collect-metadata.json`
file recording the source URL, server version, and collection timestamp
is written alongside the raw data.

### Analyzing previously collected data

```sh
sonar-insights analyze bgtasks \
  --dir ./sonar-data/ \
  --report-dir ./sonar-reports/ \
  --report-name march-2026 \
  --from 2026-03-01 \
  --to 2026-03-31
```

`--from` and `--to` accept dates in `YYYY-MM-DD` format and are interpreted
in UTC. Both are optional; omit either to leave that side of the range
unbounded.

### Global flags

These flags are accepted by every subcommand:

- `-v`, `--verbose` — enable debug-level logging.
- `--timing` — print total wall-clock execution time on exit.

For the full list of flags for any command, append `--help`:

```sh
sonar-insights collect --help
sonar-insights analyze bgtasks --help
sonar-insights run --help
```

## Output layout

After a successful run you will have:

```
sonar-data/
├── collect-metadata.json          # source URL, server version, timestamp
└── bgtasks/
    ├── background-tasks-page-0001.json
    ├── background-tasks-page-0002.json
    └── ...

sonar-reports/
└── report-bgtasks.html            # the report (open in a browser)
```

## Development

Requirements:

- Go (version pinned in [`go.mod`](go.mod))
- [`golangci-lint`](https://golangci-lint.run/)

Common tasks:

```sh
go build ./...           # compile everything
go test ./...            # run the full test suite
go fmt ./...             # format the codebase
golangci-lint run        # lint
```

Tests, `go fmt`, and `golangci-lint run` must all succeed before changes
are committed.

Releases are built with [GoReleaser](https://goreleaser.com/); the
configuration lives in [`.goreleaser.yaml`](.goreleaser.yaml).

## License

Released under the [MIT License](LICENSE.txt).
