import hmac
import hashlib

# RFC 4231 / well-known SHA-256 HMAC test vector
# key="key", msg="The quick brown fox jumps over the lazy dog"
known = "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"


def verify(body, signature, secret):
    # NOTE: scriptling has no `bytes` type and no param type annotations;
    # strings are used as byte buffers and secret.encode() (a list of byte
    # values) is accepted by hmac.new as the key.
    expected = "sha256=" + hmac.new(
        secret.encode(), body, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature)


# hmac.new with a string digestmod
assert hmac.new("key", "The quick brown fox jumps over the lazy dog", "sha256").hexdigest() == known

# passing hashlib.sha256 (a constructor reference) as digestmod works too
assert hmac.new("key", "msg", hashlib.sha256).hexdigest() == hmac.new("key", "msg", "sha256").hexdigest()

# omitted digestmod defaults to sha256
assert hmac.new("key", "msg").hexdigest() == hmac.new("key", "msg", "sha256").hexdigest()

# sha1 / md5 digestmods produce different, valid-length digests
assert len(hmac.new("key", "msg", "sha1").hexdigest()) == 40
assert len(hmac.new("key", "msg", "md5").hexdigest()) == 32

# the full verify() flow with the user's exact body code
body = "The quick brown fox jumps over the lazy dog"
secret = "key"
sig = "sha256=" + known
assert verify(body, sig, secret) is True
assert verify(body, "sha256=deadbeef", secret) is False

# update() accumulates
h = hmac.new("key", "", "sha256")
h.update("The quick brown fox ")
h.update("jumps over the lazy dog")
assert h.hexdigest() == known

# copy() is independent
h = hmac.new("key", "foo", "sha256")
c = h.copy()
h.update("bar")
assert c.hexdigest() == hmac.new("key", "foo", "sha256").hexdigest()
assert h.hexdigest() == hmac.new("key", "foobar", "sha256").hexdigest()

# attributes
h = hmac.new("key", "msg", "sha256")
assert h.name == "hmac-sha256"
assert h.digest_size == 32
assert h.block_size == 64

# hmac.digest one-shot returns the same as .digest() on an object
assert hmac.new("k", "m", "sha256").digest() == hmac.digest("k", "m", "sha256")

# compare_digest
assert hmac.compare_digest("abc", "abc") is True
assert hmac.compare_digest("abc", "abd") is False
assert hmac.compare_digest("abc", "abcd") is False

# secrets.compare_digest delegates to the same implementation
import secrets
assert secrets.compare_digest("xyz", "xyz") is True
assert secrets.compare_digest("xyz", "xya") is False
