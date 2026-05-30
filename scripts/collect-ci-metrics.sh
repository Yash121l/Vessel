#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/collect-ci-metrics.sh --mode <ci|release> --primary-binary <path> [options]

Options:
  --mode <ci|release>       Report mode.
  --version <value>         Version label to include in the report.
  --dist-dir <path>         Directory containing release binaries. Default: dist
  --output-dir <path>       Directory for generated report files. Default: ci-metrics
  --budget-file <path>      Budget env file. Default: .github/metrics/budgets.env
  --primary-binary <path>   Main install binary used by install.sh.
EOF
}

MODE=""
VERSION="dev"
DIST_DIR="dist"
OUTPUT_DIR="ci-metrics"
BUDGET_FILE=".github/metrics/budgets.env"
PRIMARY_BINARY=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)
      MODE="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --dist-dir)
      DIST_DIR="${2:-}"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="${2:-}"
      shift 2
      ;;
    --budget-file)
      BUDGET_FILE="${2:-}"
      shift 2
      ;;
    --primary-binary)
      PRIMARY_BINARY="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$MODE" || -z "$PRIMARY_BINARY" ]]; then
  usage >&2
  exit 1
fi

if [[ "$MODE" != "ci" && "$MODE" != "release" ]]; then
  echo "--mode must be 'ci' or 'release'" >&2
  exit 1
fi

if [[ ! -f "$BUDGET_FILE" ]]; then
  echo "Budget file not found: $BUDGET_FILE" >&2
  exit 1
fi

if [[ ! -f "$PRIMARY_BINARY" ]]; then
  echo "Primary binary not found: $PRIMARY_BINARY" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$BUDGET_FILE"

mkdir -p "$OUTPUT_DIR"

SUMMARY_FILE="$OUTPUT_DIR/summary.md"
JSON_FILE="$OUTPUT_DIR/metrics.json"
METRICS_FILE="$OUTPUT_DIR/metrics.tsv"
RUNTIME_LOG="$OUTPUT_DIR/runtime.log"

: > "$METRICS_FILE"

to_mib() {
  awk -v value="$1" 'BEGIN { printf "%.2f", value / 1024 / 1024 }'
}

timestamp_ms() {
  local ts
  ts="$(date +%s%3N 2>/dev/null || true)"
  if [[ "$ts" =~ ^[0-9]+$ ]]; then
    printf "%s\n" "$ts"
    return
  fi
  perl -MTime::HiRes=time -e 'printf "%.0f\n", time() * 1000'
}

bytes_from_mib() {
  awk -v value="$1" 'BEGIN { printf "%.0f", value * 1024 * 1024 }'
}

file_size_bytes() {
  if stat -c '%s' "$1" >/dev/null 2>&1; then
    stat -c '%s' "$1"
  else
    stat -f '%z' "$1"
  fi
}

dir_size_bytes() {
  if du -sb "$1" >/dev/null 2>&1; then
    du -sb "$@" | awk '{sum += $1} END {print sum + 0}'
  else
    du -sk "$@" | awk '{sum += $1} END {print (sum + 0) * 1024}'
  fi
}

random_port() {
  if command -v shuf >/dev/null 2>&1; then
    shuf -i 20000-40000 -n 1
  else
    jot -r 1 20000 40000
  fi
}

sum_files_from_stdin() {
  local total=0 file
  while IFS= read -r file; do
    [[ -n "$file" ]] || continue
    total=$((total + $(file_size_bytes "$file")))
  done
  printf "%s\n" "$total"
}

format_value() {
  local value="$1"
  local unit="$2"

  case "$unit" in
    bytes)
      printf "%s MiB" "$(to_mib "$value")"
      ;;
    kib)
      awk -v kib="$value" 'BEGIN { printf "%.2f MiB", kib / 1024 }'
      ;;
    ms)
      printf "%s ms" "$value"
      ;;
    count)
      printf "%s" "$value"
      ;;
    lines)
      printf "%s" "$value"
      ;;
    *)
      printf "%s %s" "$value" "$unit"
      ;;
  esac
}

