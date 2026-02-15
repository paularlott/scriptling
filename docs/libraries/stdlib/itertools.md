# itertools Library

The `itertools` library provides Python-compatible iteration utilities for efficient looping and combinatorial operations.

## Import

```python
import itertools
```

## Available Functions

| Function                                  | Description                              |
| ----------------------------------------- | ---------------------------------------- |
| `chain(*iterables)`                       | Chain multiple iterables together        |
| `cycle(iterable, n)`                      | Cycle through iterable n times           |
| `repeat(elem, n)`                         | Repeat element n times                   |
| `zip_longest(*iterables, fillvalue=None)` | Zip with fill value                      |
| `count(start, stop[, step])`              | Generate sequence of numbers             |
| `islice(iterable, ...)`                   | Slice an iterable by indices             |
| `takewhile(predicate, iterable)`          | Take elements while predicate is true    |
| `dropwhile(predicate, iterable)`          | Drop elements while predicate is true    |
| `filterfalse(predicate, iterable)`        | Return elements where predicate is false |
| `compress(data, selectors)`               | Filter data based on selectors           |
| `permutations(iterable[, r])`             | Generate all permutations                |
| `combinations(iterable, r)`               | Generate all combinations                |
| `product(*iterables)`                     | Cartesian product of iterables           |

## Functions

### Chaining and Combining

#### `chain(*iterables)`

Chain multiple iterables together into a single sequence.

```python
itertools.chain([1, 2], [3, 4])  # [1, 2, 3, 4]
itertools.chain("ab", "cd")      # ["a", "b", "c", "d"]
```

#### `cycle(iterable, n)`

Cycle through an iterable n times.

```python
itertools.cycle([1, 2], 3)  # [1, 2, 1, 2, 1, 2]
```

**Note:** Unlike Python's infinite `cycle()`, this requires specifying a count.

#### `repeat(elem, n)`

Repeat an element n times.

```python
itertools.repeat("x", 3)  # ["x", "x", "x"]
itertools.repeat(0, 5)    # [0, 0, 0, 0, 0]
```

#### `zip_longest(*iterables, fillvalue=None)`

Zip iterables together, filling shorter ones with fillvalue.

```python
itertools.zip_longest([1, 2, 3], ["a", "b"])
# [(1, "a"), (2, "b"), (3, None)]

itertools.zip_longest([1, 2], ["a"], fillvalue="-")
# [(1, "a"), (2, "-")]
```

### Slicing and Filtering

#### `count(start, stop[, step])`

Generate a sequence of numbers (like range).

```python
itertools.count(0, 5)       # [0, 1, 2, 3, 4]
itertools.count(0, 10, 2)   # [0, 2, 4, 6, 8]
itertools.count(5, 0, -1)   # [5, 4, 3, 2, 1]
```

#### `islice(iterable, stop)` / `islice(iterable, start, stop[, step])`

Slice an iterable by indices.

```python
itertools.islice([0, 1, 2, 3, 4], 3)        # [0, 1, 2]
itertools.islice([0, 1, 2, 3, 4], 1, 4)     # [1, 2, 3]
itertools.islice([0, 1, 2, 3, 4], 0, 5, 2)  # [0, 2, 4]
```

#### `takewhile(predicate, iterable)`

Take elements while predicate is true.

```python
itertools.takewhile(lambda x: x < 5, [1, 3, 5, 2, 4])
# [1, 3]
```

#### `dropwhile(predicate, iterable)`

Drop elements while predicate is true, then return the rest.

```python
itertools.dropwhile(lambda x: x < 5, [1, 3, 5, 2, 4])
# [5, 2, 4]
```

#### `filterfalse(predicate, iterable)`

Return elements where predicate is false.

```python
itertools.filterfalse(lambda x: x % 2, [1, 2, 3, 4])
# [2, 4]  (even numbers)
```

#### `compress(data, selectors)`

Filter data based on truthy selectors.

```python
itertools.compress([1, 2, 3, 4], [True, False, True, False])
# [1, 3]
```

### Combinatorics

#### `product(*iterables)`

Cartesian product of iterables.

```python
itertools.product([1, 2], ["a", "b"])
# [(1, "a"), (1, "b"), (2, "a"), (2, "b")]
```

#### `permutations(iterable[, r])`

Generate r-length permutations (default: full length).

```python
itertools.permutations([1, 2, 3], 2)
# [(1, 2), (1, 3), (2, 1), (2, 3), (3, 1), (3, 2)]

itertools.permutations("ab")
# [("a", "b"), ("b", "a")]
```

#### `combinations(iterable, r)`

Generate r-length combinations (no repetition).

```python
itertools.combinations([1, 2, 3], 2)
# [(1, 2), (1, 3), (2, 3)]
```

#### `combinations_with_replacement(iterable, r)`

Generate r-length combinations (with repetition).

```python
itertools.combinations_with_replacement([1, 2], 2)
# [(1, 1), (1, 2), (2, 2)]
```

### Grouping and Accumulation

#### `groupby(iterable[, key])`

Group consecutive elements with the same key.

```python
itertools.groupby([1, 1, 2, 2, 3])
# [(1, [1, 1]), (2, [2, 2]), (3, [3])]

itertools.groupby(["aa", "ab", "ba"], lambda x: x[0])
# [("a", ["aa", "ab"]), ("b", ["ba"])]
```

#### `accumulate(iterable[, func])`

Running totals/accumulation.

```python
itertools.accumulate([1, 2, 3, 4])
# [1, 3, 6, 10]  (running sum)
```

### Pairing and Batching

#### `pairwise(iterable)`

Return successive overlapping pairs.

```python
itertools.pairwise([1, 2, 3, 4])
# [(1, 2), (2, 3), (3, 4)]
```

#### `batched(iterable, n)`

Group elements into batches of size n.

```python
itertools.batched([1, 2, 3, 4, 5], 2)
# [(1, 2), (3, 4), (5,)]
```

### Function Application

#### `starmap(func, iterable)`

Apply function to argument tuples.

```python
itertools.starmap(pow, [(2, 3), (3, 2)])
# [8, 9]
```

## Examples

### Generate all 2-letter combinations

```python
import itertools

letters = "abc"
combos = itertools.combinations(letters, 2)
# [("a", "b"), ("a", "c"), ("b", "c")]
```

### Flatten nested lists

```python
import itertools

nested = [[1, 2], [3, 4], [5, 6]]
flat = itertools.chain(nested[0], nested[1], nested[2])
# [1, 2, 3, 4, 5, 6]
```

### Running total

```python
import itertools

sales = [100, 200, 150, 300]
running_total = itertools.accumulate(sales)
# [100, 300, 450, 750]
```

### All possible dice rolls

```python
import itertools

dice = [1, 2, 3, 4, 5, 6]
two_dice = itertools.product(dice, dice)
# All 36 combinations of two dice
```
