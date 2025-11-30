---
description: "Generate a beautiful ASCII calendar for any month and year"
---

# Generate Monthly Calendar

This skill demonstrates advanced date calculations and string formatting to create a professional-looking ASCII calendar for any given month and year using Scriptling's datetime functions.

## Requirements

- Year: The year for the calendar (e.g., 2024)
- Month: The month number (1-12)

## Scriptling Code

```python
import datetime

def is_leap_year(year):
    """Check if a year is a leap year."""
    return year % 4 == 0 and (year % 100 != 0 or year % 400 == 0)

def days_in_month(year, month):
    """Return the number of days in the given month."""
    days = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
    if month == 2 and is_leap_year(year):
        return 29
    return days[month - 1]

def generate_calendar(year, month):
    """
    Generate an ASCII calendar for the specified year and month.

    Parameters:
    - year: The year (integer)
    - month: The month (1-12)
    """

    # Get month name
    month_names = ["", "January", "February", "March", "April", "May", "June",
                   "July", "August", "September", "October", "November", "December"]
    month_name = f"{month_names[month]} {year}"
    header = month_name.center(20)
    days_header = "Mo Tu We Th Fr Sa Su"

    calendar_lines = [header, "", days_header, ""]

    # Find what day of the week the first day of the month is
    # Use a reference date (2000-01-01 was a Saturday)
    first_of_month = f"{year}-{month:02d}-01"
    first_date = datetime.strptime(first_of_month, "%Y-%m-%d")

    # Calculate day of week (0=Monday, 6=Sunday)
    # Reference: 2000-01-01 was a Saturday (6)
    ref_timestamp = datetime.strptime("2000-01-01", "%Y-%m-%d").timestamp()
    target_timestamp = first_date.timestamp()
    days_diff = int((target_timestamp - ref_timestamp) / 86400)  # 86400 seconds per day
    start_weekday = (6 + days_diff) % 7  # 6 for Saturday reference

    # Build the calendar
    day = 1
    num_days = days_in_month(year, month)

    # First week (may include days from previous month)
    week_line = ""
    for i in range(7):
        if i < start_weekday:
            week_line += "   "  # Empty space for previous month
        else:
            week_line += f"{day:2d} "
            day += 1
    calendar_lines.append(week_line.rstrip())

    # Remaining weeks
    while day <= num_days:
        week_line = ""
        for i in range(7):
            if day <= num_days:
                week_line += f"{day:2d} "
                day += 1
            else:
                week_line += "   "
        calendar_lines.append(week_line.rstrip())

    return "\n".join(calendar_lines)

# Example usage
year = 2025
month = 12
calendar = generate_calendar(year, month)
print(calendar)
print("\nCalendar generated successfully!")
```

## Features

- Displays full month name and year
- Properly aligned days of the week starting from Monday
- Handles leap years correctly
- Clean ASCII art formatting
- Uses only available datetime functions

## Validation

The calendar should:
1. Show the correct month and year in the header
2. Display days starting from Monday
3. Show all days of the month
4. Have proper alignment with each week on a new line
5. Handle February correctly in leap years (like 2024)