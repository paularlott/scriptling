# Test string library constants
import string

# Test that constants are strings
len(string.ascii_letters) == 52
len(string.ascii_lowercase) == 26
len(string.ascii_uppercase) == 26
len(string.digits) == 10
len(string.hexdigits) == 22
len(string.octdigits) == 8
len(string.punctuation) > 0
len(string.whitespace) > 0
len(string.printable) > 0

# Test specific values
"a" in string.ascii_lowercase
"Z" in string.ascii_uppercase
"5" in string.digits
"f" in string.hexdigits
"!" in string.punctuation
" " in string.whitespace
