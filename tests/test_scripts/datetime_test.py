import datetime

# Test datetime.datetime functions (Python-compatible)
now = datetime.datetime.now()
# Check that we got a datetime by verifying timestamp() method exists
assert now.timestamp() > 0

timestamp = 1705312245
formatted = datetime.datetime.strftime("%Y-%m-%d", timestamp)
assert formatted == "2024-01-15"

date_str = "2024-01-15 10:30:45"
parsed = datetime.datetime.strptime(date_str, "%Y-%m-%d %H:%M:%S")
# Verify the parsed datetime has a valid timestamp
assert parsed.timestamp() > 0

# Test timedelta with keyword arguments (Python-compatible)
one_day = datetime.timedelta(days=1)
two_hours = datetime.timedelta(hours=2)
one_week = datetime.timedelta(weeks=1)
combined = datetime.timedelta(days=1, hours=2, minutes=30)

# Verify the calculations (total_seconds, like Python)
assert one_day.total_seconds() == 86400
assert two_hours.total_seconds() == 7200
assert one_week.total_seconds() == 604800
assert combined.total_seconds() == 95400
assert str(combined) == "1 day, 2:30:00"