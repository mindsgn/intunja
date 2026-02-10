#!/bin/bash
# Build script for Intunja client

set -e

PROJECT_NAME="intunja"
VERSION="0.1.0"

echo "🔨 Building Intunja Client v${VERSION}"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check Go installation
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

echo -e "${GREEN}✓${NC} Go version: $(go version)"

# Build flags
BUILD_FLAGS=(-ldflags "-s -w")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

function build_desktop() {
    echo ""
    echo "📦 Building desktop executable..."
    
    # Build for current platform
    go build ${BUILD_FLAGS} -o ${PROJECT_NAME} ./cmd/intunja
    
    echo -e "${GREEN}✓${NC} Built: ./${PROJECT_NAME}"
    echo ""
    echo "Run with: ./${PROJECT_NAME} <torrent-file>"
}

function build_all_platforms() {
    echo ""
    echo "📦 Building for all platforms..."
    
    mkdir -p dist
    
    # Linux amd64
    echo "  Building linux/amd64..."
    GOOS=linux GOARCH=amd64 go build ${BUILD_FLAGS} -o dist/${PROJECT_NAME}-linux-amd64 ./cmd/intunja
    
    # Linux arm64
    echo "  Building linux/arm64..."
    GOOS=linux GOARCH=arm64 go build ${BUILD_FLAGS} -o dist/${PROJECT_NAME}-linux-arm64 ./cmd/intunja
    
    # macOS amd64
    echo "  Building darwin/amd64..."
    GOOS=darwin GOARCH=amd64 go build ${BUILD_FLAGS} -o dist/${PROJECT_NAME}-darwin-amd64 ./cmd/intunja
    
    # macOS arm64 (M1/M2)
    echo "  Building darwin/arm64..."
    GOOS=darwin GOARCH=arm64 go build ${BUILD_FLAGS} -o dist/${PROJECT_NAME}-darwin-arm64 ./cmd/intunja
    
    # Windows amd64
    echo "  Building windows/amd64..."
    GOOS=windows GOARCH=amd64 go build ${BUILD_FLAGS} -o dist/${PROJECT_NAME}-windows-amd64.exe ./cmd/intunja
    
    echo -e "${GREEN}✓${NC} Built all platforms in dist/"
}

function build_android() {
    echo ""
    echo "📱 Building for Android..."
    
    # Check gomobile installation
    if ! command -v gomobile &> /dev/null; then
        echo -e "${YELLOW}⚠${NC} gomobile not found. Installing..."
        go install golang.org/x/mobile/cmd/gomobile@latest
        gomobile init
    fi
    
    mkdir -p dist
    
    echo "  Building Android AAR..."
    gomobile bind -target=android -o dist/intunja.aar ./mobile
    
    echo -e "${GREEN}✓${NC} Built: dist/intunja.aar"
    echo ""
    echo "Import into Android Studio:"
    echo "  1. Copy dist/intunja.aar to app/libs/"
    echo "  2. Add to build.gradle: implementation(name: 'intunja', ext: 'aar')"
}

function build_ios() {
    echo ""
    echo "📱 Building for iOS..."
    
    # Check gomobile installation
    if ! command -v gomobile &> /dev/null; then
        echo -e "${YELLOW}⚠${NC} gomobile not found. Installing..."
        go install golang.org/x/mobile/cmd/gomobile@latest
        gomobile init
    fi
    
    mkdir -p dist
    
    echo "  Building iOS framework..."
    gomobile bind -target=ios -o dist/Intunja.framework ./mobile
    
    echo -e "${GREEN}✓${NC} Built: dist/Intunja.framework"
    echo ""
    echo "Import into Xcode:"
    echo "  1. Drag dist/intunja.framework into your project"
    echo "  2. Ensure it's added to 'Frameworks, Libraries, and Embedded Content'"
}

function run_tests() {
    echo ""
    echo "🧪 Running tests..."
    
    go test ./... -v
    
    echo -e "${GREEN}✓${NC} All tests passed"
}

function clean() {
    echo ""
    echo "🧹 Cleaning build artifacts..."
    
    rm -rf dist/
    rm -f ${PROJECT_NAME}
    rm -f ${PROJECT_NAME}-*
    
    echo -e "${GREEN}✓${NC} Cleaned"
}

# Parse command line arguments
case "${1}" in
    "desktop")
        build_desktop
        ;;
    "all")
        build_all_platforms
        ;;
    "android")
        build_android
        ;;
    "ios")
        build_ios
        ;;
    "mobile")
        build_android
        build_ios
        ;;
    "test")
        run_tests
        ;;
    "clean")
        clean
        ;;
    *)
        echo "Usage: $0 {desktop|all|android|ios|mobile|test|clean}"
        echo ""
        echo "Commands:"
        echo "  desktop  - Build for current platform"
        echo "  all      - Build for all desktop platforms"
        echo "  android  - Build Android AAR package"
        echo "  ios      - Build iOS framework"
        echo "  mobile   - Build both Android and iOS"
        echo "  test     - Run tests"
        echo "  clean    - Remove build artifacts"
        exit 1
        ;;
esac