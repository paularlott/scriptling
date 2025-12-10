# Test datetime.date functionality
import datetime

# Test datetime.date constructor
d = datetime.date(2025, 12, 25)
assert d.year() == 2025, f"Expected year 2025, got {d.year()}"
assert d.month() == 12, f"Expected month 12, got {d.month()}"
assert d.day() == 25, f"Expected day 25, got {d.day()}"

# Test weekday (Monday=0, Sunday=6)
# December 25, 2025 is a Thursday (weekday=3)
assert d.weekday() == 3, f"Expected Thursday (3), got {d.weekday()}"

# Test isoweekday (Monday=1, Sunday=7)
assert d.isoweekday() == 4, f"Expected Thursday (4), got {d.isoweekday()}"

# Test date.today()
today = datetime.date.today()
assert today.year() >= 2024, f"Expected year >= 2024, got {today.year()}"

# Test replace method
d2 = d.replace(year=2026, month=6)
assert d2.year() == 2026, f"Expected replaced year 2026, got {d2.year()}"
assert d2.month() == 6, f"Expected replaced month 6, got {d2.month()}"
assert d2.day() == 25, f"Expected day to remain 25, got {d2.day()}"

# Test from datetime import date
from datetime import date
d3 = date(2025, 7, 4)
assert d3.year() == 2025, f"Expected year 2025, got {d3.year()}"
assert d3.month() == 7, f"Expected month 7, got {d3.month()}"
assert d3.day() == 4, f"Expected day 4, got {d3.day()}"

# Test date.today() via import
today2 = date.today()
assert today2.year() >= 2024, f"Expected year >= 2024, got {today2.year()}"

# Test strftime on date
formatted = d.strftime("%Y-%m-%d")
assert formatted == "2025-12-25", f"Expected '2025-12-25', got '{formatted}'"

# Test isoformat
iso = d.isoformat()
# isoformat includes time portion for datetime objects
assert "2025-12-25" in iso, f"Expected '2025-12-25' in isoformat, got '{iso}'"

# Test datetime comparison with dates
d4 = date(2025, 1, 1)
d5 = date(2025, 12, 31)
assert d4 < d5, "January 1 should be less than December 31"
assert d5 > d4, "December 31 should be greater than January 1"

# Test date arithmetic with timedelta
from datetime import timedelta
d6 = date(2025, 12, 25)
d7 = d6 + timedelta(days=7)
assert d7.year() == 2026, f"Expected year 2026 after adding 7 days, got {d7.year()}"
assert d7.month() == 1, f"Expected month 1 after adding 7 days, got {d7.month()}"
assert d7.day() == 1, f"Expected day 1 after adding 7 days, got {d7.day()}"

print("All datetime.date tests passed!")
