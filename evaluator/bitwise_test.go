package evaluator

import (
	"testing"
)

func TestBitwiseOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		// Bitwise NOT
		{"~5", -6},
		{"~0", -1},
		{"~-1", 0},
		{"~10", -11},

		// Bitwise AND
		{"12 & 10", 8},
		{"5 & 3", 1},
		{"15 & 7", 7},
		{"0 & 5", 0},

		// Bitwise OR
		{"12 | 10", 14},
		{"5 | 3", 7},
		{"8 | 4", 12},
		{"0 | 5", 5},

		// Bitwise XOR
		{"12 ^ 10", 6},
		{"5 ^ 3", 6},
		{"15 ^ 15", 0},
		{"0 ^ 5", 5},

		// Left shift
		{"1 << 3", 8},
		{"5 << 2", 20},
		{"7 << 1", 14},
		{"10 << 0", 10},

		// Right shift
		{"8 >> 3", 1},
		{"20 >> 2", 5},
		{"14 >> 1", 7},
		{"10 >> 0", 10},
		{"7 >> 1", 3},

		// Operator precedence
		{"5 | 3 & 6", 7},   // & has higher precedence than |
		{"2 + 3 << 1", 10}, // + has higher precedence than <<
		{"8 >> 1 + 1", 2},  // + has higher precedence than >>

		// Combined operations
		{"170 & 15", 10}, // Extract lower 4 bits
		{"170 | 15", 175},
		{"170 ^ 15", 165},

		// Negative numbers
		{"-5 & 3", 3},
		{"-5 | 3", -5},
		{"-5 ^ 3", -8},

		// Chained operations
		{"255 & 15 | 3", 15},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestBitwiseAugmentedAssignment(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"x = 12\nx &= 10\nx", 8},
		{"x = 12\nx |= 10\nx", 14},
		{"x = 12\nx ^= 10\nx", 6},
		{"x = 5\nx <<= 2\nx", 20},
		{"x = 20\nx >>= 2\nx", 5},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}
