import hashlib

text = "Hello, World!"
hash_sha1 = hashlib.sha1(text)
len(hash_sha1) == 40