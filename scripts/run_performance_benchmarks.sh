#!/bin/bash

# Performance Benchmarks Runner for Widget Search Filters
# This script runs comprehensive benchmarks and generates performance reports

set -e

echo "🚀 Starting Widget Search Filters Performance Benchmarks"
echo "========================================================="

# Create results directory
RESULTS_DIR="benchmark_results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
REPORT_DIR="${RESULTS_DIR}/${TIMESTAMP}"

mkdir -p "${REPORT_DIR}"

echo "📊 Results will be saved to: ${REPORT_DIR}"

# Function to run benchmarks and save results
run_benchmark() {
    local package=$1
    local name=$2
    local output_file="${REPORT_DIR}/${name}_benchmark.txt"
    
    echo "🔄 Running ${name} benchmarks..."
    
    # Run benchmark with memory profiling
    go test -bench=. -benchmem -count=3 -timeout=30m "./${package}" > "${output_file}" 2>&1
    
    if [ $? -eq 0 ]; then
        echo "✅ ${name} benchmarks completed successfully"
    else
        echo "❌ ${name} benchmarks failed"
        cat "${output_file}"
    fi
}

# Function to run CPU profiling benchmark
run_cpu_profile() {
    local package=$1
    local benchmark=$2
    local name=$3
    local output_file="${REPORT_DIR}/${name}_cpu.prof"
    
    echo "🔄 Running CPU profiling for ${name}..."
    
    go test -bench="${benchmark}" -cpuprofile="${output_file}" -count=1 "./${package}" > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        echo "✅ CPU profiling for ${name} completed"
    else
        echo "❌ CPU profiling for ${name} failed"
    fi
}

# Function to run memory profiling benchmark
run_mem_profile() {
    local package=$1
    local benchmark=$2
    local name=$3
    local output_file="${REPORT_DIR}/${name}_mem.prof"
    
    echo "🔄 Running memory profiling for ${name}..."
    
    go test -bench="${benchmark}" -memprofile="${output_file}" -count=1 "./${package}" > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        echo "✅ Memory profiling for ${name} completed"
    else
        echo "❌ Memory profiling for ${name} failed"
    fi
}

# Run handler benchmarks
echo ""
echo "🎯 Handler Layer Benchmarks"
echo "============================"
run_benchmark "internal/handlers" "handler"

# Run repository benchmarks
echo ""
echo "🗄️  Repository Layer Benchmarks"
echo "================================"
run_benchmark "internal/storage" "repository"

# Run CPU profiling for key benchmarks
echo ""
echo "🧠 CPU Profiling"
echo "================"
run_cpu_profile "internal/handlers" "BenchmarkWidgetHandler_GetWidgets/NoFilters_1000_widgets" "handler_no_filters_1000"
run_cpu_profile "internal/handlers" "BenchmarkWidgetHandler_GetWidgets/CombinedFilters_1000_widgets" "handler_combined_filters_1000"
run_cpu_profile "internal/storage" "BenchmarkWidgetRepository_GetByUserIDWithFilters/NoFilters_1000_widgets" "repository_no_filters_1000"
run_cpu_profile "internal/storage" "BenchmarkWidgetRepository_GetByUserIDWithFilters/CombinedFilters_1000_widgets" "repository_combined_filters_1000"

# Run memory profiling for key benchmarks
echo ""
echo "💾 Memory Profiling"
echo "=================="
run_mem_profile "internal/handlers" "BenchmarkWidgetHandler_GetWidgets/NoFilters_1000_widgets" "handler_no_filters_1000"
run_mem_profile "internal/handlers" "BenchmarkWidgetHandler_GetWidgets/CombinedFilters_1000_widgets" "handler_combined_filters_1000"
run_mem_profile "internal/storage" "BenchmarkWidgetRepository_GetByUserIDWithFilters/NoFilters_1000_widgets" "repository_no_filters_1000"
run_mem_profile "internal/storage" "BenchmarkWidgetRepository_GetByUserIDWithFilters/CombinedFilters_1000_widgets" "repository_combined_filters_1000"

