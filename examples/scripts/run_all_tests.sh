#!/bin/bash
# Run all Scriptling test examples

echo "=== Running All Scriptling Test Examples ==="
echo ""

# Array of test files
tests=(
    # Core language
    "test_basics.py"
    "test_functions.py"
    "test_collections.py"
    "test_loops.py"
    "test_control_flow.py"
		"test_raw_triple.py"

    # Operators
    "test_operators_membership.py"
    "test_operators_augmented.py"
    "test_bitwise_operators.py"

    # Scope
    "test_scope_global.py"
    "test_scope_nonlocal.py"
    "test_scope_combined.py"

    # Error handling
    "test_error_handling.py"
    "test_error_comprehensive.py"
    "test_error_http.py"

    # Assignment
    "test_multiple_assignment.py"

    # Libraries
    "test_lib_json.py"
    "test_lib_http.py"
    "test_lib_math.py"
    "test_lib_base64.py"
    "test_lib_hashlib.py"
    "test_lib_random.py"
    "test_lib_url.py"
    "test_lib_regex.py"
    "test_lib_import.py"
    "test_requests_api.py"
    "test_requests_methods.py"
		"test_datetime.py"
		"test_time.py"

    # Small features
    "test_booleans.py"
    "test_elif.py"
    "test_break_continue.py"
    "test_append.py"
    "test_range_slice.py"

    # Comprehensive
    "test_all_features.py"

    # Advanced features
    "test_string_methods.py"
    "test_list_comprehensions.py"
    "test_default_params.py"
    "test_lambda.py"
    "test_tuples.py"
		"test_kwargs.py"
    "test_variadic_args.py"

    # Examples (keep as-is)
    "variables.py"
    "fibonacci.py"

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
