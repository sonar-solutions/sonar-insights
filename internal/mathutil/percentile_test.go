package mathutil

import (
	"math"
	"testing"
)

func TestCalculatePercentile_Empty(t *testing.T) {
	if got := CalculatePercentile([]float64{}, 0.5); got != 0 {
		t.Errorf("empty: got %v, want 0", got)
	}
}

func TestCalculatePercentile_SingleElement(t *testing.T) {
	for _, p := range []float64{0, 0.5, 1.0} {
		if got := CalculatePercentile([]float64{42}, p); got != 42 {
			t.Errorf("single element p=%v: got %v, want 42", p, got)
		}
	}
}

func TestCalculatePercentile_AllEqual(t *testing.T) {
	vals := []int{7, 7, 7, 7, 7}
	for _, p := range []float64{0, 0.25, 0.5, 0.75, 1.0} {
		if got := CalculatePercentile(vals, p); got != 7 {
			t.Errorf("all-equal p=%v: got %v, want 7", p, got)
		}
	}
}

func TestCalculatePercentile_UnsortedInput(t *testing.T) {
	// [5,1,3,2,4] sorted = [1,2,3,4,5]; median (p=0.5) = index 2 = 3
	got := CalculatePercentile([]int{5, 1, 3, 2, 4}, 0.5)
	if got != 3 {
		t.Errorf("unsorted p=0.5: got %v, want 3", got)
	}
}

func TestCalculatePercentile_Levels(t *testing.T) {
	// sorted [1,2,3,4,5,6,7,8,9,10]
	vals := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cases := []struct {
		p    float64
		want float64
	}{
		{0.0, 1.0},
		{1.0, 10.0},
		// p=0.5: position = 0.5*9 = 4.5 → interpolate sorted[4]=5 and sorted[5]=6 → 5.5
		{0.5, 5.5},
		// p=0.8: position = 0.8*9 = 7.2 → sorted[7]=8, sorted[8]=9 → 8+0.2=8.2
		{0.8, 8.2},
		// p=0.9: position = 0.9*9 = 8.1 → sorted[8]=9, sorted[9]=10 → 9+0.1=9.1
		{0.9, 9.1},
	}
	for _, tc := range cases {
		got := CalculatePercentile(vals, tc.p)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("p=%v: got %v, want %v", tc.p, got, tc.want)
		}
	}
}

func TestCalculatePercentile_IntSlice(t *testing.T) {
	// Verify generic constraint works with int
	vals := []int{10, 20, 30, 40, 50}
	// p=0.25: position=1 → sorted[1]=20
	got := CalculatePercentile(vals, 0.25)
	if got != 20 {
		t.Errorf("int p=0.25: got %v, want 20", got)
	}
}

func TestCalculatePercentile_MatchesCSharpReference(t *testing.T) {
	// Replicate C# CalculationExtensionMethods.CalculatePercentile with a simple dataset.
	// sorted = [0,1,2,...,9]; p=0.8 position=7.2 → 7+0.2*(8-7)=7.2
	vals := make([]float64, 10)
	for i := range vals {
		vals[i] = float64(i)
	}
	got := CalculatePercentile(vals, 0.8)
	want := 7.2
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("p=0.8: got %v, want %v", got, want)
	}
}
