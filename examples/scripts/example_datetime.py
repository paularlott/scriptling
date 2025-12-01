# Example: Datetime library
# Tests the datetime library functions

print("Datetime Library ===\n")

# Import the datetime library
import datetime

# Test datetime.now()
print("1. datetime.now() - Current local datetime")
now = datetime.now()
print(f"datetime.now(): {now}")
print(f"Type: {type(now)}")
print()

# Test datetime.now() with custom format
print("2. datetime.now() with custom format")
now_custom = datetime.now("%Y-%m-%d %H:%M:%S")
print(f"datetime.now('%Y-%m-%d %H:%M:%S'): {now_custom}")
print()

# Test datetime.utcnow()
print("3. datetime.utcnow() - Current UTC datetime")
utc_now = datetime.utcnow()
print(f"datetime.utcnow(): {utc_now}")
print()

# Test datetime.today()
print("4. datetime.today() - Today's date")
today = datetime.today()
print(f"datetime.today(): {today}")
print()

# Test datetime.today() with custom format
print("5. datetime.today() with custom format")
today_custom = datetime.today("%A, %B %d, %Y")
print(f"datetime.today('%A, %B %d, %Y'): {today_custom}")
print()

# Test datetime.strptime() and datetime.strftime()
print("6. datetime.strptime() and datetime.strftime()")
date_str = "2024-01-15 10:30:45"
timestamp = datetime.strptime(date_str, "%Y-%m-%d %H:%M:%S")
print(f"datetime.strptime('{date_str}', '%Y-%m-%d %H:%M:%S'): {timestamp}")

formatted_back = datetime.strftime("%Y-%m-%d %H:%M:%S", timestamp)
print(f"datetime.strftime('%Y-%m-%d %H:%M:%S', {timestamp}): {formatted_back}")
print()

# Test datetime.fromtimestamp()
print("7. datetime.fromtimestamp()")
timestamp_val = 1705314645.0  # 2024-01-15 10:30:45 UTC
from_ts = datetime.fromtimestamp(timestamp_val)
print(f"datetime.fromtimestamp({timestamp_val}): {from_ts}")

from_ts_custom = datetime.fromtimestamp(timestamp_val, "%A, %B %d, %Y at %I:%M %p")
print(f"datetime.fromtimestamp({timestamp_val}, '%A, %B %d, %Y at %I:%M %p'): {from_ts_custom}")
print()

# Test datetime.isoformat()
print("8. datetime.isoformat() - ISO 8601 format")
iso_now = datetime.isoformat()
print(f"datetime.isoformat(): {iso_now}")

iso_specific = datetime.isoformat(timestamp_val)
print(f"datetime.isoformat({timestamp_val}): {iso_specific}")
print()

print("Datetime library examples completed!")