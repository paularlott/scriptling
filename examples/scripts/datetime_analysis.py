import datetime

# Current capabilities
print("=== Current datetime library capabilities ===")

# Basic formatting
print("1. Current time:", datetime.now())
print("2. UTC time:", datetime.utcnow())
print("3. Today:", datetime.today())
print("4. Custom format:", datetime.now("%A, %B %d, %Y at %I:%M %p"))

# Parsing and formatting
dt = datetime.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
print("5. Parsed datetime:", dt)
print("6. Formatted back:", datetime.strftime("%Y-%m-%d %H:%M:%S", dt))

# Using a numeric timestamp
ts = 1705285845.0
print("7. From timestamp:", datetime.fromtimestamp(ts))

print("\n=== Additional features ===")
print("8. ISO format:", datetime.isoformat())
print("9. Date arithmetic with timedelta:", ts + datetime.timedelta(days=1))
