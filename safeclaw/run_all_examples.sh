#!/bin/bash
# run_all_examples.sh - Run all SafeClaw examples
#
# Usage:
#   ./run_all_examples.sh           # Run all examples
#   ./run_all_examples.sh 01        # Run specific example (01_hello)
#   ./run_all_examples.sh 01 03 05  # Run multiple examples

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Function to run a single example
run_example() {
    local example_num=$1
    
    # Find the example directory
    local example_dir=$(find examples -maxdepth 1 -type d -name "${example_num}_*" | head -n 1)
    
    if [ -z "$example_dir" ] || [ ! -d "$example_dir" ]; then
        echo -e "${RED}❌ Example ${example_num} not found${NC}"
        return 1
    fi
    
    local example_name=$(basename "$example_dir")
    
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Running: ${example_name}${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    if (cd "$example_dir" && go run main.go); then
        echo -e "${GREEN}✅ ${example_name} completed successfully${NC}"
        return 0
    else
        echo -e "${RED}❌ ${example_name} failed${NC}"
        return 1
    fi
}

# Main execution
echo -e "${YELLOW}SafeClaw Examples Runner${NC}"
echo -e "${YELLOW}========================${NC}\n"

# Check if specific examples were requested
if [ $# -eq 0 ]; then
    # Run all examples
    echo -e "${BLUE}Running all 10 examples...${NC}\n"
    
    failed_examples=()
    for i in 01 02 03 04 05 06 07 08 09 10; do
        if ! run_example "$i"; then
            failed_examples+=("$i")
        fi
        echo ""
    done
    
    # Summary
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Summary${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    if [ ${#failed_examples[@]} -eq 0 ]; then
        echo -e "${GREEN}✅ All 10 examples completed successfully!${NC}"
        exit 0
    else
        echo -e "${RED}❌ ${#failed_examples[@]} example(s) failed: ${failed_examples[*]}${NC}"
        exit 1
    fi
else
    # Run specific examples
    echo -e "${BLUE}Running ${#@} example(s)...${NC}\n"
    
    failed_examples=()
    for example_num in "$@"; do
        # Pad to 2 digits if needed
        example_num=$(printf "%02d" "$example_num")
        
        if ! run_example "$example_num"; then
            failed_examples+=("$example_num")
        fi
        echo ""
    done
    
    # Summary
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Summary${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    if [ ${#failed_examples[@]} -eq 0 ]; then
        echo -e "${GREEN}✅ All requested examples completed successfully!${NC}"
        exit 0
    else
        echo -e "${RED}❌ ${#failed_examples[@]} example(s) failed: ${failed_examples[*]}${NC}"
        exit 1
    fi
fi