# Generate summary report
echo ""
echo "📋 Generating Summary Report"
echo "============================"

SUMMARY_FILE="${REPORT_DIR}/benchmark_summary.md"

cat > "${SUMMARY_FILE}" << EOF
# Widget Search Filters Performance Benchmark Report

**Generated:** $(date)
**Go Version:** $(go version)
**System:** $(uname -a)

## Overview

This report contains performance benchmarks for the widget search filters functionality.
The benchmarks test various scenarios with different data sizes (10, 100, 1000 widgets).

## Test Scenarios

### Handler Layer Benchmarks
- **NoFilters**: Baseline performance without any filters
- **TypeFilter**: Performance with widget type filtering
- **VisibilityFilter**: Performance with visibility status filtering  
- **SearchFilter**: Performance with name search filtering
- **CombinedFilters**: Performance with multiple filters applied

### Repository Layer Benchmarks
- **NoFilters**: Baseline repository performance
- **TypeFilter**: Repository type filtering performance
- **VisibilityFilter**: Repository visibility filtering performance
- **SearchFilter**: Repository search filtering performance
- **CombinedFilters**: Repository combined filtering performance
- **MultipleTypes**: Repository multiple type filtering performance

## Files Generated

EOF

# List all generated files
for file in "${REPORT_DIR}"/*; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        echo "- \`${filename}\`" >> "${SUMMARY_FILE}"
    fi
done

cat >> "${SUMMARY_FILE}" << EOF

## How to Analyze Results

### Benchmark Output Format
\`\`\`
BenchmarkName-N    iterations    ns/op    B/op    allocs/op
\`\`\`

- **iterations**: Number of times the benchmark was run
- **ns/op**: Nanoseconds per operation
- **B/op**: Bytes allocated per operation
- **allocs/op**: Number of allocations per operation

### CPU Profiling
Use \`go tool pprof\` to analyze CPU profiles:
\`\`\`bash
go tool pprof handler_no_filters_1000_cpu.prof
\`\`\`

### Memory Profiling
Use \`go tool pprof\` to analyze memory profiles:
\`\`\`bash
go tool pprof handler_no_filters_1000_mem.prof
\`\`\`

## Performance Targets

Based on requirements, the filtering functionality should:
- Maintain baseline performance when no filters are applied
- Use existing Redis indexes for optimal type/visibility filtering
- Minimize memory allocations during filtering operations
- Scale linearly with the number of widgets being filtered

EOF

echo "✅ Summary report generated: ${SUMMARY_FILE}"

# Create a simple performance comparison if previous results exist
if [ -d "${RESULTS_DIR}/previous" ]; then
    echo ""
    echo "📊 Comparing with Previous Results"
    echo "=================================="
    
    COMPARISON_FILE="${REPORT_DIR}/performance_comparison.txt"
    
    echo "Performance Comparison" > "${COMPARISON_FILE}"
    echo "=====================" >> "${COMPARISON_FILE}"
    echo "" >> "${COMPARISON_FILE}"
    
    # Simple comparison logic (this could be enhanced)
    echo "Previous results found in ${RESULTS_DIR}/previous" >> "${COMPARISON_FILE}"
    echo "Manual comparison recommended using benchmark analysis tools" >> "${COMPARISON_FILE}"
    
    echo "✅ Comparison file created: ${COMPARISON_FILE}"
fi

# Create symlink to latest results
ln -sfn "${TIMESTAMP}" "${RESULTS_DIR}/latest"

echo ""
echo "🎉 All benchmarks completed successfully!"
echo "📁 Results directory: ${REPORT_DIR}"
echo "🔗 Latest results: ${RESULTS_DIR}/latest"
echo ""
echo "Next steps:"
echo "1. Review benchmark results in the generated files"
echo "2. Analyze CPU/memory profiles using 'go tool pprof'"
echo "3. Compare results with performance requirements"
echo "4. Identify any performance bottlenecks for optimization"