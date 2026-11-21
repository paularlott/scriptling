# Helper library for import_rebinding_test.py: loading it bumps LOADS, so a
# test can tell whether a second import re-executed it.
LOADS = []
LOADS.append(1)


def helper():
    return "helped"


def load_count():
    return len(LOADS)
