# Round-12 scatter: filesystem (makedirs/listdir/rmtree/glob incl. absolute
# patterns and recursive **), time.gmtime struct fields + strftime %j/%I/%y,
# StringIO read/readline/readlines, csv reader/writer, string module.
# CPython-generated.
try:
    import os, tempfile
    base = tempfile.mkdtemp(prefix='scat_')
    os.makedirs(os.path.join(base, 'a', 'b'))
    os.makedirs(os.path.join(base, 'c'))
    assert repr(sorted(os.listdir(base))) == "['a', 'c']"
except Exception as ex:
    raise ex
try:
    import os, tempfile
    base = tempfile.mkdtemp(prefix='scat_')
    os.makedirs(os.path.join(base, 'x'))
    assert repr(os.path.isdir(os.path.join(base, 'x'))) == 'True'
except Exception as ex:
    raise ex
try:
    import os, tempfile
    base = tempfile.mkdtemp(prefix='scat_')
    assert repr(os.path.isdir(base) and not os.path.isfile(base)) == 'True'
except Exception as ex:
    raise ex
try:
    import os, tempfile
    base = tempfile.mkdtemp(prefix='scat_')
    os.makedirs(os.path.join(base, 'a', 'b'))
    assert repr(os.path.exists(os.path.join(base, 'a', 'b'))) == 'True'
except Exception as ex:
    raise ex
try:
    import os, tempfile
    base = tempfile.mkdtemp(prefix='scat_')
    os.makedirs(os.path.join(base, 'a'))
    os.rmdir(os.path.join(base, 'a'))
    assert repr(os.listdir(base)) == '[]'
except Exception as ex:
    raise ex
try:
    import os, tempfile, shutil
    base = tempfile.mkdtemp(prefix='scat_')
    os.makedirs(os.path.join(base, 'gone'))
    assert repr(None if shutil.rmtree(os.path.join(base, 'gone')) else sorted(os.listdir(base))) == '[]'
except Exception as ex:
    raise ex
try:
    import os, tempfile, glob
    base = tempfile.mkdtemp(prefix='scat_')
    os.makedirs(os.path.join(base, 'd1'))
    os.makedirs(os.path.join(base, 'd2'))
    assert repr(sorted(x.split('/')[-1] for x in glob.glob(os.path.join(base, 'd*')))) == "['d1', 'd2']"
except Exception as ex:
    raise ex
try:
    import os, tempfile, glob
    base = tempfile.mkdtemp(prefix='scat_')
    os.makedirs(os.path.join(base, 'sub', 'deep'))
    assert repr(sorted(x.replace(base, '') for x in glob.glob(os.path.join(base, '**'), recursive=True))[:4]) == "['/', '/sub', '/sub/deep']"
except Exception as ex:
    raise ex
try:
    import os
    assert repr(os.path.normpath('a/./b/../c')) == "'a/c'"
except Exception as ex:
    raise ex
try:
    import os
    assert repr(os.path.commonprefix(['/a/b/x', '/a/b/y', '/a/b/z'])) == "'/a/b/'"
except Exception as ex:
    raise ex
try:
    import time
    g = time.gmtime(0)
    assert repr((g.tm_year, g.tm_mon, g.tm_mday, g.tm_hour, g.tm_min, g.tm_sec)) == '(1970, 1, 1, 0, 0, 0)'
except Exception as ex:
    raise ex
try:
    import time
    g = time.gmtime(0)
    assert repr(g.tm_wday) == '3'
except Exception as ex:
    raise ex
try:
    import time
    g = time.gmtime(0)
    assert repr(g.tm_yday) == '1'
except Exception as ex:
    raise ex
try:
    import time
    assert repr(time.strftime('%Y-%m-%d', time.gmtime(0))) == "'1970-01-01'"
except Exception as ex:
    raise ex
try:
    import time
    assert repr(time.strftime('%H:%M:%S', time.gmtime(0))) == "'00:00:00'"
except Exception as ex:
    raise ex
try:
    import time
    assert repr(time.strftime('%A', time.gmtime(0))) == "'Thursday'"
except Exception as ex:
    raise ex
try:
    import time
    assert repr(time.strftime('%j-%B', time.gmtime(0))) == "'001-January'"
except Exception as ex:
    raise ex
try:
    import io
    s = io.StringIO()
    s.write('hello')
    s.write(' world')
    assert repr(s.getvalue()) == "'hello world'"
except Exception as ex:
    raise ex
try:
    import io
    s = io.StringIO('a\nb\nc')
    assert repr(s.readline()) == "'a\\n'"
except Exception as ex:
    raise ex
try:
    import io
    s = io.StringIO('a\nb')
    assert repr(len(s.readlines())) == '2'
except Exception as ex:
    raise ex
try:
    import io
    s = io.StringIO('xyz')
    s.read(2)
    assert repr(s.read(1)) == "'z'"
except Exception as ex:
    raise ex
try:
    import io
    s = io.StringIO('hello')
    assert repr(len(s.read())) == '5'
except Exception as ex:
    raise ex
try:
    import csv, io
    out = io.StringIO()
    w = csv.writer(out)
    w.writerow(['a', 'b'])
    w.writerow(['1', 'x,y'])
    assert repr(out.getvalue()) == '\'a,b\\r\\n1,"x,y"\\r\\n\''
except Exception as ex:
    raise ex
try:
    import csv, io
    r = csv.reader(io.StringIO('a,b\n1,"x,y"\n'))
    assert repr(list(r)) == "[['a', 'b'], ['1', 'x,y']]"
except Exception as ex:
    raise ex
try:
    import string
    assert repr(len(string.ascii_lowercase)) == '26'
except Exception as ex:
    raise ex
try:
    import string
    assert repr(string.digits + string.ascii_uppercase[:3]) == "'0123456789ABC'"
except Exception as ex:
    raise ex
try:
    import string
    assert repr(string.ascii_letters[:5]) == "'abcde'"
except Exception as ex:
    raise ex
try:
    import string
    assert repr(string.hexdigits[:10]) == "'0123456789'"
except Exception as ex:
    raise ex
try:
    import uuid
    u = uuid.uuid4()
    assert repr(len(str(u)) == 36 and str(u)[8] == '-') == 'True'
except Exception as ex:
    raise ex
try:
    import platform
    assert repr(isinstance(platform.system(), str) and len(platform.system()) > 0) == 'True'
except Exception as ex:
    raise ex
assert repr(divmod(7, 2)[0] + divmod(7, 2)[1]) == '4'
try:
    import itertools
    assert repr(list(itertools.repeat(7, 3))) == '[7, 7, 7]'
except Exception as ex:
    raise ex
try:
    import itertools
    assert repr([len(t) for t in itertools.zip_longest('ab', 'xyz')]) == '[2, 2, 2]'
except Exception as ex:
    raise ex

True
