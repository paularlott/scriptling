package similarity

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	clifuzzy "github.com/paularlott/cli/fuzzy"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName    = "scriptling.similarity"
	LibraryDesc    = "String matching and similarity utilities including fuzzy search and MinHash"
	defaultKey     = "name"
	defaultHashes  = 64
	minTokenLength = 2
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

// Register registers the similarity library with the given registrar.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

func buildLibrary() *object.Library {
	builder := object.NewLibraryBuilder(LibraryName, LibraryDesc)

	builder.FunctionWithHelp("search", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		query, itemsList, err := parseSearchArgs(kwargs, args)
		if err != nil {
			return nil, err
		}

		maxResults := kwargs.MustGetInt("max_results", 5)
		threshold := kwargs.MustGetFloat("threshold", 0.5)
		keyField := kwargs.MustGetString("key", defaultKey)

		if maxResults < 1 {
			maxResults = 5
		}
		if threshold < 0 || threshold > 1 {
			threshold = 0.5
		}

		items := toNamedItems(itemsList, keyField)
		opts := clifuzzy.Options{MaxResults: int(maxResults), Threshold: threshold}
		results := clifuzzy.Search(query, items, opts)

		resultList := make([]map[string]any, len(results))
		for i, r := range results {
			resultList[i] = map[string]any{
				"id":    r.ID,
				"name":  r.Name,
				"score": r.Score,
			}
		}

		return conversion.FromGo(resultList), nil
	}, `search(query, items, max_results=5, threshold=0.5, key="name") - Search for fuzzy matches

Searches for fuzzy matches in a list of items using a multi-tier algorithm
(exact -> substring -> word boundary -> Levenshtein distance).

Parameters:
  query (str): The search query string
  items (list): List of items to search
  max_results (int, optional): Maximum results to return. Default: 5
  threshold (float, optional): Minimum similarity threshold (0.0-1.0). Default: 0.5
  key (str, optional): Key to use for item names in dicts. Default: "name"

Returns:
  list: List of match dictionaries with id, name, and score`)

	builder.FunctionWithHelp("best", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		query, itemsList, err := parseSearchArgs(kwargs, args)
		if err != nil {
			return nil, err
		}

		entityType := kwargs.MustGetString("entity_type", "item")
		keyField := kwargs.MustGetString("key", defaultKey)
		threshold := kwargs.MustGetFloat("threshold", 0.5)

		items := toNamedItems(itemsList, keyField)
		opts := clifuzzy.Options{MaxResults: 5, Threshold: threshold}
		result := clifuzzy.Best(query, items, entityType, opts)

		resultMap := map[string]any{
			"found": result.Found,
			"id":    nil,
			"name":  nil,
			"score": 0.0,
			"error": result.Error,
		}
		if result.Found {
			resultMap["id"] = result.ID
			resultMap["name"] = result.Name
			resultMap["score"] = result.Score
			resultMap["error"] = nil
		}

		return conversion.FromGo(resultMap), nil
	}, `best(query, items, entity_type="item", key="name", threshold=0.5) - Find best match with error formatting

Finds the best fuzzy match for a query and returns either the match or a
helpful error message with suggestions.

Parameters:
  query (str): The search query string
  items (list): List of items to search
  entity_type (str, optional): Type name for error messages. Default: "item"
  key (str, optional): Key to use for item names in dicts. Default: "name"
  threshold (float, optional): Minimum similarity threshold (0.0-1.0). Default: 0.5

Returns:
  dict: {found, id, name, score, error}`)

	builder.FunctionWithHelp("score", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		var s1, s2 string
		if len(args) >= 2 {
			var objErr object.Object
			s1, objErr = args[0].AsString()
			if objErr != nil {
				return nil, fmt.Errorf("first argument must be a string")
			}
			s2, objErr = args[1].AsString()
			if objErr != nil {
				return nil, fmt.Errorf("second argument must be a string")
			}
		} else {
			s1 = kwargs.MustGetString("s1", "")
			s2 = kwargs.MustGetString("s2", "")
		}

		if s1 == "" || s2 == "" {
			return &object.Float{Value: 0.0}, nil
		}

		return &object.Float{Value: clifuzzy.Score(s1, s2)}, nil
	}, `score(s1, s2) - Calculate fuzzy similarity between two strings

Returns a normalized score between 0.0 and 1.0 using edit-distance based
matching.`)

	builder.FunctionWithHelp("tokenize", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("text parameter is required")
		}
		text, err := args[0].AsString()
		if err != nil {
			return nil, fmt.Errorf("text must be a string")
		}
		return conversion.FromGo(tokenize(text)), nil
	}, `tokenize(text) - Split text into lowercase alphanumeric tokens

Only letters a-z and digits 0-9 are retained; everything else becomes a word
boundary.

Returns:
  list[str]: Lowercase tokens`)

	builder.FunctionWithHelp("minhash", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("text parameter is required")
		}
		text, err := args[0].AsString()
		if err != nil {
			return nil, fmt.Errorf("text must be a string")
		}
		numHashes := int(kwargs.MustGetInt("num_hashes", defaultHashes))
		if numHashes <= 0 {
			numHashes = defaultHashes
		}
		return conversion.FromGo(computeMinHash(text, numHashes)), nil
	}, `minhash(text, num_hashes=64) - Compute a MinHash signature for text

Useful for approximate set similarity and semantic-ish recall over tokenized
text. The default output contains 64 32-bit hash values.

Returns:
  list[int]: MinHash signature`)

	builder.FunctionWithHelp("minhash_similarity", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		var leftObj, rightObj object.Object
		if len(args) >= 2 {
			leftObj = args[0]
			rightObj = args[1]
		} else {
			leftObj = kwargs.Get("a")
			rightObj = kwargs.Get("b")
		}
		if leftObj == nil || rightObj == nil {
			return nil, fmt.Errorf("a and b parameters are required")
		}

		left, err := objectListToUint32s(leftObj)
		if err != nil {
			return nil, fmt.Errorf("a must be a list of integers")
		}
		right, err := objectListToUint32s(rightObj)
		if err != nil {
			return nil, fmt.Errorf("b must be a list of integers")
		}

		return &object.Float{Value: minHashSimilarity(left, right)}, nil
	}, `minhash_similarity(a, b) - Compare two MinHash signatures

Returns the fraction of matching positions between two signatures. This is an
estimate of Jaccard similarity.

Returns:
  float: Similarity score between 0.0 and 1.0`)

	return builder.Build()
}

