#!/usr/bin/env bash
set -euo pipefail

VERBOSE=0
REBUILD_ONLY=0
JSON_OUTPUT=0
DOCKER_MODE=0
DOCKER_IMAGE="codestylechecker"

IN_CONTAINER=0
if [[ -x "/app/bin/check_style" ]]; then
  IN_CONTAINER=1
fi

print_usage() {
  cat <<EOF
Usage:
  $0 [options] <style: kr|allman> <file.c|directory>
Options:
  -h, --help            Show this help message and exit
  -v, --verbose         Enable verbose output
  -r, --rebuild-only    Only (re)build the Go binary; do not run checks
  -j, --json            Emit pretty JSON of each error (written to ./out/errors_<style>_<date>_<time>.json)
  --docker              Run the analysis inside a Docker container (mounting the target file/dir into /work)
EOF
}

# ----------------------- Parse options -----------------------
while [[ $# -gt 0 && "$1" == -* ]]; do
  case "$1" in
    -h|--help)         print_usage; exit 0 ;;
    -v|--verbose)      VERBOSE=1; shift ;;
    -r|--rebuild-only) REBUILD_ONLY=1; shift ;;
    -j|--json)         JSON_OUTPUT=1; shift ;;
    --docker)          DOCKER_MODE=1; shift ;;
    --)                shift; break ;;
    *) echo "Unknown option: $1" >&2; print_usage; exit 1 ;;
  esac
done

if [[ $# -lt 2 ]]; then
  echo "Error: expected style + file_or_directory" >&2
  print_usage
  exit 1
fi

STYLE="$1"; TARGET="$2"

if [[ "$STYLE" != "kr" && "$STYLE" != "allman" ]]; then
  echo "Error: style must be 'kr' or 'allman'" >&2
  exit 1
fi

# ----------------------- Docker mode (host) -----------------------
if (( DOCKER_MODE )) && (( IN_CONTAINER == 0 )); then
  if ! docker image inspect "$DOCKER_IMAGE" >/dev/null 2>&1; then
    echo "[INFO] Docker image '$DOCKER_IMAGE' not found. Building..."
    docker build --load -t "$DOCKER_IMAGE" --build-arg UID=$(id -u) --build-arg GID=$(id -g) .
  fi

  if command -v realpath >/dev/null 2>&1; then
    TARGET_ABS="$(realpath "$TARGET")"
  else
    TARGET_ABS="$(readlink -f "$TARGET")"
  fi

  DOCKER_VOLUMES=(-v "$TARGET_ABS:/work:ro")
  if (( JSON_OUTPUT )); then
    mkdir -p out
    DOCKER_VOLUMES+=(-v "$(pwd)/out:/app/out")
  fi

  CMD_ARGS=()
  (( VERBOSE )) && CMD_ARGS+=("-v")
  (( JSON_OUTPUT )) && CMD_ARGS+=("-j")

  docker run --rm \
    "${DOCKER_VOLUMES[@]}" \
    "$DOCKER_IMAGE" \
    "${CMD_ARGS[@]}" "$STYLE" "/work"

  exit 0
fi

# ----------------------- Local/Container mode -----------------------
SRC=./src/check_style.go
BIN_DIR=./bin
BIN="$BIN_DIR/check_style"

if (( IN_CONTAINER )); then
  BIN="/app/bin/check_style"
else
  command -v go >/dev/null || { echo "Error: Go not found." >&2; exit 1; }
  mkdir -p "$BIN_DIR"
  if [[ $REBUILD_ONLY -eq 1 || ! -x "$BIN" || "$SRC" -nt "$BIN" ]]; then
    (( VERBOSE )) && echo "Building checker..."
    go build -o "$BIN" "$SRC"
    [[ $REBUILD_ONLY -eq 1 ]] && exit 0
  elif (( VERBOSE )); then
    echo "Using existing checker binary"
  fi
fi

# ----------------------- Collect files -----------------------
if [[ -d "$TARGET" ]]; then
  mapfile -t files < <(find "$TARGET" -type f -name '*.c')
elif [[ -f "$TARGET" ]]; then
  files=("$TARGET")
else
  echo "Error: '$TARGET' not found" >&2
  exit 1
fi

# ----------------------- JSON output init -----------------------
if (( JSON_OUTPUT )); then
  mkdir -p out
  DATE=$(date +"%Y%m%d")
  TIME=$(date +"%H%M%S")
  OUT="./out/errors_${STYLE}_${DATE}_${TIME}.json"
  echo "[" > "$OUT"
  first=1
fi

# ----------------------- Process files -----------------------
for file in "${files[@]}"; do
  (( VERBOSE )) && echo "Checking $file..."
  raw="$("$BIN" --style="$STYLE" "$file" 2>&1 || true)"
  cleaned=$(printf '%s\n' "$raw" | sed -r 's/\x1B\[[0-9;]*[mK]//g;1,/^[-]{5,}$/d')

  state=0
  errnum=0; level=""; errmsg=""; lineno=0; colno=0

  while IFS= read -r line; do
    if [[ $line =~ ^#([0-9]+)\ \[([A-Z]+)\]:[[:space:]]*(.*)$ ]]; then
      errnum=${BASH_REMATCH[1]}
      level=${BASH_REMATCH[2]}
      errmsg=${BASH_REMATCH[3]}
      state=1
      continue
    fi

    if (( state == 1 )) && [[ $line =~ ^[^[:space:]]+:[0-9]+:[0-9]+$ ]]; then
      lineno=$(cut -d: -f2 <<<"$line")
      colno=$(cut -d: -f3 <<<"$line")
      state=2
    fi

    if (( state == 2 )); then
      if (( JSON_OUTPUT )); then
        (( first == 0 )) && echo "," >> "$OUT"
        first=0
        esc=$(printf '%s' "$errmsg" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g')
        {
          echo "  {"
          echo "    \"error_num\": $errnum,"
          echo "    \"file\": \"$file\","
          echo "    \"level\": \"$level\","
          echo "    \"line\": $lineno,"
          echo "    \"column\": $colno,"
          echo "    \"error_msg\": \"$esc\""
          echo -n "  }"
        } >> "$OUT"
      fi
      state=0
    fi
  done <<< "$cleaned"
done

# ----------------------- Close JSON -----------------------
if (( JSON_OUTPUT )); then
  echo "" >> "$OUT"
  echo "]" >> "$OUT"
  echo "Written pretty JSON errors to $OUT"
  exit 0
fi

# ----------------------- Normal output -----------------------
fail=0
for file in "${files[@]}"; do
  "$BIN" --style="$STYLE" "$file" || fail=1
done
exit $fail
