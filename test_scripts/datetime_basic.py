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

True