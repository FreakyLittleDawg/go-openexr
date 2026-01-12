#!/bin/bash
#
# Download external test data for go-openexr integration tests.
#
# Usage:
#   ./download.sh           # Download all test data
#   ./download.sh --check   # Check if test data exists (exit 0 if yes, 1 if no)
#   ./download.sh --clean   # Remove downloaded test data
#
# This script downloads test files from external sources. These files are
# excluded from git via .gitignore and must be downloaded before running
# certain integration tests.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Cryptomatte sample files from official Psyop repository
CRYPTOMATTE_BASE_URL="https://raw.githubusercontent.com/Psyop/Cryptomatte/master/sample_images"
CRYPTOMATTE_FILES=(
    "bunny_CryptoObject.exr"
    "bunny_CryptoMaterial.exr"
    "bunny_CryptoAsset.exr"
    "testGrid_CryptoObject.exr"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_dependencies() {
    if ! command -v curl &> /dev/null; then
        error "curl is required but not installed"
        exit 1
    fi
}

download_cryptomatte() {
    info "Downloading Cryptomatte test files..."
    mkdir -p cryptomatte

    local success=0
    local failed=0

    for file in "${CRYPTOMATTE_FILES[@]}"; do
        local url="${CRYPTOMATTE_BASE_URL}/${file}"
        local dest="cryptomatte/${file}"

        if [[ -f "$dest" ]]; then
            info "  $file (already exists, skipping)"
            ((success++))
            continue
        fi

        echo -n "  Downloading $file... "
        if curl -sL "$url" -o "$dest" 2>/dev/null; then
            # Verify it's a valid EXR (magic number: 0x762f3101)
            if head -c 4 "$dest" | xxd -p | grep -q "762f3101"; then
                echo "OK"
                ((success++))
            else
                echo "FAILED (invalid EXR)"
                rm -f "$dest"
                ((failed++))
            fi
        else
            echo "FAILED"
            rm -f "$dest"
            ((failed++))
        fi
    done

    info "Cryptomatte: $success downloaded, $failed failed"
    return $failed
}

check_files() {
    local missing=0

    info "Checking for test data files..."

    echo "Cryptomatte files:"
    for file in "${CRYPTOMATTE_FILES[@]}"; do
        local dest="cryptomatte/${file}"
        if [[ -f "$dest" ]]; then
            echo -e "  ${GREEN}✓${NC} $file"
        else
            echo -e "  ${RED}✗${NC} $file (missing)"
            ((missing++))
        fi
    done

    if [[ $missing -eq 0 ]]; then
        info "All test data files present"
        return 0
    else
        warn "$missing file(s) missing. Run './download.sh' to download."
        return 1
    fi
}

clean_files() {
    info "Removing downloaded test data..."

    rm -rf cryptomatte/*.exr

    info "Done"
}

show_help() {
    cat << EOF
Download external test data for go-openexr integration tests.

Usage:
  ./download.sh           Download all test data
  ./download.sh --check   Check if test data exists
  ./download.sh --clean   Remove downloaded test data
  ./download.sh --help    Show this help message

Test data sources:
  - Cryptomatte: https://github.com/Psyop/Cryptomatte/tree/master/sample_images

Downloaded files are excluded from git via .gitignore.
EOF
}

# Main
case "${1:-}" in
    --check)
        check_files
        ;;
    --clean)
        clean_files
        ;;
    --help|-h)
        show_help
        ;;
    "")
        check_dependencies
        download_cryptomatte
        echo ""
        check_files
        ;;
    *)
        error "Unknown option: $1"
        show_help
        exit 1
        ;;
esac
