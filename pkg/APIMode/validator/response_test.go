package validator

import (
	"slices"
	"testing"
)

func TestSampleSlice_SizeLessThanLimit(t *testing.T) {
	input := []int{1, 2, 3}
	n := 5

	result := SampleSlice(input, n)

	if len(result) != len(input) {
		t.Errorf("Expected %d elements, got %d", len(input), len(result))
	}
}

func TestSampleSlice_SizeEqualToLimit(t *testing.T) {
	input := []int{1, 2, 3}
	n := 3

	result := SampleSlice(input, n)

	if len(result) != n {
		t.Errorf("Expected %d elements, got %d", n, len(result))
	}
}

func TestSampleSlice_SizeGreaterThanLimit(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	n := 3

	result := SampleSlice(input, n)

	if len(result) != n {
		t.Errorf("Expected %d elements, got %d", n, len(result))
	}

	originalSet := make(map[int]bool)
	for _, v := range input {
		originalSet[v] = true
	}

	for _, v := range result {
		if !originalSet[v] {
			t.Errorf("Sample returned element not in original slice: %v", v)
		}
	}
}

func TestSampleSlice_NoDuplicates(t *testing.T) {
	input := []int{10, 20, 30, 40, 50}
	n := 5

	result := SampleSlice(input, n)

	seen := make(map[int]bool)
	for _, v := range result {
		if seen[v] {
			t.Errorf("Duplicate element found in sample: %v", v)
		}
		seen[v] = true
	}
}

func TestSampleSlice_NoLimit(t *testing.T) {
	input := []int{10, 20, 30, 40, 50}
	n := 0

	result := SampleSlice(input, n)

	if len(result) != len(input) {
		t.Errorf("Expected %d elements, got %d", n, len(result))
	}

	if !slices.Equal(input, result) {
		t.Errorf("Expected %v, got %v", input, result)
	}
}
