import time
print("With time library:")
now = time.time()
tuple = time.localtime(now)
formatted = time.strftime("%Y-%m-%d %H:%M:%S", tuple)
print(formatted)

import datetime
print("With datetime library:")
formatted2 = datetime.now()
print(formatted2)
