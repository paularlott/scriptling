import datetime

# Test basic f-string functionality
name = "Scriptling"
version = 1.0
basic_fstring = f"Hello, {name} v{version}!"
assert basic_fstring == "Hello, Scriptling v1.0!"

# Test f-string with expressions
x = 5
y = 10
expr_fstring = f"Sum: {x + y}, Product: {x * y}"
assert expr_fstring == "Sum: 15, Product: 50"

# Test f-string with format specifiers - integer formatting
day = 5
month = 2
year = 2023

# Test :2d (right-aligned with 2 spaces)
formatted_day = f"{day:2d}"
assert formatted_day == " 5"

# Test :02d (zero-padded with 2 digits)
formatted_month = f"{month:02d}"
assert formatted_month == "02"

# Test :4d (right-aligned with 4 spaces)
formatted_year = f"{year:4d}"
assert formatted_year == "2023"

# Test :04d (zero-padded with 4 digits)
formatted_year_zero = f"{year:04d}"
assert formatted_year_zero == "2023"

# Test combination in date string
date_str = f"{year}-{month:02d}-{day:02d}"
assert date_str == "2023-02-05"

# Test f-string with datetime objects
dt = datetime.strptime("2023-01-01", "%Y-%m-%d")
datetime_fstring = f"Date: {dt}"
assert "2023-01-01" in datetime_fstring

# Test f-string with method calls on datetime
timestamp_fstring = f"Timestamp: {dt.timestamp()}"
assert "Timestamp: 1672531200.0" in timestamp_fstring

# Test f-string with no format specifiers (should work like before)
simple = f"Value: {x}"
assert simple == "Value: 5"

# Test f-string with multiple expressions and formats
multi_format = f"Date: {year:04d}-{month:02d}-{day:02d}, Sum: {x + y:3d}"
assert multi_format == "Date: 2023-02-05, Sum:  15"

# Test float formatting
pi = 3.14159
float_fstring = f"Pi is approximately {pi}"
assert float_fstring == "Pi is approximately 3.14159"

# Test f-string with strings and format specs (should fallback to inspect)
test_str = "hello"
str_format = f"String: {test_str:>10}"
assert str_format == "String: hello"  # fallback to inspect since we only implemented int formatting

# Test edge cases
zero = 0
zero_format = f"{zero:02d}"
assert zero_format == "00"

large_num = 12345
large_format = f"{large_num:02d}"
assert large_format == "12345"  # width smaller than number

True