package similarity

import (
	"fmt"
	"strings"

	clifuzzy "github.com/paularlott/cli/fuzzy"
	"github.com/paularlott/scriptling/libutil"
	"github.com/paularlott/scriptling/object"
)

// The fuzzy matching here is github.com/paularlott/cli/fuzzy (Search, Best
// and Score give the same results) with the edit distances charged to a
// WorkMeter: Levenshtein distance is quadratic, so a script comparing two
// long strings could otherwise run for as long as it takes whatever the
// call's deadline.

// fuzzyScore is clifuzzy.Score honouring the call's cancellation.
func fuzzyScore(s1, s2 string, meter *libutil.WorkMeter) (float64, object.Object) {
	s1 = strings.ToLower(s1)
	s2 = strings.ToLower(s2)
	if s1 == s2 {
		return 1.0, nil
	}
	len1, len2 := len(s1), len(s2)
	if len1 == 0 || len2 == 0 {
		return 0.0, nil
	}
	dist, errObj := levenshtein(s1, s2, meter)
	if errObj != nil {
		return 0, errObj
	}
	return 1.0 - float64(dist)/float64(max(len1, len2)), nil
}

// fuzzySearch is clifuzzy.Search honouring the call's cancellation.
func fuzzySearch(query string, items []clifuzzy.NamedItem, opts clifuzzy.Options, meter *libutil.WorkMeter) ([]clifuzzy.Result, object.Object) {
	if len(items) == 0 {
		return nil, nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, nil
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 5
	}

	results := make([]clifuzzy.Result, 0, opts.MaxResults)
	seen := make(map[int]bool, len(items))
	queryWords := strings.Fields(query)

	for _, item := range items {
		if seen[item.GetID()] {
			continue
		}
		name := item.GetName()
		if errObj := meter.Add(len(name) + len(query)); errObj != nil {
			return nil, errObj
		}
		nameLower := strings.ToLower(name)
		var score float64
		var matched bool

		if nameLower == query {
			score = 1.0
			matched = true
		} else if strings.Contains(nameLower, query) {
			score = 0.9 * (float64(len(query)) / float64(len(nameLower)))
			matched = true
		} else {
			nameWords := strings.Fields(nameLower)
			for _, word := range nameWords {
				if strings.HasPrefix(word, query) {
					score = 0.85
					matched = true
					break
				}
			}

			if !matched && len(results) < opts.MaxResults {
				minDistance := 999999.0
				for _, searchWord := range queryWords {
					for _, nameWord := range nameWords {
						maxLen := max(len(searchWord), len(nameWord))
						if maxLen == 0 {
							continue
						}
						dist, errObj := levenshtein(searchWord, nameWord, meter)
						if errObj != nil {
							return nil, errObj
						}
						if d := float64(dist) / float64(maxLen); d < minDistance {
							minDistance = d
						}
					}
				}
				if 1.0-minDistance >= opts.Threshold {
					score = 1.0 - minDistance
					matched = true
				}
			}
		}

		if matched {
			results = append(results, clifuzzy.Result{ID: item.GetID(), Name: name, Score: score})
			seen[item.GetID()] = true
		}
	}

	// Insertion sort, best first, ties in input order (as clifuzzy).
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && results[j].Score < key.Score {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}
	if len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}
	return results, nil
}

// fuzzyBest is clifuzzy.Best honouring the call's cancellation.
func fuzzyBest(query string, items []clifuzzy.NamedItem, entityType string, meter *libutil.WorkMeter) (clifuzzy.BestResult, object.Object) {
	if len(items) == 0 {
		return clifuzzy.BestResult{Error: fmt.Sprintf("no %s available", entityType)}, nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return clifuzzy.BestResult{Error: fmt.Sprintf("%s name is required", entityType)}, nil
	}
	for _, item := range items {
		if errObj := meter.Add(len(item.GetName())); errObj != nil {
			return clifuzzy.BestResult{}, errObj
		}
		if strings.ToLower(item.GetName()) == query {
			return clifuzzy.BestResult{Found: true, ID: item.GetID(), Name: item.GetName(), Score: 1.0}, nil
		}
	}
	suggestions, errObj := fuzzySearch(query, items, clifuzzy.Options{MaxResults: 3, Threshold: 0.5}, meter)
	if errObj != nil {
		return clifuzzy.BestResult{}, errObj
	}
	if len(suggestions) == 0 {
		return clifuzzy.BestResult{Error: fmt.Sprintf("%s '%s' is unknown. No similar matches found", entityType, query)}, nil
	}
	best := suggestions[0]
	return clifuzzy.BestResult{Found: true, ID: best.ID, Name: best.Name, Score: best.Score}, nil
}

// levenshtein is the byte-wise edit distance between s1 and s2, charging
// meter one unit per cell of the distance table.
// levStackRow is the longest distance-table row kept on the stack.
const levStackRow = 128

func levenshtein(s1, s2 string, meter *libutil.WorkMeter) (int, object.Object) {
	len1, len2 := len(s1), len(s2)
	if len1 == 0 {
		return len2, nil
	}
	if len2 == 0 {
		return len1, nil
	}
	if len1 < len2 {
		s1, s2 = s2, s1
		len1, len2 = len2, len1
	}
	// Rows for short strings live on the stack, as the pooled rows of
	// cli/fuzzy avoid allocating; longer ones are allocated.
	var small [2 * levStackRow]int
	var prev, curr []int
	if len2+1 <= levStackRow {
		prev, curr = small[:len2+1], small[levStackRow:levStackRow+len2+1]
	} else {
		prev, curr = make([]int, len2+1), make([]int, len2+1)
	}
	for j := 0; j <= len2; j++ {
		prev[j] = j
	}
	for i := 1; i <= len1; i++ {
		if errObj := meter.Add(len2); errObj != nil {
			return 0, errObj
		}
		curr[0] = i
		c := s1[i-1]
		for j := 1; j <= len2; j++ {
			cost := 1
			if c == s2[j-1] {
				cost = 0
			}
			del, ins, sub := prev[j]+1, curr[j-1]+1, prev[j-1]+cost
			if del < ins {
				if del < sub {
					curr[j] = del
				} else {
					curr[j] = sub
				}
			} else if ins < sub {
				curr[j] = ins
			} else {
				curr[j] = sub
			}
		}
		prev, curr = curr, prev
	}
	return prev[len2], nil
}
