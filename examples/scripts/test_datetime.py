# Test: Datetime library
# Tests the datetime library functions

print("=== Testing Datetime Library ===\n")

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

# Test datetime.add_days()
print("9. datetime.add_days() - Add/subtract days")
original_ts = 1705314645.0  # 2024-01-15 10:30:45 UTC
future_ts = datetime.add_days(original_ts, 7)
past_ts = datetime.add_days(original_ts, -3)
print(f"Original: {datetime.fromtimestamp(original_ts)}")
print(f"Plus 7 days: {datetime.fromtimestamp(future_ts)}")
print(f"Minus 3 days: {datetime.fromtimestamp(past_ts)}")
print()

# Test datetime.add_hours()
print("10. datetime.add_hours() - Add/subtract hours")
future_hour = datetime.add_hours(original_ts, 5)
past_hour = datetime.add_hours(original_ts, -2)
print(f"Original: {datetime.fromtimestamp(original_ts)}")
print(f"Plus 5 hours: {datetime.fromtimestamp(future_hour)}")
print(f"Minus 2 hours: {datetime.fromtimestamp(past_hour)}")
print()

# Test datetime.add_minutes()
print("11. datetime.add_minutes() - Add/subtract minutes")
future_min = datetime.add_minutes(original_ts, 30)
past_min = datetime.add_minutes(original_ts, -15)
print(f"Original: {datetime.fromtimestamp(original_ts)}")
print(f"Plus 30 minutes: {datetime.fromtimestamp(future_min)}")
print(f"Minus 15 minutes: {datetime.fromtimestamp(past_min)}")
print()

# Test datetime.add_seconds()
print("12. datetime.add_seconds() - Add/subtract seconds")
future_sec = datetime.add_seconds(original_ts, 45)
past_sec = datetime.add_seconds(original_ts, -30)
print(f"Original: {datetime.fromtimestamp(original_ts)}")
print(f"Plus 45 seconds: {datetime.fromtimestamp(future_sec)}")
print(f"Minus 30 seconds: {datetime.fromtimestamp(past_sec)}")
print()

# Test error cases
print("13. Error handling")
try:
    # Wrong number of arguments
    result = datetime.now("format1", "format2")
    print(f"ERROR: Should have failed: {result}")
except Exception as e:
    print(f"✓ Correctly caught error for too many args: {e}")

try:
    # Wrong argument type
    result = datetime.strptime(123, "%Y-%m-%d")
    print(f"ERROR: Should have failed: {result}")
except Exception as e:
    print(f"✓ Correctly caught error for wrong arg type: {e}")

try:
    # Wrong argument type for add_days
    result = datetime.add_days("not_a_timestamp", 5)
    print(f"ERROR: Should have failed: {result}")
except Exception as e:
    print(f"✓ Correctly caught error for wrong timestamp type: {e}")

try:
    # Wrong argument type for days
    result = datetime.add_days(original_ts, "not_a_number")
    print(f"ERROR: Should have failed: {result}")
except Exception as e:
    print(f"✓ Correctly caught error for wrong days type: {e}")

print("\n✓ All datetime library tests completed!")