budget_for_metric() {
  local key="$1"
  case "$key" in
    install_binary_bytes) printf "%s" "$(bytes_from_mib "$BUDGET_INSTALL_BINARY_MIB")" ;;
    release_total_bytes) printf "%s" "$(bytes_from_mib "$BUDGET_RELEASE_TOTAL_MIB")" ;;
    tracked_repo_bytes) printf "%s" "$(bytes_from_mib "$BUDGET_TRACKED_REPO_MIB")" ;;
    checkout_disk_bytes) printf "%s" "$(bytes_from_mib "$BUDGET_CHECKOUT_DISK_MIB")" ;;
    git_history_bytes) printf "%s" "$(bytes_from_mib "$BUDGET_GIT_HISTORY_MIB")" ;;
    first_run_disk_bytes) printf "%s" "$(bytes_from_mib "$BUDGET_FIRST_RUN_DISK_MIB")" ;;
    idle_rss_kib) printf "%s" "$(awk -v mib="$BUDGET_IDLE_RSS_MIB" 'BEGIN { printf "%.0f", mib * 1024 }')" ;;
    startup_ms) printf "%s" "$BUDGET_STARTUP_MS" ;;
    template_catalog_bytes) printf "%s" "$(bytes_from_mib "$BUDGET_TEMPLATE_CATALOG_MIB")" ;;
    *) printf "" ;;
  esac
}

add_metric() {
  local key="$1"
  local label="$2"
  local value="$3"
  local unit="$4"
  local note="$5"
  local budget status budget_display

  budget="$(budget_for_metric "$key")"

  if [[ -n "$budget" ]]; then
    budget_display="$(format_value "$budget" "$unit")"
    if awk -v value="$value" -v budget="$budget" 'BEGIN { exit(value <= budget ? 0 : 1) }'; then
      status="OK"
    else
      status="WARN"
    fi
  else
    budget_display="n/a"
    status="INFO"
  fi

  printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\n" \
    "$key" "$label" "$value" "$unit" "$budget_display" "$status" "$note" >> "$METRICS_FILE"
}

sum_tracked_bytes() {
  local total=0 file
  while IFS= read -r -d '' file; do
    total=$((total + $(file_size_bytes "$file")))
  done < <(git ls-files -z)
  printf "%s\n" "$total"
}

