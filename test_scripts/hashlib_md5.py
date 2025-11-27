import hashlib

text = "Hello, World!"
hash_md5 = hashlib.md5(text)
hash_md5 == "65a8e27d8879283831b664bd8b7f0ad4"