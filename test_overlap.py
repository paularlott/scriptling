import time
import datetime

# Test timestamp parsing
ts1 = time.mktime(time.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S"))
ts2 = datetime.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
print("Time strptime result:", ts1)
print("Datetime strptime result:", ts2)
print("Are they equal?", ts1 == ts2)

# Test timestamp formatting  
str1 = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(ts1))
str2 = datetime.strftime("%Y-%m-%d %H:%M:%S", ts1)
print("Time strftime result:", str1)
print("Datetime strftime result:", str2)
print("Are they equal?", str1 == str2)
