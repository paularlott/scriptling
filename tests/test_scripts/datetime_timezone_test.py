from datetime import datetime, timezone, timedelta

utc = timezone.utc
ist = timezone(timedelta(hours=5, minutes=30))
pst = timezone(timedelta(hours=-8), "PST")

assert str(utc) == "UTC" and repr(utc) == "datetime.timezone.utc"
assert str(ist) == "UTC+05:30"
assert repr(ist) == "datetime.timezone(datetime.timedelta(seconds=19800))"
assert str(pst) == "PST"
assert utc == timezone(timedelta(0))
assert ist.utcoffset(None).total_seconds() == 19800

d = datetime(2024, 1, 2, 3, 4, 5, tzinfo=utc)
assert d.isoformat() == "2024-01-02T03:04:05+00:00"
assert str(d) == "2024-01-02 03:04:05+00:00"
assert d.isoformat(" ") == "2024-01-02 03:04:05+00:00"
assert d.tzinfo is utc
assert str(d.utcoffset()) == "0:00:00"
assert datetime(2024, 1, 2, 3, 4, 5, 6000, tzinfo=ist).isoformat() == "2024-01-02T03:04:05.006000+05:30"
assert datetime(2024, 1, 1, tzinfo=ist).tzname() == "UTC+05:30"

# naive datetimes are unchanged
naive = datetime(2024, 1, 2, 3, 4, 5)
assert naive.isoformat() == "2024-01-02T03:04:05"
assert naive.tzinfo is None
assert naive.utcoffset() is None
assert datetime(2024, 3, 4, 5, 6, 7, 890000).isoformat() == "2024-03-04T05:06:07.890000"

# replace(tzinfo=...) sets or removes the zone, keeping the wall time
assert naive.replace(tzinfo=utc).isoformat() == "2024-01-02T03:04:05+00:00"
assert d.replace(tzinfo=None).isoformat() == "2024-01-02T03:04:05"

# astimezone converts the instant
assert d.astimezone(ist).isoformat() == "2024-01-02T08:34:05+05:30"
assert d.astimezone(pst).isoformat() == "2024-01-01T19:04:05-08:00"
assert (d + timedelta(hours=1)).isoformat() == "2024-01-02T04:04:05+00:00"

# now(), fromtimestamp() and fromisoformat() with zones
assert datetime.now(timezone.utc).tzinfo is timezone.utc
assert datetime.now(tz=utc).isoformat().endswith("+00:00")
assert datetime.fromtimestamp(0, tz=utc).isoformat() == "1970-01-01T00:00:00+00:00"
assert datetime.fromtimestamp(86400, utc).isoformat() == "1970-01-02T00:00:00+00:00"
assert datetime.fromisoformat("2024-05-06T07:08:09+02:00").isoformat() == "2024-05-06T07:08:09+02:00"
assert d.timestamp() == 1704164645.0

# aware arithmetic and comparison work across zones
a = datetime(2024, 1, 1, 12, tzinfo=utc)
b = datetime(2024, 1, 1, 10, tzinfo=ist)
assert (a - b).total_seconds() == 27000.0
assert datetime(2024, 1, 1, 12, tzinfo=utc) == datetime(2024, 1, 1, 17, 30, tzinfo=ist)

# timedelta takes positional arguments
assert timedelta(1, 30).total_seconds() == 86430.0

for bad in (lambda: timezone(timedelta(hours=25)), lambda: datetime(2024, 1, 1, tzinfo="x")):
    try:
        bad()
        assert False, "expected an error"
    except (ValueError, TypeError):
        pass

print("OK")
