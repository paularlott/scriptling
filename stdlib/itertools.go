package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// ItertoolsLibrary provides Python-like itertools functions
var ItertoolsLibrary = object.NewLibrary(map[string]*object.Builtin{
	"chain": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// chain(*iterables) - Chain multiple iterables together
			result := []object.Object{}
			for _, arg := range args {
				switch a := arg.(type) {
				case *object.List:
					result = append(result, a.Elements...)
				case *object.Tuple:
					result = append(result, a.Elements...)
				case *object.String:
					for _, ch := range a.Value {
						result = append(result, &object.String{Value: string(ch)})
					}
				default:
					return errors.NewTypeError("iterable", arg.Type().String())
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `chain(*iterables) - Chain multiple iterables together

Returns a list with elements from all iterables concatenated.

Example:
  itertools.chain([1, 2], [3, 4]) -> [1, 2, 3, 4]
  itertools.chain("ab", "cd") -> ["a", "b", "c", "d"]`,
	},
	"repeat": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// repeat(elem, n) - Repeat element n times
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			elem := args[0]
			times := int64(1)
			if len(args) == 2 {
				if n, ok := args[1].(*object.Integer); ok {
					times = n.Value
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if times < 0 {
				times = 0
			}
			result := make([]object.Object, times)
			for i := int64(0); i < times; i++ {
				result[i] = elem
			}
			return &object.List{Elements: result}
		},
		HelpText: `repeat(elem, n) - Repeat element n times

Returns a list with the element repeated n times.

Example:
  itertools.repeat("x", 3) -> ["x", "x", "x"]
  itertools.repeat(0, 5) -> [0, 0, 0, 0, 0]`,
	},
	"cycle": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// cycle(iterable, n) - Cycle through iterable n times
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}
			n, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			if len(elements) == 0 || n.Value <= 0 {
				return &object.List{Elements: []object.Object{}}
			}
			result := make([]object.Object, 0, len(elements)*int(n.Value))
			for i := int64(0); i < n.Value; i++ {
				result = append(result, elements...)
			}
			return &object.List{Elements: result}
		},
		HelpText: `cycle(iterable, n) - Cycle through iterable n times

Returns a list with elements of iterable repeated n times.
Note: Unlike Python's infinite cycle, this requires specifying the count.

Example:
  itertools.cycle([1, 2, 3], 2) -> [1, 2, 3, 1, 2, 3]`,
	},
	"count": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// count(start, stop[, step]) - Generate a sequence of numbers
			if err := errors.RangeArgs(args, 2, 3); err != nil {
				return err
			}
			start, ok := args[0].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}
			stop, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			step := int64(1)
			if len(args) == 3 {
				if s, ok := args[2].(*object.Integer); ok {
					step = s.Value
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
			}
			if step == 0 {
				return errors.NewError("step cannot be zero")
			}
			result := []object.Object{}
			if step > 0 {
				for i := start.Value; i < stop.Value; i += step {
					result = append(result, object.NewInteger(i))
				}
			} else {
				for i := start.Value; i > stop.Value; i += step {
					result = append(result, object.NewInteger(i))
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `count(start, stop[, step]) - Generate a sequence of numbers

Returns a list of numbers from start to stop (exclusive) with optional step.
Similar to range() but as an itertools function.

Example:
  itertools.count(0, 5) -> [0, 1, 2, 3, 4]
  itertools.count(0, 10, 2) -> [0, 2, 4, 6, 8]`,
	},
	"islice": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// islice(iterable, stop) or islice(iterable, start, stop[, step])
			if err := errors.RangeArgs(args, 2, 4); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			var start, stop, step int64 = 0, 0, 1
			if len(args) == 2 {
				// islice(iterable, stop)
				if s, ok := args[1].(*object.Integer); ok {
					stop = s.Value
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			} else {
				// islice(iterable, start, stop[, step])
				if s, ok := args[1].(*object.Integer); ok {
					start = s.Value
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
				if s, ok := args[2].(*object.Integer); ok {
					stop = s.Value
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
				if len(args) == 4 {
					if s, ok := args[3].(*object.Integer); ok {
						step = s.Value
					} else {
						return errors.NewTypeError("INTEGER", args[3].Type().String())
					}
				}
			}

			if step <= 0 {
				return errors.NewError("step must be positive")
			}
			if start < 0 {
				start = 0
			}
			if stop > int64(len(elements)) {
				stop = int64(len(elements))
			}

			result := []object.Object{}
			for i := start; i < stop; i += step {
				result = append(result, elements[i])
			}
			return &object.List{Elements: result}
		},
		HelpText: `islice(iterable, stop) or islice(iterable, start, stop[, step]) - Slice an iterable

Returns a list with elements from the iterable sliced by indices.

Example:
  itertools.islice([0, 1, 2, 3, 4], 3) -> [0, 1, 2]
  itertools.islice([0, 1, 2, 3, 4], 1, 4) -> [1, 2, 3]
  itertools.islice([0, 1, 2, 3, 4], 0, 5, 2) -> [0, 2, 4]`,
	},
	"takewhile": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// takewhile(predicate, iterable) - Take elements while predicate is true
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			pred, ok := args[0].(*object.Builtin)
			if !ok {
				return errors.NewError("takewhile() predicate must be a builtin function")
			}
			var elements []object.Object
			switch a := args[1].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				return errors.NewTypeError("iterable", args[1].Type().String())
			}
			result := []object.Object{}
			for _, elem := range elements {
				res := pred.Fn(ctx, object.NewKwargs(nil), elem)
				if isError(res) {
					return res
				}
				if !isTruthy(res) {
					break
				}
				result = append(result, elem)
			}
			return &object.List{Elements: result}
		},
		HelpText: `takewhile(predicate, iterable) - Take elements while predicate is true

Returns a list with elements from the start of iterable as long as predicate is true.

Example:
  itertools.takewhile(lambda x: x < 5, [1, 3, 5, 2, 4]) -> [1, 3]`,
	},
	"dropwhile": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// dropwhile(predicate, iterable) - Drop elements while predicate is true
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			pred, ok := args[0].(*object.Builtin)
			if !ok {
				return errors.NewError("dropwhile() predicate must be a builtin function")
			}
			var elements []object.Object
			switch a := args[1].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				return errors.NewTypeError("iterable", args[1].Type().String())
			}
			result := []object.Object{}
			dropping := true
			for _, elem := range elements {
				if dropping {
					res := pred.Fn(ctx, object.NewKwargs(nil), elem)
					if isError(res) {
						return res
					}
					if isTruthy(res) {
						continue
					}
					dropping = false
				}
				result = append(result, elem)
			}
			return &object.List{Elements: result}
		},
		HelpText: `dropwhile(predicate, iterable) - Drop elements while predicate is true

Returns a list with elements after the predicate becomes false.

Example:
  itertools.dropwhile(lambda x: x < 5, [1, 3, 5, 2, 4]) -> [5, 2, 4]`,
	},
	"zip_longest": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// zip_longest(*iterables, fillvalue=None)
			if len(args) < 1 {
				return &object.List{Elements: []object.Object{}}
			}

			// Get fillvalue from kwargs
			fillvalue := object.Object(&object.Null{})
			if kwargs.Len() > 0 {
				if kwargs.Has("fillvalue") {
					fillvalue = kwargs.Get("fillvalue")
				}
			}

			// Convert all arguments to slices
			iterables := make([][]object.Object, len(args))
			maxLen := 0
			for i, arg := range args {
				switch a := arg.(type) {
				case *object.List:
					iterables[i] = a.Elements
				case *object.Tuple:
					iterables[i] = a.Elements
				case *object.String:
					chars := []object.Object{}
					for _, ch := range a.Value {
						chars = append(chars, &object.String{Value: string(ch)})
					}
					iterables[i] = chars
				default:
					return errors.NewTypeError("iterable", arg.Type().String())
				}
				if len(iterables[i]) > maxLen {
					maxLen = len(iterables[i])
				}
			}

			result := []object.Object{}
			for j := 0; j < maxLen; j++ {
				tuple := make([]object.Object, len(iterables))
				for i, iter := range iterables {
					if j < len(iter) {
						tuple[i] = iter[j]
					} else {
						tuple[i] = fillvalue
					}
				}
				result = append(result, &object.Tuple{Elements: tuple})
			}
			return &object.List{Elements: result}
		},
		HelpText: `zip_longest(*iterables, fillvalue=None) - Zip iterables, filling shorter ones

Zips iterables together, using fillvalue for missing values in shorter iterables.

Example:
  itertools.zip_longest([1, 2, 3], ["a", "b"]) -> [(1, "a"), (2, "b"), (3, None)]
  itertools.zip_longest([1, 2], ["a"], fillvalue="-") -> [(1, "a"), (2, "-")]`,
	},
	"product": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// product(*iterables) - Cartesian product
			if len(args) < 1 {
				return &object.List{Elements: []object.Object{&object.Tuple{Elements: []object.Object{}}}}
			}

			// Convert all arguments to slices
			iterables := make([][]object.Object, len(args))
			for i, arg := range args {
				switch a := arg.(type) {
				case *object.List:
					iterables[i] = a.Elements
				case *object.Tuple:
					iterables[i] = a.Elements
				case *object.String:
					chars := []object.Object{}
					for _, ch := range a.Value {
						chars = append(chars, &object.String{Value: string(ch)})
					}
					iterables[i] = chars
				default:
					return errors.NewTypeError("iterable", arg.Type().String())
				}
			}

			// Check for empty iterables
			for _, iter := range iterables {
				if len(iter) == 0 {
					return &object.List{Elements: []object.Object{}}
				}
			}

			// Calculate cartesian product
			result := []object.Object{}
			indices := make([]int, len(iterables))

			for {
				// Create current tuple
				tuple := make([]object.Object, len(iterables))
				for i, idx := range indices {
					tuple[i] = iterables[i][idx]
				}
				result = append(result, &object.Tuple{Elements: tuple})

				// Increment indices
				carry := true
				for i := len(indices) - 1; i >= 0 && carry; i-- {
					indices[i]++
					if indices[i] >= len(iterables[i]) {
						indices[i] = 0
					} else {
						carry = false
					}
				}
				if carry {
					break
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `product(*iterables) - Cartesian product of iterables

Returns all possible combinations (tuples) of elements from input iterables.

Example:
  itertools.product([1, 2], ["a", "b"]) -> [(1, "a"), (1, "b"), (2, "a"), (2, "b")]
  itertools.product([1, 2], [3, 4]) -> [(1, 3), (1, 4), (2, 3), (2, 4)]`,
	},
	"permutations": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// permutations(iterable[, r])
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			r := len(elements)
			if len(args) == 2 {
				if rArg, ok := args[1].(*object.Integer); ok {
					r = int(rArg.Value)
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}

			if r < 0 || r > len(elements) {
				return &object.List{Elements: []object.Object{}}
			}

			// Generate permutations
			result := []object.Object{}
			generatePermutations(elements, r, []object.Object{}, make([]bool, len(elements)), &result)
			return &object.List{Elements: result}
		},
		HelpText: `permutations(iterable[, r]) - Generate permutations

Returns all r-length permutations of elements from iterable.
If r is not specified, defaults to length of iterable (full permutations).

Example:
  itertools.permutations([1, 2, 3], 2) -> [(1, 2), (1, 3), (2, 1), (2, 3), (3, 1), (3, 2)]
  itertools.permutations("ab") -> [("a", "b"), ("b", "a")]`,
	},
	"combinations": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// combinations(iterable, r)
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			rArg, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			r := int(rArg.Value)

			if r < 0 || r > len(elements) {
				return &object.List{Elements: []object.Object{}}
			}

			// Generate combinations
			result := []object.Object{}
			generateCombinations(elements, r, 0, []object.Object{}, &result)
			return &object.List{Elements: result}
		},
		HelpText: `combinations(iterable, r) - Generate combinations

Returns all r-length combinations of elements from iterable (without repetition).

Example:
  itertools.combinations([1, 2, 3], 2) -> [(1, 2), (1, 3), (2, 3)]
  itertools.combinations("abc", 2) -> [("a", "b"), ("a", "c"), ("b", "c")]`,
	},
	"combinations_with_replacement": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// combinations_with_replacement(iterable, r)
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			rArg, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			r := int(rArg.Value)

			if r < 0 {
				return &object.List{Elements: []object.Object{}}
			}
			if len(elements) == 0 && r > 0 {
				return &object.List{Elements: []object.Object{}}
			}

			// Generate combinations with replacement
			result := []object.Object{}
			generateCombinationsWithReplacement(elements, r, 0, []object.Object{}, &result)
			return &object.List{Elements: result}
		},
		HelpText: `combinations_with_replacement(iterable, r) - Generate combinations with replacement

Returns all r-length combinations of elements from iterable (with repetition allowed).

Example:
  itertools.combinations_with_replacement([1, 2], 2) -> [(1, 1), (1, 2), (2, 2)]
  itertools.combinations_with_replacement("ab", 2) -> [("a", "a"), ("a", "b"), ("b", "b")]`,
	},
	"groupby": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// groupby(iterable[, key]) - Group consecutive elements
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			var keyFunc *object.Builtin
			if len(args) == 2 {
				var ok bool
				keyFunc, ok = args[1].(*object.Builtin)
				if !ok {
					return errors.NewError("groupby() key must be a builtin function")
				}
			}

			if len(elements) == 0 {
				return &object.List{Elements: []object.Object{}}
			}

			result := []object.Object{}
			var currentKey object.Object
			var currentGroup []object.Object

			for i, elem := range elements {
				var key object.Object
				if keyFunc != nil {
					key = keyFunc.Fn(ctx, object.NewKwargs(nil), elem)
					if isError(key) {
						return key
					}
				} else {
					key = elem
				}

				if i == 0 {
					currentKey = key
					currentGroup = []object.Object{elem}
				} else if objectsEqual(key, currentKey) {
					currentGroup = append(currentGroup, elem)
				} else {
					// Save current group and start new one
					result = append(result, &object.Tuple{Elements: []object.Object{
						currentKey,
						&object.List{Elements: currentGroup},
					}})
					currentKey = key
					currentGroup = []object.Object{elem}
				}
			}

			// Don't forget the last group
			if len(currentGroup) > 0 {
				result = append(result, &object.Tuple{Elements: []object.Object{
					currentKey,
					&object.List{Elements: currentGroup},
				}})
			}

			return &object.List{Elements: result}
		},
		HelpText: `groupby(iterable[, key]) - Group consecutive elements

Groups consecutive elements that have the same key value.
Returns list of (key, group) tuples where group is a list.

Example:
  itertools.groupby([1, 1, 2, 2, 3]) -> [(1, [1, 1]), (2, [2, 2]), (3, [3])]
  itertools.groupby(["aa", "ab", "ba"], lambda x: x[0]) -> [("a", ["aa", "ab"]), ("b", ["ba"])]`,
	},
	"accumulate": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// accumulate(iterable[, func]) - Running totals
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			if len(elements) == 0 {
				return &object.List{Elements: []object.Object{}}
			}

			var accumFunc *object.Builtin
			if len(args) == 2 {
				var ok bool
				accumFunc, ok = args[1].(*object.Builtin)
				if !ok {
					return errors.NewError("accumulate() func must be a builtin function")
				}
			}

			result := []object.Object{elements[0]}
			accumulator := elements[0]

			for i := 1; i < len(elements); i++ {
				if accumFunc != nil {
					accumulator = accumFunc.Fn(ctx, object.NewKwargs(nil), accumulator, elements[i])
					if isError(accumulator) {
						return accumulator
					}
				} else {
					// Default: addition
					accumulator = addObjects(accumulator, elements[i])
					if isError(accumulator) {
						return accumulator
					}
				}
				result = append(result, accumulator)
			}
			return &object.List{Elements: result}
		},
		HelpText: `accumulate(iterable[, func]) - Running totals/accumulation

Returns list of accumulated values. Default is sum, but can provide custom function.

Example:
  itertools.accumulate([1, 2, 3, 4]) -> [1, 3, 6, 10]
  itertools.accumulate([1, 2, 3], operator.mul) -> [1, 2, 6]`,
	},
	"filterfalse": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// filterfalse(predicate, iterable) - Filter elements where predicate is false
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			pred, ok := args[0].(*object.Builtin)
			if !ok {
				return errors.NewError("filterfalse() predicate must be a builtin function")
			}
			var elements []object.Object
			switch a := args[1].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				return errors.NewTypeError("iterable", args[1].Type().String())
			}
			result := []object.Object{}
			for _, elem := range elements {
				res := pred.Fn(ctx, object.NewKwargs(nil), elem)
				if isError(res) {
					return res
				}
				if !isTruthy(res) {
					result = append(result, elem)
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `filterfalse(predicate, iterable) - Filter elements where predicate is false

Returns elements for which the predicate returns false.

Example:
  itertools.filterfalse(lambda x: x % 2, [1, 2, 3, 4]) -> [2, 4]`,
	},
	"starmap": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// starmap(func, iterable) - Apply function to argument tuples
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			fn, ok := args[0].(*object.Builtin)
			if !ok {
				return errors.NewError("starmap() func must be a builtin function")
			}
			var elements []object.Object
			switch a := args[1].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				return errors.NewTypeError("iterable", args[1].Type().String())
			}
			result := []object.Object{}
			for _, elem := range elements {
				var fnArgs []object.Object
				switch e := elem.(type) {
				case *object.List:
					fnArgs = e.Elements
				case *object.Tuple:
					fnArgs = e.Elements
				default:
					return errors.NewError("starmap() iterable must contain sequences")
				}
				res := fn.Fn(ctx, object.NewKwargs(nil), fnArgs...)
				if isError(res) {
					return res
				}
				result = append(result, res)
			}
			return &object.List{Elements: result}
		},
		HelpText: `starmap(func, iterable) - Apply function to argument tuples

Applies function using elements of each tuple as arguments.

Example:
  itertools.starmap(pow, [(2, 3), (3, 2)]) -> [8, 9]`,
	},
	"compress": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// compress(data, selectors) - Filter data based on selectors
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var data []object.Object
			switch a := args[0].(type) {
			case *object.List:
				data = a.Elements
			case *object.Tuple:
				data = a.Elements
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}
			var selectors []object.Object
			switch a := args[1].(type) {
			case *object.List:
				selectors = a.Elements
			case *object.Tuple:
				selectors = a.Elements
			default:
				return errors.NewTypeError("iterable", args[1].Type().String())
			}

			result := []object.Object{}
			minLen := len(data)
			if len(selectors) < minLen {
				minLen = len(selectors)
			}
			for i := 0; i < minLen; i++ {
				if isTruthy(selectors[i]) {
					result = append(result, data[i])
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `compress(data, selectors) - Filter data based on selectors

Returns elements from data where corresponding selector is truthy.

Example:
  itertools.compress([1, 2, 3, 4], [True, False, True, False]) -> [1, 3]
  itertools.compress("abcd", [1, 0, 1, 0]) -> ["a", "c"]`,
	},
	"pairwise": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// pairwise(iterable) - Return successive overlapping pairs
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			if len(elements) < 2 {
				return &object.List{Elements: []object.Object{}}
			}

			result := make([]object.Object, len(elements)-1)
			for i := 0; i < len(elements)-1; i++ {
				result[i] = &object.Tuple{Elements: []object.Object{elements[i], elements[i+1]}}
			}
			return &object.List{Elements: result}
		},
		HelpText: `pairwise(iterable) - Return successive overlapping pairs

Returns consecutive pairs from the iterable.

Example:
  itertools.pairwise([1, 2, 3, 4]) -> [(1, 2), (2, 3), (3, 4)]
  itertools.pairwise("abc") -> [("a", "b"), ("b", "c")]`,
	},
	"batched": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// batched(iterable, n) - Batch elements into tuples of size n
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			n, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			if n.Value <= 0 {
				return errors.NewError("n must be positive")
			}

			batchSize := int(n.Value)
			result := []object.Object{}
			for i := 0; i < len(elements); i += batchSize {
				end := i + batchSize
				if end > len(elements) {
					end = len(elements)
				}
				batch := make([]object.Object, end-i)
				copy(batch, elements[i:end])
				result = append(result, &object.Tuple{Elements: batch})
			}
			return &object.List{Elements: result}
		},
		HelpText: `batched(iterable, n) - Batch elements into tuples of size n

Groups elements into batches of n elements each.

Example:
  itertools.batched([1, 2, 3, 4, 5], 2) -> [(1, 2), (3, 4), (5,)]
  itertools.batched("abcdef", 3) -> [("a", "b", "c"), ("d", "e", "f")]`,
	},
}, nil, "Python-compatible itertools library for iteration utilities")

