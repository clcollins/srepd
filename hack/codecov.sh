#!/bin/bash

# filepath: /home/chcollin/Projects/github.com/clcollins/srepd/hack/codecov.sh

set -euo pipefail

# Ensure the script is run from the project root
PROJECT_ROOT=$(git rev-parse --show-toplevel)
cd "$PROJECT_ROOT"

# Output file for coverage
COVERAGE_FILE="coverage.out"

# Generate test coverage report
echo "Generating test coverage report..."
go test ./... -coverprofile="$COVERAGE_FILE" -covermode=atomic

# Check if the coverage file was generated
if [ ! -f "$COVERAGE_FILE" ]; then
    echo "Error: Coverage file not generated."
    exit 1
fi

# Display coverage summary
echo "Coverage summary:"
go tool cover -func="$COVERAGE_FILE"

# Upload coverage to Codecov if the codecov CLI is installed
if command -v codecov >/dev/null 2>&1; then
    echo "Uploading coverage report to Codecov..."
    codecov -f "$COVERAGE_FILE"
else
    echo "Codecov CLI not found. Skipping upload."
    echo "You can install it from https://docs.codecov.io/docs/codecov-cli."
fi

# Clean up
echo "Cleaning up..."
rm -f "$COVERAGE_FILE"

echo "Test coverage report generation complete."