type namedItemWrapper struct {
	id   int
	name string
}

func (n namedItemWrapper) GetID() int      { return n.id }
func (n namedItemWrapper) GetName() string { return n.name }

func parseSearchArgs(kwargs object.Kwargs, args []object.Object) (string, *object.List, error) {
	var query string
	if len(args) > 0 {
		str, ok := args[0].(*object.String)
		if !ok {
			return "", nil, fmt.Errorf("query must be a string")
		}
		query = str.Value
	} else {
		query = kwargs.MustGetString("query", "")
		if query == "" {
			return "", nil, fmt.Errorf("query parameter is required")
		}
	}

	itemsObj := kwargs.Get("items")
	if itemsObj == nil && len(args) > 1 {
		itemsObj = args[1]
	}
	if itemsObj == nil {
		return "", nil, fmt.Errorf("items parameter is required")
	}

	itemsList, ok := itemsObj.(*object.List)
	if !ok {
		return "", nil, fmt.Errorf("items must be a list")
	}

	return query, itemsList, nil
}

func toNamedItems(itemsList *object.List, keyField string) []clifuzzy.NamedItem {
	items := make([]clifuzzy.NamedItem, 0, len(itemsList.Elements))
	for i, elem := range itemsList.Elements {
		item := convertToNamedItem(elem, i, keyField)
		if item != nil {
			items = append(items, item)
		}
	}
	return items
}

func convertToNamedItem(obj object.Object, index int, keyField string) clifuzzy.NamedItem {
	switch v := obj.(type) {
	case *object.String:
		return namedItemWrapper{id: index, name: v.Value}
	case *object.Dict:
		id := index
		if idPair, hasID := v.GetByString("id"); hasID {
			if idVal, err := idPair.Value.CoerceInt(); err == nil {
				id = int(idVal)
			}
		}
		if namePair, hasName := v.GetByString(keyField); hasName {
			if nameStr, ok := namePair.Value.(*object.String); ok {
				return namedItemWrapper{id: id, name: nameStr.Value}
			}
		}
	}
	return nil
}

func objectListToUint32s(obj object.Object) ([]uint32, error) {
	listObj, ok := obj.(*object.List)
	if !ok {
		return nil, fmt.Errorf("not a list")
	}
	out := make([]uint32, 0, len(listObj.Elements))
	for _, elem := range listObj.Elements {
		v, objErr := elem.CoerceInt()
		if objErr != nil {
			return nil, fmt.Errorf("element is not an integer")
		}
		out = append(out, uint32(v))
	}
	return out, nil
}

func tokenize(text string) []string {
	var tokens []string
	var buf strings.Builder
	for _, r := range strings.ToLower(text) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			buf.WriteRune(r)
		} else if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}
	return tokens
}

func computeMinHash(text string, numHashes int) []uint32 {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return make([]uint32, numHashes)
	}

	signature := make([]uint32, numHashes)
	for i := range signature {
		signature[i] = ^uint32(0)
	}

	for _, token := range tokens {
		if len(token) <= minTokenLength {
			continue
		}

		h := fnv.New128a()
		_, _ = h.Write([]byte(token))
		tokenHash := h.Sum(nil)

		for i := 0; i < numHashes; i++ {
			seed := uint32(i)
			hashVal := binary.BigEndian.Uint32(tokenHash[:4]) ^ seed
			hashVal ^= hashVal >> 13
			hashVal *= 0x5bd1e995
			hashVal ^= hashVal >> 15

			if hashVal < signature[i] {
				signature[i] = hashVal
			}
		}
	}

	return signature
}

func minHashSimilarity(a, b []uint32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	matches := 0
	for i := range a {
		if a[i] == b[i] {
			matches++
		}
	}
	return float64(matches) / float64(len(a))
}
