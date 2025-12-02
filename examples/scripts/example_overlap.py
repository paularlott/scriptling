import time
import datetime

# Test timestamp parsing
# time module uses Unix timestamps (seconds since epoch)
ts1 = time.mktime(time.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S"))
# datetime module returns datetime objects
dt2 = datetime.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
print("Time strptime result (timestamp):", ts1)
print("Datetime strptime result (datetime object):", dt2)

# Convert timestamp to datetime for comparison
dt1 = datetime.fromtimestamp(ts1)
print("Are the datetime values equal?", str(dt1) == str(dt2))

# Test timestamp formatting
str1 = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(ts1))
str2 = datetime.strftime("%Y-%m-%d %H:%M:%S", dt2)
print("Time strftime result:", str1)
print("Datetime strftime result:", str2)
print("Are they equal?", str1 == str2)