measure_runtime() {
  local temp_root config_dir data_dir config_file port start_ms end_ms pid ready rss_kib disk_bytes

  temp_root="$(mktemp -d)"
  config_dir="$temp_root/config"
  data_dir="$temp_root/data"
  config_file="$config_dir/config.yaml"
  port="${VESSEL_METRICS_PORT:-$(random_port)}"

  mkdir -p "$config_dir" "$data_dir"

  cleanup_runtime() {
    if [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
    rm -rf "$temp_root"
  }

  trap cleanup_runtime EXIT

  start_ms="$(timestamp_ms)"
  VESSEL_TEMPLATE_CATALOG_DISABLED=1 \
  VESSEL_CONFIG="$config_file" \
  VESSEL_DATA_DIR="$data_dir" \
  VESSEL_PORT="$port" \
    "$PRIMARY_BINARY" serve > "$RUNTIME_LOG" 2>&1 &
  pid=$!

  ready=0
  for _ in $(seq 1 60); do
    if ! kill -0 "$pid" 2>/dev/null; then
      break
    fi
    if curl --silent --fail "http://127.0.0.1:${port}/api/v1/health" > /dev/null; then
      ready=1
      break
    fi
    sleep 0.5
  done

  end_ms="$(timestamp_ms)"
  STARTUP_MS=$((end_ms - start_ms))

  if [[ "$ready" -ne 1 ]]; then
    echo "Failed to start Vessel for runtime metrics. Recent output:" >&2
    tail -n 50 "$RUNTIME_LOG" >&2 || true
    exit 1
  fi

  sleep 1
  rss_kib="$(ps -o rss= -p "$pid" | awk '{print $1 + 0}')"
  disk_bytes="$(dir_size_bytes "$config_dir" "$data_dir")"

  IDLE_RSS_KIB="$rss_kib"
  FIRST_RUN_DISK_BYTES="$disk_bytes"

  trap - EXIT
  cleanup_runtime
}

INSTALL_BINARY_BYTES="$(file_size_bytes "$PRIMARY_BINARY")"
TRACKED_REPO_BYTES="$(sum_tracked_bytes)"
CHECKOUT_DISK_BYTES="$(dir_size_bytes .)"
GIT_HISTORY_BYTES="$(dir_size_bytes .git)"
TEMPLATE_COUNT="$(find internal/registry/templates -type f -name '*.yaml' | wc -l | tr -d ' ')"
TEMPLATE_CATALOG_BYTES="$(find internal/registry/templates -type f -name '*.yaml' | sort | sum_files_from_stdin)"
GO_SUM_LINES="$(wc -l < go.sum | tr -d ' ')"

measure_runtime

add_metric "install_binary_bytes" "Install binary size (linux/amd64)" "$INSTALL_BINARY_BYTES" "bytes" "Binary downloaded by install.sh on x86_64 Linux."
add_metric "tracked_repo_bytes" "Tracked repository size" "$TRACKED_REPO_BYTES" "bytes" "Sum of tracked source assets."
add_metric "checkout_disk_bytes" "Checkout disk usage" "$CHECKOUT_DISK_BYTES" "bytes" "Full workspace size in CI checkout."
add_metric "git_history_bytes" "Git history size" "$GIT_HISTORY_BYTES" "bytes" "Current .git directory size."
add_metric "first_run_disk_bytes" "First-run disk footprint" "$FIRST_RUN_DISK_BYTES" "bytes" "Config + data written after a clean boot."
add_metric "idle_rss_kib" "Idle RSS after boot" "$IDLE_RSS_KIB" "kib" "Approximate RAM used after health endpoint becomes ready."
add_metric "startup_ms" "Startup to healthy" "$STARTUP_MS" "ms" "Binary start until /api/v1/health responds."
add_metric "template_catalog_bytes" "Bundled template catalog size" "$TEMPLATE_CATALOG_BYTES" "bytes" "Total bytes of shipped YAML templates."
add_metric "template_catalog_count" "Bundled template count" "$TEMPLATE_COUNT" "count" "Number of built-in templates."
add_metric "go_sum_lines" "go.sum line count" "$GO_SUM_LINES" "lines" "Dependency lockfile growth signal."

if [[ -d "$DIST_DIR" ]]; then
  RELEASE_TOTAL_BYTES="$(find "$DIST_DIR" -maxdepth 1 -type f -name 'vessel_linux_*' | sort | sum_files_from_stdin)"
  if [[ "$RELEASE_TOTAL_BYTES" -gt 0 ]]; then
    add_metric "release_total_bytes" "Total release binary payload" "$RELEASE_TOTAL_BYTES" "bytes" "Sum of all linux release binaries in dist/."
    while IFS= read -r artifact; do
      artifact_name="$(basename "$artifact")"
      artifact_size="$(file_size_bytes "$artifact")"
      add_metric "artifact_${artifact_name}" "Release artifact ${artifact_name}" "$artifact_size" "bytes" "Per-architecture release binary."
    done < <(find "$DIST_DIR" -maxdepth 1 -type f -name 'vessel_linux_*' | sort)
  fi
fi

{
  printf '# Vessel Metrics Report\n\n'
  printf -- '- Mode: `%s`\n' "$MODE"
  printf -- '- Version: `%s`\n' "$VERSION"
  printf -- '- Generated: `%s`\n\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  printf '| Metric | Current | Budget | Status | Notes |\n'
  printf '| --- | ---: | ---: | --- | --- |\n'
  while IFS=$'\t' read -r key label value unit budget status note; do
    printf "| %s | %s | %s | %s | %s |\n" \
      "$label" "$(format_value "$value" "$unit")" "$budget" "$status" "$note"
  done < "$METRICS_FILE"
} > "$SUMMARY_FILE"

{
  printf '{\n'
  printf '  "mode": "%s",\n' "$MODE"
  printf '  "version": "%s",\n' "$VERSION"
  printf '  "generated_at": "%s",\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  printf '  "metrics": [\n'

  first=1
  while IFS=$'\t' read -r key label value unit budget status note; do
    if [[ "$first" -eq 0 ]]; then
      printf ',\n'
    fi
    first=0
    printf '    {"key":"%s","label":"%s","value":"%s","unit":"%s","budget":"%s","status":"%s","note":"%s"}' \
      "$key" "$label" "$value" "$unit" "$budget" "$status" "$note"
  done < "$METRICS_FILE"

  printf '\n  ]\n'
  printf '}\n'
} > "$JSON_FILE"

printf 'Metrics written to %s and %s\n' "$SUMMARY_FILE" "$JSON_FILE"
