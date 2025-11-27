import hashlib

text = "Hello, World!"
hash_sha256 = hashlib.sha256(text)
hash_sha256 == "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"