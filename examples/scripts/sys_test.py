# Test sys library (extended)
import sys

# Test platform constant
platform = sys.platform
platform == "darwin" or platform == "linux" or platform == "win32"

# Test version constant
len(sys.version) > 0

# Test maxsize constant
sys.maxsize > 0

# Test path_sep constant
len(sys.path_sep) == 1

# Test argv (should be a list)
type(sys.argv) == "LIST"
