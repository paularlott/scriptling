import datetime

# Test datetime functions
now = datetime.now()
assert len(now) > 0

timestamp = 1705312245
formatted = datetime.strftime("%Y-%m-%d", timestamp)
assert formatted == "2024-01-15"

date_str = "2024-01-15 10:30:45"
parsed = datetime.strptime(date_str, "%Y-%m-%d %H:%M:%S")
assert len(str(parsed)) > 0

# Test timedelta with keyword arguments (Python-compatible)
one_day = datetime.timedelta(days=1)
two_hours = datetime.timedelta(hours=2)
one_week = datetime.timedelta(weeks=1)
combined = datetime.timedelta(days=1, hours=2, minutes=30)

# Verify the calculations (all return seconds)
assert one_day == 86400
assert two_hours == 7200
assert one_week == 604800
assert combined == 95400