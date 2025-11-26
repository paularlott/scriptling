# Test: Time library
# Tests the time library functions for Python compatibility

print("=== Testing Time Library ===\n")

# Import the time library
import time

# Test time.time()
print("1. time.time() - Current Unix timestamp")
timestamp = time.time()
print(f"time.time(): {timestamp}")
print(f"Type: {type(timestamp)}")
print()

# Test time.sleep()
print("2. time.sleep() - Sleep for 0.1 seconds")
start = time.time()
time.sleep(0.1)
end = time.time()
elapsed = end - start
print(f"Slept for approximately: {elapsed} seconds")
print()

# Test time.perf_counter()
print("3. time.perf_counter() - High-resolution timer")
start_perf = time.perf_counter()
time.sleep(0.05)
end_perf = time.perf_counter()
elapsed_perf = end_perf - start_perf
print(f"High-res elapsed: {elapsed_perf} seconds")
print()

# Test time.localtime()
print("4. time.localtime() - Current time as time tuple")
local_tuple = time.localtime()
print(f"time.localtime(): {local_tuple}")
print(f"Type: {type(local_tuple)}")
if len(local_tuple) >= 9:
    print(f"Year: {local_tuple[0]}, Month: {local_tuple[1]}, Day: {local_tuple[2]}")
    print(f"Hour: {local_tuple[3]}, Minute: {local_tuple[4]}, Second: {local_tuple[5]}")
    print(f"Weekday: {local_tuple[6]}, Yearday: {local_tuple[7]}, DST: {local_tuple[8]}")
print()

# Test time.localtime(timestamp)
print("5. time.localtime(timestamp) - Specific timestamp as time tuple")
specific_tuple = time.localtime(1705314645.0)  # 2024-01-15 10:30:45 UTC
print(f"time.localtime(1705314645.0): {specific_tuple}")
print()

# Test time.gmtime()
print("6. time.gmtime() - Current UTC time as time tuple")
utc_tuple = time.gmtime()
print(f"time.gmtime(): {utc_tuple}")
print()

# Test time.gmtime(timestamp)
print("7. time.gmtime(timestamp) - Specific UTC timestamp as time tuple")
utc_specific = time.gmtime(1705314645.0)
print(f"time.gmtime(1705314645.0): {utc_specific}")
print()

# Test time.mktime()
print("8. time.mktime() - Convert time tuple back to timestamp")
reconstructed = time.mktime(specific_tuple)
print(f"time.mktime({specific_tuple}): {reconstructed}")
print(f"Original timestamp: 1705314645.0, Reconstructed: {reconstructed}")
print()

# Test time.strftime()
print("9. time.strftime() - Format time tuple to string")
formatted = time.strftime("%Y-%m-%d %H:%M:%S", local_tuple)
print(f"time.strftime('%Y-%m-%d %H:%M:%S', local_tuple): {formatted}")
print()

# Test time.strftime() with default (current time)
print("10. time.strftime() with default time tuple")
formatted_default = time.strftime("%Y-%m-%d %H:%M:%S")
print(f"time.strftime('%Y-%m-%d %H:%M:%S'): {formatted_default}")
print()

# Test time.strptime()
print("11. time.strptime() - Parse string to time tuple")
date_str = "2024-01-15 10:30:45"
parsed_tuple = time.strptime(date_str, "%Y-%m-%d %H:%M:%S")
print(f"time.strptime('{date_str}', '%Y-%m-%d %H:%M:%S'): {parsed_tuple}")
print()

# Test time.asctime()
print("12. time.asctime() - Convert time tuple to string")
ascii_time = time.asctime(local_tuple)
print(f"time.asctime(local_tuple): {ascii_time}")
print()

# Test time.asctime() with default
print("13. time.asctime() with default time tuple")
ascii_default = time.asctime()
print(f"time.asctime(): {ascii_default}")
print()

# Test time.ctime()
print("14. time.ctime() - Convert timestamp to string")
ctime_str = time.ctime(timestamp)
print(f"time.ctime({timestamp}): {ctime_str}")
print()

# Test time.ctime() with default
print("15. time.ctime() with default timestamp")
ctime_default = time.ctime()
print(f"time.ctime(): {ctime_default}")
print()

# Test round-trip conversion
print("16. Round-trip conversion test")
original_ts = 1705314645.0
tuple_from_ts = time.localtime(original_ts)
ts_from_tuple = time.mktime(tuple_from_ts)
print(f"Original: {original_ts}")
print(f"Tuple: {tuple_from_ts}")
print(f"Back to timestamp: {ts_from_tuple}")
diff = ts_from_tuple - original_ts
print(f"Difference: {diff}")
print(f"Round-trip successful: {diff < 1 and diff > -1}")
print()

print("✓ All time library tests completed!")