# statistics Library

The `statistics` library provides functions for calculating mathematical statistics of numeric data, compatible with Python's `statistics` module.

## Import

```python
import statistics
```

## Available Functions

| Function               | Description                              |
| ---------------------- | ---------------------------------------- |
| `mean(data)`           | Arithmetic mean (average)                |
| `fmean(data)`          | Float mean (same as mean, returns float) |
| `geometric_mean(data)` | Geometric mean                           |
| `harmonic_mean(data)`  | Harmonic mean                            |
| `median(data)`         | Median (middle value)                    |
| `mode(data)`           | Mode (most common value)                 |
| `stdev(data)`          | Sample standard deviation                |
| `variance(data)`       | Sample variance                          |

## Functions

### Averages

#### mean(data)

Calculate the arithmetic mean (average) of data.

```python
statistics.mean([1, 2, 3, 4, 5])  # Returns 3.0
statistics.mean([10.5, 20.5, 30.5])  # Returns 20.5
```

#### fmean(data)

Calculate the arithmetic mean of data (same as mean, returns float).

```python
statistics.fmean([1, 2, 3, 4, 5])  # Returns 3.0
```

#### geometric_mean(data)

Calculate the geometric mean of data.

```python
statistics.geometric_mean([1, 2, 4, 8])  # Returns ~2.83
statistics.geometric_mean([1, 3, 9, 27])  # Returns 5.196...
```

#### harmonic_mean(data)

Calculate the harmonic mean of data.

```python
statistics.harmonic_mean([1, 2, 4])  # Returns ~1.71
```

### Central Tendency

#### median(data)

Calculate the median (middle value) of data.

```python
statistics.median([1, 3, 5, 7, 9])  # Returns 5.0 (odd count)
statistics.median([1, 2, 3, 4])  # Returns 2.5 (even count)
```

#### mode(data)

Calculate the mode (most common value) of data.

```python
statistics.mode([1, 2, 2, 3, 3, 3])  # Returns 3
statistics.mode(["a", "b", "b"])  # Returns "b"
```

### Measures of Spread

#### variance(data)

Calculate the sample variance of data.

```python
data = [2, 4, 4, 4, 5, 5, 7, 9]
statistics.variance(data)  # Returns ~4.57
```

#### pvariance(data)

Calculate the population variance of data.

```python
data = [2, 4, 4, 4, 5, 5, 7, 9]
statistics.pvariance(data)  # Returns 4.0
```

#### stdev(data)

Calculate the sample standard deviation.

```python
data = [2, 4, 4, 4, 5, 5, 7, 9]
statistics.stdev(data)  # Returns ~2.14
```

#### pstdev(data)

Calculate the population standard deviation.

```python
data = [2, 4, 4, 4, 5, 5, 7, 9]
statistics.pstdev(data)  # Returns 2.0
```

## Examples

### Basic Statistics

```python
import statistics

grades = [85, 90, 78, 92, 88, 76, 95, 89]

avg = statistics.mean(grades)
med = statistics.median(grades)
std = statistics.stdev(grades)

print(f"Average: {avg}")
print(f"Median: {med}")
print(f"Std Dev: {std}")
```

### Comparing Sample vs Population Statistics

```python
import statistics

data = [2, 4, 4, 4, 5, 5, 7, 9]

# Sample statistics (use when data is a sample)
sample_var = statistics.variance(data)
sample_std = statistics.stdev(data)

# Population statistics (use when data is the entire population)
pop_var = statistics.pvariance(data)
pop_std = statistics.pstdev(data)
```

### Finding the Mode

```python
import statistics

responses = ["yes", "no", "yes", "yes", "no", "yes"]
most_common = statistics.mode(responses)  # Returns "yes"
```

## Notes

- All functions require at least one data point (except variance/stdev which require at least 2)
- The `mode()` function returns the first mode encountered if there are multiple modes
- For sample statistics, use `variance()` and `stdev()`
- For population statistics, use `pvariance()` and `pstdev()`

## Python Compatibility

This library implements a subset of Python's `statistics` module:

| Function       | Supported |
| -------------- | --------- |
| mean           | ✅        |
| fmean          | ✅        |
| geometric_mean | ✅        |
| harmonic_mean  | ✅        |
| median         | ✅        |
| median_low     | ❌        |
| median_high    | ❌        |
| median_grouped | ❌        |
| mode           | ✅        |
| multimode      | ❌        |
| variance       | ✅        |
| pvariance      | ✅        |
| stdev          | ✅        |
| pstdev         | ✅        |
| quantiles      | ❌        |
| NormalDist     | ❌        |
