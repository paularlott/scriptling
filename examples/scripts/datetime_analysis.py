import datetime
import time

# Current capabilities (Python-compatible)
print("=== Python-compatible datetime library capabilities ===")

# Basic formatting using datetime.datetime class
print("1. Current time:", datetime.datetime.now())
print("2. UTC time:", datetime.datetime.utcnow())

# For custom formatting, use strftime with a timestamp
now_ts = time.time()
print("3. Custom format:", datetime.datetime.strftime("%A, %B %d, %Y at %I:%M %p", now_ts))

# Parsing and formatting
dt = datetime.datetime.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
print("4. Parsed datetime:", dt)
print("5. Formatted back:", datetime.datetime.strftime("%Y-%m-%d %H:%M:%S", dt.timestamp()))

# Using a numeric timestamp
ts = 1705285845.0
print("6. From timestamp:", datetime.datetime.fromtimestamp(ts))

print("\n=== Additional features ===")
print("7. Date arithmetic with timedelta:", ts + datetime.timedelta(days=1))
