import datetime

print("=== Datetime Library Features ===\n")

# ISO format for APIs
print("1. ISO 8601 format:")
print("Current time:", datetime.isoformat())
print("Specific time:", datetime.isoformat(1705314645.0))
print()

# Date arithmetic using timedelta (returns seconds)
print("2. Date arithmetic using timedelta:")
base_ts = 1705314645.0  # 2024-01-15 10:30:45 UTC
print("Base time:", datetime.fromtimestamp(base_ts))

# Add days using timedelta
week_offset = datetime.timedelta(days=7)
week_later = base_ts + week_offset
print("+7 days:", datetime.fromtimestamp(week_later))

# Add hours
hour_offset = datetime.timedelta(hours=1)
hour_later = base_ts + hour_offset
print("+1 hour:", datetime.fromtimestamp(hour_later))

# Add minutes
min_offset = datetime.timedelta(minutes=30)
min_later = base_ts + min_offset
print("+30 mins:", datetime.fromtimestamp(min_later))

# Add seconds
sec_offset = datetime.timedelta(seconds=45)
sec_later = base_ts + sec_offset
print("+45 secs:", datetime.fromtimestamp(sec_later))

# Negative values work too
day_offset = datetime.timedelta(days=-1)
day_earlier = base_ts + day_offset
print("-1 day:", datetime.fromtimestamp(day_earlier))

print("\n=== Perfect for scheduling and time calculations! ===")
