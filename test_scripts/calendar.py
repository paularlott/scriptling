def is_leap_year(year):
    return year % 4 == 0 and (year % 100 != 0 or year % 400 == 0)

def days_in_month(year, month):
    days = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
    if month == 2 and is_leap_year(year):
        return 29
    return days[month - 1]

import datetime
import time

def is_leap_year(year):
    return year % 4 == 0 and (year % 100 != 0 or year % 400 == 0)

def days_in_month(year, month):
    days = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
    if month == 2 and is_leap_year(year):
        return 29
    return days[month - 1]

def generate_calendar(year, month):
    month_names = ["", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"]
    month_name = month_names[month] + " " + str(year)
    header = month_name
    days_header = "Mo Tu We Th Fr Sa Su"

    calendar_lines = [header, "", days_header, ""]

    if month < 10:
        month_str = "0" + str(month)
    else:
        month_str = str(month)
    first_of_month = str(year) + "-" + month_str + "-01"
    first_timestamp = datetime.strptime(first_of_month, "%Y-%m-%d")
    first_time_tuple = time.localtime(first_timestamp)
    start_weekday = (first_time_tuple[6] + 6) % 7

    day = 1
    num_days = days_in_month(year, month)

    week_line = ""
    for i in range(7):
        if i < start_weekday:
            week_line += "   "
        else:
            if day < 10:
                week_line += " " + str(day) + " "
            else:
                week_line += str(day) + " "
            day += 1
    calendar_lines.append(week_line.rstrip())

    while day <= num_days:
        week_line = ""
        for i in range(7):
            if day <= num_days:
                if day < 10:
                    week_line += " " + str(day) + " "
                else:
                    week_line += str(day) + " "
                day += 1
            else:
                week_line += "   "
        calendar_lines.append(week_line.rstrip())

    return "\n".join(calendar_lines)

year = 2026
month = 2
calendar = generate_calendar(year, month)
print(calendar)
print("\nCalendar generated successfully!")

# Example usage
year = 2026
month = 2
calendar = generate_calendar(year, month)
print(calendar)
print("\nCalendar generated successfully!")