import time
print("With time library:")
now = time.time()
tuple = time.localtime(now)
formatted = time.strftime("%Y-%m-%d %H:%M:%S", tuple)
print(formatted)

import datetime
print("With datetime library (Python-compatible):")
formatted2 = datetime.datetime.now()
print(formatted2)
