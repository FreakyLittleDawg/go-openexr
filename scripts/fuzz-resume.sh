#!/bin/bash
# Resume fuzz testing from interrupted run
# Picks up failed tests (now fixed) and continues where we left off

set -e

LOGDIR="fuzz-logs"
mkdir -p "$LOGDIR"
LOGFILE="$LOGDIR/fuzz-resume-$(date +%Y%m%d-%H%M%S).log"

log() {
    echo "$1" | tee -a "$LOGFILE"
}

run_fuzz() {
    local name=$1
    local pkg=$2
    local duration=$3

    log "----------------------------------------"
    log "[$name] Starting at $(date) (duration: $duration)"

    if timeout "$duration" go test -fuzz="$name" -fuzztime="$duration" "./$pkg/..." >> "$LOGFILE" 2>&1; then
        log "[$name] PASSED"
    else
        local exit_code=$?
        if [ $exit_code -eq 124 ]; then
            log "[$name] PASSED (timeout)"
        else
            log "[$name] FAILED (exit code: $exit_code)"
        fi
    fi
}

log "=== go-openexr Fuzz Testing Resume ==="
log "Started: $(date)"
log "Log file: $LOGFILE"
log ""

# Re-run previously failed tests (bug now fixed) - full 1h each
log "=== Re-running Previously Failed Tests (1h each) ==="
run_fuzz "FuzzScanlineReader" "exr" "1h"
run_fuzz "FuzzTiledReader" "exr" "1h"
run_fuzz "FuzzAttributeValue" "exr" "1h"

# Finish RLEDecompress (was at 54m30s, run remaining ~6m)
log ""
log "=== Finishing Interrupted Test ==="
run_fuzz "FuzzRLEDecompress" "compression" "6m"

# Continue with remaining high-priority tests
log ""
log "=== Remaining High Priority Tests (1h each) ==="
run_fuzz "FuzzZIPDecompress" "compression" "1h"
run_fuzz "FuzzPIZDecompress" "compression" "1h"
run_fuzz "FuzzPXR24Decompress" "compression" "1h"
run_fuzz "FuzzB44Decompress" "compression" "1h"
run_fuzz "FuzzDWADecompress" "compression" "1h"

# Medium priority (30m each)
log ""
log "=== Medium Priority Tests (30m each) ==="
run_fuzz "FuzzRLERoundtrip" "compression" "30m"
run_fuzz "FuzzZIPRoundtrip" "compression" "30m"
run_fuzz "FuzzPIZRoundtrip" "compression" "30m"
run_fuzz "FuzzInterleave" "compression" "30m"
run_fuzz "FuzzXDRString" "internal/xdr" "30m"
run_fuzz "FuzzXDRRead" "internal/xdr" "30m"
run_fuzz "FuzzXDRWrite" "internal/xdr" "30m"
run_fuzz "FuzzPredictor" "internal/predictor" "30m"

# Low priority (15m each)
log ""
log "=== Low Priority Tests (15m each) ==="
run_fuzz "FuzzHalfToFloat" "half" "15m"
run_fuzz "FuzzFloatToHalf" "half" "15m"
run_fuzz "FuzzHalfRoundtrip" "half" "15m"
run_fuzz "FuzzParseManifest" "exrid" "15m"
run_fuzz "FuzzManifestRoundtrip" "exrid" "15m"

log ""
log "=== Fuzz Testing Resume Complete ==="
log "Finished: $(date)"

# Summary
log ""
log "=== Summary ==="
grep -E "^\[Fuzz.*\] (PASSED|FAILED)" "$LOGFILE" | sort | uniq -c
