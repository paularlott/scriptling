package similarity

import (
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	cases := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical", []float64{1, 0, 0}, []float64{1, 0, 0}, 1.0},
		{"orthogonal", []float64{1, 0}, []float64{0, 1}, 0.0},
		{"opposite", []float64{1, 1}, []float64{-1, -1}, -1.0},
		{"45deg", []float64{1, 0}, []float64{1, 1}, 0.7071067811865476},
		{"zero_mag", []float64{0, 0}, []float64{1, 1}, 0.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CosineSimilarity(c.a, c.b)
			if err != nil {
				t.Fatal(err)
			}
			if abs(got-c.want) > 1e-9 {
				t.Errorf("got %.10f, want %.10f", got, c.want)
			}
		})
	}
}

func TestCosineSimilarityErrors(t *testing.T) {
	if _, err := CosineSimilarity([]float64{1}, []float64{1, 2}); err == nil {
		t.Error("expected dimension mismatch error")
	}
	if _, err := CosineSimilarity([]float64{}, []float64{}); err == nil {
		t.Error("expected empty vector error")
	}
}

func TestVectorFromText(t *testing.T) {
	// Same text produces same vector.
	v1 := VectorFromText("hello world", 64)
	v2 := VectorFromText("hello world", 64)
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatal("identical text should produce identical vectors")
		}
	}

	// Different text produces different vector.
	v3 := VectorFromText("goodbye universe", 64)
	diff := false
	for i := range v1 {
		if abs(v1[i]-v3[i]) > 1e-9 {
			diff = true
			break
		}
	}
	if !diff {
		t.Fatal("different text should produce different vectors")
	}

	// Output dimension matches request.
	if len(v1) != 64 {
		t.Errorf("expected 64 dims, got %d", len(v1))
	}

	// L2-normalised (magnitude ≈ 1).
	mag := 0.0
	for _, v := range v1 {
		mag += v * v
	}
	if abs(mag-1.0) > 1e-9 {
		t.Errorf("vector not L2-normalised: magnitude^2 = %.6f", mag)
	}

	// Empty text produces zero vector.
	empty := VectorFromText("", 64)
	for _, v := range empty {
		if v != 0 {
			t.Fatal("empty text should produce zero vector")
		}
	}
}

func TestVectorFromTextSimilarity(t *testing.T) {
	// Texts sharing words should have higher similarity than disjoint texts.
	similar := "the quick brown fox"
	related := "the quick red fox"
	unrelated := "completely different content here"

	vS := VectorFromText(similar, 256)
	vR := VectorFromText(related, 256)
	vU := VectorFromText(unrelated, 256)

	scoreRel, _ := CosineSimilarity(vS, vR)
	scoreUnrel, _ := CosineSimilarity(vS, vU)

	if scoreRel <= scoreUnrel {
		t.Errorf("related texts should score higher: related=%.4f unrelated=%.4f", scoreRel, scoreUnrel)
	}
}

func TestMostSimilar(t *testing.T) {
	query := []float64{1, 0, 0}
	vectors := [][]float64{
		{1, 0, 0},     // identical → score 1.0
		{0, 1, 0},     // orthogonal → score 0.0
		{0.9, 0.1, 0}, // close → high score
		{-1, 0, 0},    // opposite → score -1.0
	}

	results := MostSimilar(query, vectors, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Best match should be index 0 (identical).
	if results[0].Index != 0 {
		t.Errorf("top result should be index 0, got %d", results[0].Index)
	}
	if abs(results[0].Score-1.0) > 1e-9 {
		t.Errorf("top score should be 1.0, got %.6f", results[0].Score)
	}
	// Second should be index 2 (close match).
	if results[1].Index != 2 {
		t.Errorf("second result should be index 2, got %d", results[1].Index)
	}

	// top_k=0 returns all.
	all := MostSimilar(query, vectors, 0)
	if len(all) != 4 {
		t.Errorf("top_k=0 should return all, got %d", len(all))
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
