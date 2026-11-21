# Naive datetimes are local wall-clock values however they are built: parsed,
# combined, replaced or converted values equal the constructor's, and
# timestamp()/astimezone() read them as local time. Nothing here depends on
# the host's zone.
import datetime
import time

dt = datetime.datetime
utc = datetime.timezone.utc
a = dt(2024, 5, 6, 7, 8, 9)

# Every way of building the same naive value agrees with the constructor.
same = [
    dt.fromisoformat("2024-05-06T07:08:09"),
    dt.fromisoformat("2024-05-06 07:08:09"),
    dt.strptime("2024-05-06 07:08:09", "%Y-%m-%d %H:%M:%S"),
    dt.combine(datetime.date(2024, 5, 6), datetime.time(7, 8, 9)),
    dt.combine(datetime.date.fromisoformat("2024-05-06"), datetime.time(7, 8, 9)),
    dt(2024, 5, 6, 7, 8, 9, tzinfo=utc).replace(tzinfo=None),
    dt(2024, 1, 1).replace(month=5, day=6, hour=7, minute=8, second=9),
    dt.fromisoformat(a.isoformat()),
    dt.fromtimestamp(a.timestamp()),
    dt.fromordinal(a.toordinal()).replace(hour=7, minute=8, second=9),
]
for v in same:
    assert v == a, v
    assert not (v < a) and not (v > a)
    assert hash(v) == hash(a)
    assert v.timestamp() == a.timestamp()
    assert v.tzinfo is None
    assert v.isoformat() == "2024-05-06T07:08:09"
    assert time.mktime(v.timetuple()) == a.timestamp()
assert len(set(same)) == 1
assert sorted([dt.fromisoformat("2024-05-06T07:08:10"), a, dt.strptime("2024-05-06", "%Y-%m-%d")])[1] == a

# Dates.
d = datetime.date(2024, 5, 6)
assert datetime.date.fromisoformat("2024-05-06") == d
assert a.date() == d
assert datetime.date.fromordinal(d.toordinal()) == d
assert dt.fromisoformat("2024-05-06") == dt(2024, 5, 6)
assert dt.fromisoformat("2024-05-06").timestamp() == dt(2024, 5, 6).timestamp()
assert datetime.date.fromisoformat(datetime.date.today().isoformat()) == datetime.date.today()
assert datetime.date.today() == dt.now().date() or (dt.now() - dt.combine(datetime.date.today(), datetime.time())).total_seconds() < 5
assert (datetime.date(2024, 3, 11) - datetime.date.fromisoformat("2024-03-09")).days == 2

# timestamp() reads the wall clock as local time: the same instant as the
# aware value astimezone() gives, and fromtimestamp() round-trips.
for v in [a, dt(2024, 1, 15, 12), dt(2024, 7, 1, 12), dt(1999, 12, 31, 23, 59, 59), dt(2024, 5, 6, 7, 8, 9, 250000)]:
    aw = v.astimezone()
    assert aw.tzinfo is not None
    assert aw.timestamp() == v.timestamp()
    assert aw.replace(tzinfo=None) == v
    assert v.astimezone(utc).timestamp() == v.timestamp()
    assert dt.fromtimestamp(v.timestamp()) == v
    assert dt.fromtimestamp(v.timestamp(), utc) == v.astimezone(utc)
    assert dt.fromtimestamp(v.timestamp(), utc).replace(tzinfo=None) == v.astimezone(utc).replace(tzinfo=None)
    assert int(v.strftime("%s")) == int(v.timestamp())

# Aware values compare by instant.
assert dt.fromisoformat("2024-05-06T05:08:09+00:00") == dt(2024, 5, 6, 7, 8, 9, tzinfo=datetime.timezone(datetime.timedelta(hours=2)))
assert dt.fromisoformat("2024-05-06T07:08:09+02:00").timestamp() == dt(2024, 5, 6, 5, 8, 9, tzinfo=utc).timestamp()
assert dt.fromtimestamp(1714950489, utc) == dt(2024, 5, 5, 23, 8, 9, tzinfo=utc)
assert dt.fromtimestamp(1714950489, utc).timestamp() == 1714950489
assert dt.fromtimestamp(1714950489, utc).replace(tzinfo=None) == dt(2024, 5, 5, 23, 8, 9)
assert dt.fromtimestamp(0, utc).replace(tzinfo=None) == dt(1970, 1, 1)

# Naive and aware never compare equal, and ordering them is a TypeError.
assert (a == a.replace(tzinfo=utc)) is False
assert (a != a.replace(tzinfo=utc)) is True
try:
    a < a.replace(tzinfo=utc)
    assert False
except TypeError as e:
    assert "offset-naive and offset-aware" in str(e)
assert (datetime.time(7, 8, tzinfo=utc) == datetime.time(7, 8)) is False

# Naive arithmetic is on the wall clock, even across a daylight-saving change
# (2024-03-10 in New York): a day later is the same wall time.
b = dt(2024, 3, 9, 12, 0)
assert b + datetime.timedelta(days=1) == dt(2024, 3, 10, 12, 0)
assert dt(2024, 3, 10, 12, 0) - b == datetime.timedelta(days=1)
assert dt.fromisoformat("2024-03-10T12:00:00") - dt.fromisoformat("2024-03-09T12:00:00") == datetime.timedelta(days=1)
assert dt.fromisoformat("2024-05-06T07:08:09") + datetime.timedelta(days=1) == dt(2024, 5, 7, 7, 8, 9)
assert dt.fromisoformat("2024-05-07T07:08:09") - a == datetime.timedelta(days=1)
# A wall time keeps its fields whatever the local zone does at that hour.
g = dt(2024, 3, 10, 2, 30)
assert (g.hour, g.minute) == (2, 30)
assert dt.fromisoformat("2024-03-10T02:30:00") == g
f = dt(2024, 11, 3, 1, 30)
assert (f.hour, f.minute) == (1, 30)
assert f.astimezone(utc).timestamp() == f.timestamp()

# now(), utcnow() and today() describe the current instant.
n = dt.now()
assert n.tzinfo is None
assert abs(n.timestamp() - time.time()) < 5
assert abs((dt.now(utc).replace(tzinfo=None) - dt.fromtimestamp(time.time(), utc).replace(tzinfo=None)).total_seconds()) < 5
assert abs((dt.now(utc).astimezone().replace(tzinfo=None) - dt.now()).total_seconds()) < 5
assert dt.min == dt(1, 1, 1)
assert dt.max == dt(9999, 12, 31, 23, 59, 59, 999999)

print("OK")