// Helper functions for permutations and combinations

func generatePermutations(elements []object.Object, r int, current []object.Object, used []bool, result *[]object.Object) {
	if len(current) == r {
		tuple := make([]object.Object, r)
		copy(tuple, current)
		*result = append(*result, &object.Tuple{Elements: tuple})
		return
	}
	for i := 0; i < len(elements); i++ {
		if !used[i] {
			used[i] = true
			current = append(current, elements[i])
			generatePermutations(elements, r, current, used, result)
			current = current[:len(current)-1]
			used[i] = false
		}
	}
}

func generateCombinations(elements []object.Object, r int, start int, current []object.Object, result *[]object.Object) {
	if len(current) == r {
		tuple := make([]object.Object, r)
		copy(tuple, current)
		*result = append(*result, &object.Tuple{Elements: tuple})
		return
	}
	for i := start; i < len(elements); i++ {
		current = append(current, elements[i])
		generateCombinations(elements, r, i+1, current, result)
		current = current[:len(current)-1]
	}
}

func generateCombinationsWithReplacement(elements []object.Object, r int, start int, current []object.Object, result *[]object.Object) {
	if len(current) == r {
		tuple := make([]object.Object, r)
		copy(tuple, current)
		*result = append(*result, &object.Tuple{Elements: tuple})
		return
	}
	for i := start; i < len(elements); i++ {
		current = append(current, elements[i])
		generateCombinationsWithReplacement(elements, r, i, current, result)
		current = current[:len(current)-1]
	}
}

