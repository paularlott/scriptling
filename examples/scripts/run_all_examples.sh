#!/bin/bash
# Run all Scriptling test examples

echo "=== Running All Scriptling Test Examples ==="
echo ""

# Array of test files
tests=(
    # Core language
    "example_basics.py"
    "example_functions.py"
    "example_collections.py"
    "example_loops.py"
    "example_control_flow.py"
		"example_raw_triple.py"

    # Operators
    "example_operators_membership.py"
    "example_operators_augmented.py"
    "example_bitwise_operators.py"

    # Scope
    "example_scope_global.py"
    "example_scope_nonlocal.py"
    "example_scope_combined.py"

    # Error handling
    "example_error_handling.py"
    "example_error_comprehensive.py"
    "example_error_http.py"

    # Assignment
    "example_multiple_assignment.py"

    # Libraries
    "example_lib_json.py"
    "example_lib_http.py"
    "example_lib_math.py"
    "example_lib_base64.py"
    "example_lib_hashlib.py"
    "example_lib_random.py"
    "example_lib_url.py"
    "example_lib_regex.py"
    "example_lib_import.py"
    "example_requests_api.py"
    "example_requests_methods.py"
		"example_datetime.py"
		"example_time.py"

    # Small features
    "example_booleans.py"
    "example_elif.py"
    "example_break_continue.py"
    "example_append.py"
    "example_range_slice.py"

    # Comprehensive
    "example_all_features.py"

    # Advanced features
    "example_string_methods.py"
    "example_list_comprehensions.py"
    "example_default_params.py"
    "example_lambda.py"
    "example_tuples.py"
		"example_kwargs.py"
    "example_variadic_args.py"

    # Examples (keep as-is)
    "variables.py"
    "fibonacci.py"
		"changelog_scraper.py"

		# Help
		"demo_help.py"
)

passed=0
failed=0

for test in "${tests[@]}"; do
    if [ -f "$test" ]; then
        echo "Running: $test"
        if go run main.go "$test" > /dev/null 2>&1; then
            echo "  ✓ PASSED"
            ((passed++))
        else
            echo "  ✗ FAILED"
            ((failed++))
        fi
    else
        echo "Skipping: $test (not found)"
    fi
done

echo ""
echo "=== Test Summary ==="
echo "Passed: $passed"
echo "Failed: $failed"
echo "Total:  $((passed + failed))"

if [ $failed -eq 0 ]; then
    echo ""
    echo "✓ All tests passed!"
    exit 0
else
    echo ""
    echo "✗ Some tests failed"
    exit 1
fi
