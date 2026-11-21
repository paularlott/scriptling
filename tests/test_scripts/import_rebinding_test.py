# `import X` always binds X to the module and `from X import Y` always binds
# Y, in statement order, even when the name was bound to something else.
from datetime import datetime

assert datetime(2024, 1, 2).month == 1

import datetime

assert datetime.datetime(2024, 1, 2).year == 2024
assert datetime.date(2024, 1, 2).day == 2

from datetime import datetime

assert datetime(2024, 3, 4).month == 3


# A from-import inside a function leaves the global binding alone.
def day_of(y, m, d):
    from datetime import date
    return date(y, m, d).day


assert day_of(2024, 5, 6) == 6
assert datetime(2024, 7, 8).month == 7


# `import X` inside a function binds the module locally even when a global
# of that name is something else.
def year_via_module():
    import datetime
    return datetime.date(2021, 1, 1).year


assert year_via_module() == 2021
assert datetime(2024, 9, 1).month == 9


# A local name rebinds in order.
def local_rebind():
    from json import dumps as json
    first = json([1])
    import json
    return first + json.dumps([2])


assert local_rebind() == "[1][2]"

# A plain variable is rebound by the import too.
json = 5
import json

assert json.loads("[3]") == [3]

# Aliases and dotted names.
import datetime as dt
from datetime import timedelta

assert dt.timedelta(days=1) == timedelta(days=1)

import os.path

assert os.path.join("a", "b") == "a" + os.sep + "b"

# Script libraries: rebinding does not run the library again.
import lib_import_rebind
from lib_import_rebind import helper as lib_import_rebind

assert lib_import_rebind() == "helped"

import lib_import_rebind

assert lib_import_rebind.helper() == "helped"
assert lib_import_rebind.load_count() == 1

print("OK")
