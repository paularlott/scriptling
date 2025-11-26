import datetime

print("=== New Datetime Library Features ===\n")

# ISO format for APIs
print("1. ISO 8601 format:")
print(f"Current time: {datetime.isoformat()}")
print(f"Specific time: {datetime.isoformat(1705314645.0)}")
print()

# Date arithmetic
print("2. Date arithmetic:")
base_ts = 1705314645.0  # 2024-01-15 10:30:45 UTC
print(f"Base time: {datetime.fromtimestamp(base_ts)}")

# Add days
week_later = datetime.add_days(base_ts, 7)
print(f"+7 days: {datetime.fromtimestamp(week_later)}")

# Add hours  
hour_later = datetime.add_hours(base_ts, 1)
print(f"+1 hour: {datetime.fromtimestamp(hour_later)}")

# Add minutes
min_later = datetime.add_minutes(base_ts, 30)
print(f"+30 mins: {datetime.fromtimestamp(min_later)}")

# Add seconds
sec_later = datetime.add_seconds(base_ts, 45)
print(f"+45 secs: {datetime.fromtimestamp(sec_later)}")

# Negative values work too
day_earlier = datetime.add_days(base_ts, -1)
print(f"-1 day: {datetime.fromtimestamp(day_earlier)}")

print("\n=== Perfect for scheduling and time calculations! ===")