// Helper function to compare objects for equality
func objectsEqual(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch av := a.(type) {
	case *object.Integer:
		if bv, ok := b.(*object.Integer); ok {
			return av.Value == bv.Value
		}
	case *object.Float:
		if bv, ok := b.(*object.Float); ok {
			return av.Value == bv.Value
		}
	case *object.String:
		if bv, ok := b.(*object.String); ok {
			return av.Value == bv.Value
		}
	case *object.Boolean:
		if bv, ok := b.(*object.Boolean); ok {
			return av.Value == bv.Value
		}
	case *object.Null:
		_, ok := b.(*object.Null)
		return ok
	}
	return a.Inspect() == b.Inspect()
}

// Helper function to add two objects (for accumulate default)
func addObjects(a, b object.Object) object.Object {
	switch av := a.(type) {
	case *object.Integer:
		switch bv := b.(type) {
		case *object.Integer:
			return object.NewInteger(av.Value + bv.Value)
		case *object.Float:
			return &object.Float{Value: float64(av.Value) + bv.Value}
		}
	case *object.Float:
		switch bv := b.(type) {
		case *object.Integer:
			return &object.Float{Value: av.Value + float64(bv.Value)}
		case *object.Float:
			return &object.Float{Value: av.Value + bv.Value}
		}
	case *object.String:
		if bv, ok := b.(*object.String); ok {
			return &object.String{Value: av.Value + bv.Value}
		}
	case *object.List:
		if bv, ok := b.(*object.List); ok {
			newElements := make([]object.Object, len(av.Elements)+len(bv.Elements))
			copy(newElements, av.Elements)
			copy(newElements[len(av.Elements):], bv.Elements)
			return &object.List{Elements: newElements}
		}
	}
	return errors.NewError("cannot add %s and %s", a.Type(), b.Type())
}

// isError and isTruthy helpers (same as evaluator)
func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func isTruthy(obj object.Object) bool {
	switch v := obj.(type) {
	case *object.Null:
		return false
	case *object.Boolean:
		return v.Value
	case *object.Integer:
		return v.Value != 0
	case *object.Float:
		return v.Value != 0
	case *object.String:
		return len(v.Value) > 0
	case *object.List:
		return len(v.Elements) > 0
	case *object.Dict:
		return len(v.Pairs) > 0
	default:
		return true
	}
}
