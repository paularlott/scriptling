import datetime

# Test timedelta with keyword arguments (Python-compatible)
one_day = datetime.timedelta(days=1)
two_hours = datetime.timedelta(hours=2)
one_week = datetime.timedelta(weeks=1)
combined = datetime.timedelta(days=1, hours=2, minutes=30)

# Verify the calculations (all return seconds)
# 1 day = 86400 seconds
# 2 hours = 7200 seconds
# 1 week = 604800 seconds
# 1 day + 2 hours + 30 min = 86400 + 7200 + 1800 = 95400 seconds

one_day == 86400 and two_hours == 7200 and one_week == 604800 and combined == 95400
