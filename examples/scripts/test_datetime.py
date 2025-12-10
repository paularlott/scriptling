# Example: Datetime library (Python-compatible)
# Tests the datetime library functions using datetime.datetime.* syntax

print("Datetime Library (Python-compatible) ===\n")

# Import the datetime library
import datetime
import time

# Test datetime.datetime.now()
print("1. datetime.datetime.now() - Current local datetime")
now = datetime.datetime.now()
print(f"datetime.datetime.now(): {now}")
print(f"Type: {type(now)}")
print()

# Test datetime.datetime.utcnow()
print("2. datetime.datetime.utcnow() - Current UTC datetime")
utc_now = datetime.datetime.utcnow()
print(f"datetime.datetime.utcnow(): {utc_now}")
print()

# Test datetime.datetime.strptime() and datetime.datetime.strftime()
print("3. datetime.datetime.strptime() and datetime.datetime.strftime()")
date_str = "2024-01-15 10:30:45"
parsed = datetime.datetime.strptime(date_str, "%Y-%m-%d %H:%M:%S")
print(f"datetime.datetime.strptime('{date_str}', '%Y-%m-%d %H:%M:%S'): {parsed}")

formatted_back = datetime.datetime.strftime("%Y-%m-%d %H:%M:%S", parsed.timestamp())
print(f"datetime.datetime.strftime('%Y-%m-%d %H:%M:%S', timestamp): {formatted_back}")
print()

# Test datetime.datetime.fromtimestamp()
print("4. datetime.datetime.fromtimestamp()")
timestamp_val = 1705314645.0  # 2024-01-15 10:30:45 UTC
from_ts = datetime.datetime.fromtimestamp(timestamp_val)
print(f"datetime.datetime.fromtimestamp({timestamp_val}): {from_ts}")
print()

# Test datetime.timedelta() (module level, Python-compatible)
print("5. datetime.timedelta() - Duration calculations")
one_day = datetime.timedelta(days=1)
print(f"datetime.timedelta(days=1): {one_day} seconds")

two_hours = datetime.timedelta(hours=2)
print(f"datetime.timedelta(hours=2): {two_hours} seconds")

combined = datetime.timedelta(days=1, hours=2, minutes=30)
print(f"datetime.timedelta(days=1, hours=2, minutes=30): {combined} seconds")
print()

# Date arithmetic
print("6. Date arithmetic with timedelta")
now_ts = time.time()
tomorrow_ts = now_ts + datetime.timedelta(days=1)
print(f"Current timestamp: {now_ts}")
print(f"Tomorrow timestamp: {tomorrow_ts}")
print()

print("Datetime library examples completed!")