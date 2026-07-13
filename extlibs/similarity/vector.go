package similarity

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/paularlott/scriptling/object"
)

var (
	ErrDimensionMismatch = errors.New("vectors must have the same length")
	ErrEmptyVector       = errors.New("vectors must not be empty")
	errNotAVector        = errors.New("expected a list of numbers")
)

func errVectorElement(i int) error {
	return fmt.Errorf("vector element %d is not a number", i)
}

// CosineSimilarity computes the cosine of the angle between two vectors.
// Returns 0.0 if either vector has zero magnitude.
func CosineSimilarity(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, ErrDimensionMismatch
	}
	if len(a) == 0 {
		return 0, ErrEmptyVector
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0, nil
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB)), nil
}

// VectorFromText produces a fixed-dimensional vector from text using the
// feature-hashing trick (the "hashing trick"). Each token is mapped to a
// dimension via FNV-1a and contributes +1 or −1 based on a sign hash. The
// resulting vector is L2-normalised so it can be compared directly with
// CosineSimilarity. This is a fast, deterministic, CPU-only method that
// captures lexical overlap — similar texts produce similar vectors.
func VectorFromText(text string, dims int) []float64 {
	if dims < 1 {
		dims = 256
	}
	vec := make([]float64, dims)
	for _, tok := range vectorTokens(text) {
		h := fnv.New64a()
		h.Write([]byte(tok))
		hash := h.Sum64()
		idx := int(hash % uint64(dims))
		sign := 1.0
		if (hash>>32)%2 == 1 {
			sign = -1.0
		}
		vec[idx] += sign
	}
	// L2-normalise.
	var mag float64
	for _, v := range vec {
		mag += v * v
	}
	if mag > 0 {
		inv := 1.0 / math.Sqrt(mag)
		for i := range vec {
			vec[i] *= inv
		}
	}
	return vec
}

// ScoredVector is a ranked result from MostSimilar.
type ScoredVector struct {
	Index int
	Score float64
}

// MostSimilar ranks vectors by cosine similarity to query, returning the
// top-k results sorted by descending score. If topK <= 0 all results are
// returned.
func MostSimilar(query []float64, vectors [][]float64, topK int) []ScoredVector {
	results := make([]ScoredVector, len(vectors))
	for i, v := range vectors {
		score, _ := CosineSimilarity(query, v)
		results[i] = ScoredVector{Index: i, Score: score}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if topK > 0 && topK < len(results) {
		results = results[:topK]
	}
	return results
}

// ToFloat64Slice converts a scriptling Object (FloatArray or List of numbers)
// to a []float64. Shared by ai and similarity libraries.
func ToFloat64Slice(obj object.Object) ([]float64, error) {
	switch v := obj.(type) {
	case *object.FloatArray:
		return v.Data, nil
	case *object.List:
		result := make([]float64, len(v.Elements))
		for i, item := range v.Elements {
			f, err := item.AsFloat()
			if err != nil {
				return nil, errVectorElement(i)
			}
			result[i] = f
		}
		return result, nil
	default:
		return nil, errNotAVector
	}
}

// vectorTokens lowercases text and splits on non-alphanumeric boundaries,
// returning tokens of length >= 2.
func vectorTokens(text string) []string {
	text = strings.ToLower(text)
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
