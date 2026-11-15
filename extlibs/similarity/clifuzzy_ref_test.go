package similarity

import clifuzzy "github.com/paularlott/cli/fuzzy"

func clifuzzyScore(a, b string) float64 { return clifuzzy.Score(a, b) }
