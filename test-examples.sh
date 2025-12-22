#!/bin/bash

# Test script to validate and optionally apply Terraform examples
# Usage: ./test-examples.sh [--apply] [--examples "example1 example2"]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="${SCRIPT_DIR}/examples"
APPLY=false
SPECIFIC_EXAMPLES=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --apply)
      APPLY=true
      shift
      ;;
    --examples)
      SPECIFIC_EXAMPLES="$2"
      shift
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--apply] [--examples \"example1 example2\"]"
      echo ""
      echo "Options:"
      echo "  --apply     Also run terraform apply (default: validate and plan only)"
      echo "  --examples  Space-separated list of specific examples to test"
      echo "  -h, --help  Show this help message"
      echo ""
      echo "Examples:"
      echo "  $0                                    # Test all examples (validate + plan)"
      echo "  $0 --apply                           # Test all examples (validate + plan + apply)"
      echo "  $0 --examples \"postgres endpoints\"  # Test specific examples"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use -h or --help for usage information"
      exit 1
      ;;
  esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

# Function to test a single example
test_example() {
  local example_dir="$1"
  local example_name="$(basename "$example_dir")"

  log_info "Testing example: $example_name"

  # Check if directory exists and contains main.tf
  if [[ ! -f "$example_dir/main.tf" ]]; then
    log_warning "Skipping $example_name: no main.tf found"
    return 1
  fi

  # Change to example directory
  cd "$example_dir"

  # Initialize Terraform
  log_info "  Running terraform init..."
  if ! terraform init -upgrade; then
    log_error "  terraform init failed for $example_name"
    cd "$SCRIPT_DIR"
    return 1
  fi

  # Validate configuration
  log_info "  Running terraform validate..."
  if ! terraform validate >/dev/null 2>&1; then
    log_error "  terraform validate failed for $example_name"
    cd "$SCRIPT_DIR"
    return 1
  fi

  # Plan configuration
  log_info "  Running terraform plan..."
  if ! terraform plan -out=tfplan >/dev/null 2>&1; then
    log_error "  terraform plan failed for $example_name"
    cd "$SCRIPT_DIR"
    return 1
  fi

  # Optionally apply
  if [[ "$APPLY" == "true" ]]; then
    log_info "  Running terraform apply..."
    if ! terraform apply -auto-approve tfplan; then
      log_error "  terraform apply failed for $example_name"
      cd "$SCRIPT_DIR"
      return 1
    fi
    log_success "  Applied $example_name successfully"
  else
    log_success "  Planned $example_name successfully"
  fi

  # Clean up plan file
  rm -f tfplan

  # Return to script directory
  cd "$SCRIPT_DIR"
  return 0
}

# Main execution
log_info "Starting Terraform examples test"
log_info "Apply mode: $APPLY"

# Get list of examples to test
if [[ -n "$SPECIFIC_EXAMPLES" ]]; then
  # Test specific examples
  examples_to_test=($SPECIFIC_EXAMPLES)
else
  # Test all examples (directories in examples/ that contain main.tf)
  examples_to_test=()
  for dir in "$EXAMPLES_DIR"/*/; do
    if [[ -f "$dir/main.tf" ]]; then
      examples_to_test+=("$(basename "$dir")")
    fi
  done
fi

log_info "Examples to test: ${examples_to_test[*]}"

# Track results
total_examples=${#examples_to_test[@]}
passed_examples=0
failed_examples=0

# Test each example
for example in "${examples_to_test[@]}"; do
  example_dir="$EXAMPLES_DIR/$example"
  if [[ -d "$example_dir" ]]; then
    if test_example "$example_dir"; then
      ((passed_examples++))
    else
      ((failed_examples++))
    fi
  else
    log_error "Example directory not found: $example_dir"
    ((failed_examples++))
  fi
done

# Summary
echo
log_info "Test Summary:"
log_info "  Total examples: $total_examples"
log_success "  Passed: $passed_examples"
if [[ $failed_examples -gt 0 ]]; then
  log_error "  Failed: $failed_examples"
  exit 1
else
  log_success "  All examples passed!"
fi