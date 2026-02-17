package fuzzy

import (
	"context"
	"fmt"
	"sync"

	clifuzzy "github.com/paularlott/cli/fuzzy"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.fuzzy"
	LibraryDesc = "Fuzzy string matching utilities for searching and matching text"
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

// Register registers the fuzzy library with the given registrar.
// First call builds the library, subsequent calls just register it.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

// buildLibrary builds the fuzzy library
func buildLibrary() *object.Library {
	builder := object.NewLibraryBuilder(LibraryName, LibraryDesc)

	// search(query, items, max_results=5, threshold=0.5, key="name") - Search for multiple matches
	builder.FunctionWithHelp("search", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		// Get query from first positional arg or kwargs
		var query string
		if len(args) > 0 {
			if str, ok := args[0].(*object.String); ok {
				query = str.Value
			} else {
				return nil, fmt.Errorf("query must be a string")
			}
		} else {
			query = kwargs.MustGetString("query", "")
			if query == "" {
				return nil, fmt.Errorf("query parameter is required")
			}
		}

		// Get items (required)
		itemsObj := kwargs.Get("items")
		if itemsObj == nil {
			if len(args) > 1 {
				itemsObj = args[1]
			}
		}
		if itemsObj == nil {
			return nil, fmt.Errorf("items parameter is required")
		}

		// Convert items to list
		itemsList, ok := itemsObj.(*object.List)
		if !ok {
			return nil, fmt.Errorf("items must be a list")
		}

		// Get optional parameters
		maxResults := kwargs.MustGetInt("max_results", 5)
		threshold := kwargs.MustGetFloat("threshold", 0.5)
		keyField := kwargs.MustGetString("key", "name")

		// Validate parameters
		if maxResults < 1 {
			maxResults = 5
		}
		if threshold < 0 || threshold > 1 {
			threshold = 0.5
		}

		// Convert list items to fuzzy.NamedItem
		items := make([]clifuzzy.NamedItem, 0, len(itemsList.Elements))
		for i, elem := range itemsList.Elements {
			item := convertToNamedItem(elem, i, keyField)
			if item != nil {
				items = append(items, item)
			}
		}

		// Perform search
		opts := clifuzzy.Options{
			MaxResults: int(maxResults),
			Threshold:  threshold,
		}
		results := clifuzzy.Search(query, items, opts)

		// Convert results to scriptling format
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
(exact → substring → word boundary → Levenshtein distance).

Parameters:
  query (str): The search query string
  items (list): List of items to search. Each item can be:
    - A string (id will be index)
    - A dict with 'id' and 'name' keys (or keys specified by 'key' param)
  max_results (int, optional): Maximum results to return. Default: 5
  threshold (float, optional): Minimum similarity threshold (0.0-1.0). Default: 0.5
  key (str, optional): Key to use for item name in dicts. Default: "name"

Returns:
  list: List of match dictionaries, each with:
    - id: The matched item's ID
    - name: The matched item's name
    - score: Match score (0.0 to 1.0, higher is better)

Example:
  import scriptling.fuzzy as fuzzy

  # Search list of strings
  results = fuzzy.search("proj", ["Project Alpha", "Task Beta", "Project Gamma"])
  for r in results:
      print(f"{r['name']}: {r['score']}")

  # Search list of dicts
  projects = [
      {"id": 1, "name": "Website Redesign"},
      {"id": 2, "name": "Mobile App Development"},
      {"id": 3, "name": "Server Migration"},
  ]
  results = fuzzy.search("web", projects, max_results=3)

  # Search with custom key field
  items = [{"id": 1, "title": "My Project"}]
  results = fuzzy.search("proj", items, key="title")`)

	// best(query, items, entity_type="item", key="id") - Find single best match
	builder.FunctionWithHelp("best", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		// Get query from first positional arg or kwargs
		var query string
		if len(args) > 0 {
			if str, ok := args[0].(*object.String); ok {
				query = str.Value
			} else {
				return nil, fmt.Errorf("query must be a string")
			}
		} else {
			query = kwargs.MustGetString("query", "")
			if query == "" {
				return nil, fmt.Errorf("query parameter is required")
			}
		}

		// Get items (required)
		itemsObj := kwargs.Get("items")
		if itemsObj == nil {
			if len(args) > 1 {
				itemsObj = args[1]
			}
		}
		if itemsObj == nil {
			return nil, fmt.Errorf("items parameter is required")
		}

		// Convert items to list
		itemsList, ok := itemsObj.(*object.List)
		if !ok {
			return nil, fmt.Errorf("items must be a list")
		}

		// Get optional parameters
		entityType := kwargs.MustGetString("entity_type", "item")
		keyField := kwargs.MustGetString("key", "name")
		threshold := kwargs.MustGetFloat("threshold", 0.5)

		// Convert list items to fuzzy.NamedItem
		items := make([]clifuzzy.NamedItem, 0, len(itemsList.Elements))
		for i, elem := range itemsList.Elements {
			item := convertToNamedItem(elem, i, keyField)
			if item != nil {
				items = append(items, item)
			}
		}

		// Perform best match search
		opts := clifuzzy.Options{
			MaxResults: 5,
			Threshold:  threshold,
		}
		result := clifuzzy.Best(query, items, entityType, opts)

		// Convert result to scriptling format
		var resultMap map[string]any
		if result.Found {
			resultMap = map[string]any{
				"found": true,
				"id":    result.ID,
				"name":  result.Name,
				"score": result.Score,
				"error": nil,
			}
		} else {
			resultMap = map[string]any{
				"found": false,
				"id":    nil,
				"name":  nil,
				"score": 0.0,
				"error": result.Error,
			}
		}

		return conversion.FromGo(resultMap), nil
	}, `best(query, items, entity_type="item", key="name", threshold=0.5) - Find best match with error formatting

Finds the best match for a query. If no match is found, returns an error
message with suggestions. This is ideal for command-line tools where you
want to suggest alternatives when a name is not found.

Parameters:
  query (str): The search query string
  items (list): List of items to search. Each item can be:
    - A string (id will be index)
    - A dict with 'id' and 'name' keys (or keys specified by 'key' param)
  entity_type (str, optional): Type name for error messages. Default: "item"
  key (str, optional): Key to use for item name in dicts. Default: "name"
  threshold (float, optional): Minimum similarity threshold (0.0-1.0). Default: 0.5

Returns:
  dict: Dictionary with:
    - found (bool): True if a match was found
    - id (int or None): The matched item's ID
    - name (str or None): The matched item's name
    - score (float): Match score (0 if not found)
    - error (str or None): Error message with suggestions if not found

Example:
  import scriptling.fuzzy as fuzzy

  projects = [
      {"id": 1, "name": "Website Redesign"},
      {"id": 2, "name": "Mobile App Development"},
      {"id": 3, "name": "Server Migration"},
  ]

  # Exact match (case-insensitive)
  result = fuzzy.best("website redesign", projects, entity_type="project")
  if result['found']:
      print(f"Found project ID: {result['id']}")

  # Fuzzy match with error handling
  result = fuzzy.best("web design", projects, entity_type="project")
  if not result['found']:
      print(result['error'])  # "project 'web design' is unknown..."`)

	// score(s1, s2) - Calculate similarity score between two strings
	builder.FunctionWithHelp("score", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) (object.Object, error) {
		// Get first string
		var s1, s2 string
		if len(args) >= 2 {
			if str, ok := args[0].(*object.String); ok {
				s1 = str.Value
			} else {
				return nil, fmt.Errorf("first argument must be a string")
			}
			if str, ok := args[1].(*object.String); ok {
				s2 = str.Value
			} else {
				return nil, fmt.Errorf("second argument must be a string")
			}
		} else {
			s1 = kwargs.MustGetString("s1", "")
			s2 = kwargs.MustGetString("s2", "")
		}

		if s1 == "" || s2 == "" {
			return &object.Float{Value: 0.0}, nil
		}

		score := clifuzzy.Score(s1, s2)
		return &object.Float{Value: score}, nil
	}, `score(s1, s2) - Calculate similarity score between two strings

Calculates the similarity between two strings using normalized Levenshtein
distance. Returns a value between 0.0 (completely different) and 1.0 (identical).

Parameters:
  s1 (str): First string
  s2 (str): Second string

Returns:
  float: Similarity score (0.0 to 1.0)

Example:
  import scriptling.fuzzy as fuzzy

  score = fuzzy.score("hello", "hallo")  # ~0.8
  score = fuzzy.score("hello", "hello")  # 1.0
  score = fuzzy.score("hello", "xyz")    # ~0.2`)

	return builder.Build()
}

// namedItemWrapper implements fuzzy.NamedItem for scriptling objects
type namedItemWrapper struct {
	id   int
	name string
}

func (n namedItemWrapper) GetID() int    { return n.id }
func (n namedItemWrapper) GetName() string { return n.name }

// convertToNamedItem converts a scriptling object to a fuzzy.NamedItem
func convertToNamedItem(obj object.Object, index int, keyField string) clifuzzy.NamedItem {
	switch v := obj.(type) {
	case *object.String:
		return namedItemWrapper{id: index, name: v.Value}
	case *object.Dict:
		// Get ID
		var id int
		idPair, hasID := v.GetByString("id")
		if hasID {
			idVal, err := idPair.Value.CoerceInt()
			if err == nil {
				id = int(idVal)
			} else {
				id = index
			}
		} else {
			id = index
		}

		// Get name using keyField
		namePair, hasName := v.GetByString(keyField)
		if hasName {
			if nameStr, ok := namePair.Value.(*object.String); ok {
				return namedItemWrapper{id: id, name: nameStr.Value}
			}
		}
	}
	return nil
}
