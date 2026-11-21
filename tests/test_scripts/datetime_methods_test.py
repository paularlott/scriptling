from datetime import datetime, date, timedelta

d = datetime.strptime("2024-01-02 03:04:05", "%Y-%m-%d %H:%M:%S")

# datetime.date() gives the date part as a date.
day = d.date()
assert type(day).__name__ == "date"
assert day == date(2024, 1, 2)
assert str(day) == "2024-01-02"
assert day.isoformat() == "2024-01-02"
assert (day.year, day.month, day.day) == (2024, 1, 2)
assert datetime.strptime("2024-01-02", "%Y-%m-%d").date() == date(2024, 1, 2)
assert type(datetime.now().date()).__name__ == "date"
assert (d.date() - date(2024, 1, 1)).days == 1

assert tuple(d.timetuple())[:8] == (2024, 1, 2, 3, 4, 5, 1, 2)
assert tuple(d.isocalendar()) == (2024, 1, 2)
assert tuple(date(2021, 1, 3).isocalendar()) == (2020, 53, 7)
assert d.toordinal() == 738887
assert date(2024, 1, 2).toordinal() == 738887
assert date(1970, 1, 1).toordinal() == 719163
assert date.fromordinal(738887) == date(2024, 1, 2)
assert d.weekday() == 1 and d.isoweekday() == 2

assert type(date.today()).__name__ == "date"
assert date.fromisoformat("2024-01-02").day == 2
assert datetime.fromisoformat("2024-01-02T03:04:05").second == 5

# Dates and timedeltas are hashable and usable as dict keys.
assert {date(2024, 1, 1): 1}[date(2024, 1, 1)] == 1
assert len({timedelta(hours=1), timedelta(minutes=60)}) == 1

# timedelta: positional arguments, arithmetic and comparisons.
assert timedelta(1) == timedelta(days=1)
assert timedelta(1, 30).total_seconds() == 86430.0
assert timedelta(1) < timedelta(2)
assert timedelta(hours=1) + timedelta(minutes=30) == timedelta(seconds=5400)
assert timedelta(hours=1) - timedelta(minutes=30) == timedelta(minutes=30)
assert timedelta(hours=1) * 2 == timedelta(hours=2)
assert 2 * timedelta(hours=1) == timedelta(hours=2)
assert timedelta(hours=1) / 2 == timedelta(minutes=30)
assert timedelta(hours=1) / timedelta(minutes=30) == 2.0
assert -timedelta(hours=1) == timedelta(hours=-1)
assert str(-timedelta(hours=1)) == "-1 day, 23:00:00"
assert str(timedelta(hours=1, minutes=2)) == "1:02:00"

# Arithmetic and comparisons between datetimes, dates and timedeltas.
assert (datetime(2024, 1, 3) - datetime(2024, 1, 1)).days == 2
assert (date(2024, 1, 3) - date(2024, 1, 1)).days == 2
assert date(2024, 1, 3) + timedelta(days=2) == date(2024, 1, 5)
assert datetime(2024, 1, 3) + timedelta(hours=2) == datetime(2024, 1, 3, 2)
assert datetime(2024, 1, 3) - timedelta(hours=2) == datetime(2024, 1, 2, 22)
assert datetime(2024, 1, 1) < datetime(2024, 1, 2)
assert date(2024, 1, 1) < date(2024, 1, 2)
assert max([date(2024, 1, 1), date(2023, 1, 1)]) == date(2024, 1, 1)
assert sorted([datetime(2024, 1, 2), datetime(2024, 1, 1)])[0] == datetime(2024, 1, 1)
assert d.replace(year=2020).year == 2020
assert date(2024, 1, 2).replace(day=5) == date(2024, 1, 5)
assert d.strftime("%Y/%m/%d") == "2024/01/02"
assert day.strftime("%d.%m.%Y") == "02.01.2024"

print("OK")
