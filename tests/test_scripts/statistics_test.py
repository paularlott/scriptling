# Test statistics library
import statistics

# Test mean
data = [1, 2, 3, 4, 5]
result = statistics.mean(data)
assert result == 3.0

# Test fmean
result = statistics.fmean(data)
assert result == 3.0

# Test median (odd length)
result = statistics.median([1, 3, 5, 7, 9])
assert result == 5.0

# Test median (even length)
result = statistics.median([1, 2, 3, 4])
assert result == 2.5

# Test mode
result = statistics.mode([1, 2, 2, 3, 3, 3, 4])
assert result == 3

# Test sample variance
data = [2, 4, 4, 4, 5, 5, 7, 9]
result = statistics.variance(data)
assert result > 4.5 and result < 4.6

# Test population variance
result = statistics.pvariance(data)
assert result == 4.0

# Test sample stdev
result = statistics.stdev(data)
assert result > 2.1 and result < 2.2

# Test population stdev
result = statistics.pstdev(data)
assert result == 2.0

# Test geometric mean
result = statistics.geometric_mean([1, 2, 4, 8])
assert result > 2.8 and result < 2.9

# Test harmonic mean
result = statistics.harmonic_mean([1, 2, 4])
assert result > 1.7 and result < 1.8
