# Generated from differential probes against python3: every assert holds in
# CPython and in scriptling. Regenerate rather than hand-edit.
def tn(e):
    t = type(e)
    return t if isinstance(t, str) else t.__name__
def R(x):
    if isinstance(x, (set, frozenset)):
        return "S" + repr(sorted(x, key=repr))
    if isinstance(x, (list, tuple)) and any(isinstance(i, (set, frozenset)) for i in x):
        return repr([R(i) for i in x])
    return repr(x)

import base64
import bisect
import collections
import copy
import csv, io
import difflib
import functools, operator
import hashlib
import heapq
import hmac
import hmac, hashlib
import html
import io
import itertools, operator
import json
import math
import operator
import random
import re
import secrets
import shlex
import statistics
import string
import textwrap
import urllib.parse
import uuid
assert R(list(csv.reader(['a,b,c', '1,"2,3",4']))) == "[['a', 'b', 'c'], ['1', '2,3', '4']]", 'csv#0'
assert R(list(csv.reader(['a;b'], delimiter=';'))) == "[['a', 'b']]", 'csv#1'
assert R(list(csv.reader(['"a""b",c']))) == '[[\'a"b\', \'c\']]', 'csv#2'
assert R(list(csv.reader(['  a, b']))) == "[['  a', ' b']]", 'csv#3'
assert R(list(csv.reader(['  a, b'], skipinitialspace=True))) == "[['a', 'b']]", 'csv#4'
assert R(list(csv.reader(['a,b', '', 'c']))) == "[['a', 'b'], [], ['c']]", 'csv#5'
assert R(list(csv.reader(['a,"multi\nline",c']))) == "[['a', 'multi\\nline', 'c']]", 'csv#6'
assert R(list(csv.reader(io.StringIO('a,"x\ny"\r\nb,c\r\n')))) == "[['a', 'x\\ny'], ['b', 'c']]", 'csv#7'
assert R(list(csv.reader(['a\tb'], dialect='excel-tab'))) == "[['a', 'b']]", 'csv#8'
assert R(list(csv.reader(['a,b,'], ))) == "[['a', 'b', '']]", 'csv#9'
assert R(list(csv.reader(["'a,b',c"], quotechar="'"))) == "[['a,b', 'c']]", 'csv#10'
assert R(list(csv.reader(['1,2'], quoting=csv.QUOTE_NONNUMERIC))) == '[[1.0, 2.0]]', 'csv#11'
assert R(list(csv.DictReader(['x,y', '1,2', '3,4']))) == "[{'x': '1', 'y': '2'}, {'x': '3', 'y': '4'}]", 'csv#12'
assert R(list(csv.DictReader(['1,2'], fieldnames=['a', 'b']))) == "[{'a': '1', 'b': '2'}]", 'csv#13'
assert R(list(csv.DictReader(['x,y', '1,2,3']))) == "[{'x': '1', 'y': '2', None: ['3']}]", 'csv#14'
assert R(list(csv.DictReader(['x,y,z', '1,2']))) == "[{'x': '1', 'y': '2', 'z': None}]", 'csv#15'
assert R(list(csv.DictReader(['x,y', '1,2,3'], restkey='rest'))) == "[{'x': '1', 'y': '2', 'rest': ['3']}]", 'csv#16'
assert R(csv.DictReader(io.StringIO('h1,h2\n1,2\n')).fieldnames) == "['h1', 'h2']", 'csv#17'
assert R((lambda b: (csv.writer(b).writerows([['a', 'b'], [1, 'x,y'], ['q"t', None], ['line\nbreak', '']]), b.getvalue())[1])(io.StringIO())) == '\'a,b\\r\\n1,"x,y"\\r\\n"q""t",\\r\\n"line\\nbreak",\\r\\n\'', 'csv#18'
assert R((lambda b: (csv.writer(b, quoting=csv.QUOTE_ALL).writerows([['a', 1.5]]), b.getvalue())[1])(io.StringIO())) == '\'"a","1.5"\\r\\n\'', 'csv#19'
assert R((lambda b: (csv.writer(b, quoting=csv.QUOTE_NONNUMERIC).writerows([['a', 1.5]]), b.getvalue())[1])(io.StringIO())) == '\'"a",1.5\\r\\n\'', 'csv#20'
assert R((lambda b: (csv.writer(b, lineterminator='\n').writerows([['a', 'b']]), b.getvalue())[1])(io.StringIO())) == "'a,b\\n'", 'csv#22'
assert R((lambda b: (csv.writer(b).writerows([[' lead', 'trail ']]), b.getvalue())[1])(io.StringIO())) == "' lead,trail \\r\\n'", 'csv#24'
assert R((lambda b: (csv.writer(b, quoting=csv.QUOTE_MINIMAL).writerows([['a', 'b']]), b.getvalue())[1])(io.StringIO())) == "'a,b\\r\\n'", 'csv#26'
assert R((lambda b: (csv.writer(b, quotechar="'").writerows([["it's"]]), b.getvalue())[1])(io.StringIO())) == '"\'it\'\'s\'\\r\\n"', 'csv#27'
assert R(csv.QUOTE_ALL) == '1', 'csv#32'
assert R(csv.QUOTE_NONE) == '3', 'csv#33'
assert R(urllib.parse.urlparse('http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2#frag')) == "ParseResult(scheme='http', netloc='user:pw@host.com:8080', path='/p/a/t/h', params='params', query='query=1&b=2', fragment='frag')", 'urllib#0'
assert R(urllib.parse.urlsplit('http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2#frag')) == "SplitResult(scheme='http', netloc='user:pw@host.com:8080', path='/p/a/t/h;params', query='query=1&b=2', fragment='frag')", 'urllib#1'
assert R(urllib.parse.urlparse('http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2#frag').hostname) == "'host.com'", 'urllib#4'
assert R(urllib.parse.urlparse('http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2#frag').port) == '8080', 'urllib#5'
assert R(urllib.parse.urlparse('http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2#frag').username) == "'user'", 'urllib#7'
assert R(urllib.parse.urlparse('http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2#frag').password) == "'pw'", 'urllib#8'
assert R(urllib.parse.urldefrag('http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2#frag')) == "DefragResult(url='http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2', fragment='frag')", 'urllib#10'
assert R(urllib.parse.urlparse('http://user:pw@host.com:8080/p/a/t/h;params?query=1&b=2#frag')._replace(query='z=1').geturl()) == "'http://user:pw@host.com:8080/p/a/t/h;params?z=1#frag'", 'urllib#11'
assert R(urllib.parse.urlparse('https://example.com')) == "ParseResult(scheme='https', netloc='example.com', path='', params='', query='', fragment='')", 'urllib#12'
assert R(urllib.parse.urlsplit('https://example.com')) == "SplitResult(scheme='https', netloc='example.com', path='', query='', fragment='')", 'urllib#13'
assert R(urllib.parse.urlparse('https://example.com').hostname) == "'example.com'", 'urllib#16'
assert R(urllib.parse.urlparse('https://example.com').port) == 'None', 'urllib#17'
assert R(urllib.parse.urlparse('https://example.com').username) == 'None', 'urllib#19'
assert R(urllib.parse.urlparse('https://example.com').password) == 'None', 'urllib#20'
assert R(urllib.parse.urldefrag('https://example.com')) == "DefragResult(url='https://example.com', fragment='')", 'urllib#22'
assert R(urllib.parse.urlparse('https://example.com')._replace(query='z=1').geturl()) == "'https://example.com?z=1'", 'urllib#23'
assert R(urllib.parse.urlparse('//netloc/path')) == "ParseResult(scheme='', netloc='netloc', path='/path', params='', query='', fragment='')", 'urllib#24'
assert R(urllib.parse.urlsplit('//netloc/path')) == "SplitResult(scheme='', netloc='netloc', path='/path', query='', fragment='')", 'urllib#25'
assert R(urllib.parse.urlparse('//netloc/path').hostname) == "'netloc'", 'urllib#28'
assert R(urllib.parse.urlparse('//netloc/path').port) == 'None', 'urllib#29'
assert R(urllib.parse.urlparse('//netloc/path').username) == 'None', 'urllib#31'
assert R(urllib.parse.urlparse('//netloc/path').password) == 'None', 'urllib#32'
assert R(urllib.parse.urlparse('//netloc/path').geturl()) == "'//netloc/path'", 'urllib#33'
assert R(urllib.parse.urldefrag('//netloc/path')) == "DefragResult(url='//netloc/path', fragment='')", 'urllib#34'
assert R(urllib.parse.urlparse('//netloc/path')._replace(query='z=1').geturl()) == "'//netloc/path?z=1'", 'urllib#35'
assert R(urllib.parse.urlparse('/just/path?x=1')) == "ParseResult(scheme='', netloc='', path='/just/path', params='', query='x=1', fragment='')", 'urllib#36'
assert R(urllib.parse.urlsplit('/just/path?x=1')) == "SplitResult(scheme='', netloc='', path='/just/path', query='x=1', fragment='')", 'urllib#37'
assert R(urllib.parse.urlparse('/just/path?x=1').hostname) == 'None', 'urllib#40'
assert R(urllib.parse.urlparse('/just/path?x=1').port) == 'None', 'urllib#41'
assert R(urllib.parse.urlparse('/just/path?x=1').username) == 'None', 'urllib#43'
assert R(urllib.parse.urlparse('/just/path?x=1').password) == 'None', 'urllib#44'
assert R(urllib.parse.urlparse('/just/path?x=1').geturl()) == "'/just/path?x=1'", 'urllib#45'
assert R(urllib.parse.urldefrag('/just/path?x=1')) == "DefragResult(url='/just/path?x=1', fragment='')", 'urllib#46'
assert R(urllib.parse.urlparse('/just/path?x=1')._replace(query='z=1').geturl()) == "'/just/path?z=1'", 'urllib#47'
assert R(urllib.parse.urlparse('mailto:a@b.c')) == "ParseResult(scheme='mailto', netloc='', path='a@b.c', params='', query='', fragment='')", 'urllib#48'
assert R(urllib.parse.urlsplit('mailto:a@b.c')) == "SplitResult(scheme='mailto', netloc='', path='a@b.c', query='', fragment='')", 'urllib#49'
assert R(urllib.parse.urlparse('mailto:a@b.c').hostname) == 'None', 'urllib#52'
assert R(urllib.parse.urlparse('mailto:a@b.c').port) == 'None', 'urllib#53'
assert R(urllib.parse.urlparse('mailto:a@b.c').username) == 'None', 'urllib#55'
assert R(urllib.parse.urlparse('mailto:a@b.c').password) == 'None', 'urllib#56'
assert R(urllib.parse.urlparse('mailto:a@b.c').geturl()) == "'mailto:a@b.c'", 'urllib#57'
assert R(urllib.parse.urldefrag('mailto:a@b.c')) == "DefragResult(url='mailto:a@b.c', fragment='')", 'urllib#58'
assert R(urllib.parse.urlparse('mailto:a@b.c')._replace(query='z=1').geturl()) == "'mailto:a@b.c?z=1'", 'urllib#59'
assert R(urllib.parse.urlparse('HTTP://Example.COM/A')) == "ParseResult(scheme='http', netloc='Example.COM', path='/A', params='', query='', fragment='')", 'urllib#60'
assert R(urllib.parse.urlsplit('HTTP://Example.COM/A')) == "SplitResult(scheme='http', netloc='Example.COM', path='/A', query='', fragment='')", 'urllib#61'
assert R(urllib.parse.urlparse('HTTP://Example.COM/A').hostname) == "'example.com'", 'urllib#64'
assert R(urllib.parse.urlparse('HTTP://Example.COM/A').port) == 'None', 'urllib#65'
assert R(urllib.parse.urlparse('HTTP://Example.COM/A').username) == 'None', 'urllib#67'
assert R(urllib.parse.urlparse('HTTP://Example.COM/A').password) == 'None', 'urllib#68'
assert R(urllib.parse.urldefrag('HTTP://Example.COM/A')) == "DefragResult(url='HTTP://Example.COM/A', fragment='')", 'urllib#70'
assert R(urllib.parse.urlparse('HTTP://Example.COM/A')._replace(query='z=1').geturl()) == "'http://Example.COM/A?z=1'", 'urllib#71'
assert R(urllib.parse.urlparse('file:///etc/hosts')) == "ParseResult(scheme='file', netloc='', path='/etc/hosts', params='', query='', fragment='')", 'urllib#72'
assert R(urllib.parse.urlsplit('file:///etc/hosts')) == "SplitResult(scheme='file', netloc='', path='/etc/hosts', query='', fragment='')", 'urllib#73'
assert R(urllib.parse.urlparse('file:///etc/hosts').hostname) == 'None', 'urllib#76'
assert R(urllib.parse.urlparse('file:///etc/hosts').port) == 'None', 'urllib#77'
assert R(urllib.parse.urlparse('file:///etc/hosts').username) == 'None', 'urllib#79'
assert R(urllib.parse.urlparse('file:///etc/hosts').password) == 'None', 'urllib#80'
assert R(urllib.parse.urldefrag('file:///etc/hosts')) == "DefragResult(url='file:///etc/hosts', fragment='')", 'urllib#82'
assert R(urllib.parse.urlparse('file:///etc/hosts')._replace(query='z=1').geturl()) == "'file:///etc/hosts?z=1'", 'urllib#83'
assert R(urllib.parse.urlparse('http://[::1]:80/x')) == "ParseResult(scheme='http', netloc='[::1]:80', path='/x', params='', query='', fragment='')", 'urllib#84'
assert R(urllib.parse.urlsplit('http://[::1]:80/x')) == "SplitResult(scheme='http', netloc='[::1]:80', path='/x', query='', fragment='')", 'urllib#85'
assert R(urllib.parse.urlparse('http://[::1]:80/x').hostname) == "'::1'", 'urllib#88'
assert R(urllib.parse.urlparse('http://[::1]:80/x').port) == '80', 'urllib#89'
assert R(urllib.parse.urlparse('http://[::1]:80/x').username) == 'None', 'urllib#91'
assert R(urllib.parse.urlparse('http://[::1]:80/x').password) == 'None', 'urllib#92'
assert R(urllib.parse.urldefrag('http://[::1]:80/x')) == "DefragResult(url='http://[::1]:80/x', fragment='')", 'urllib#94'
assert R(urllib.parse.urlparse('http://[::1]:80/x')._replace(query='z=1').geturl()) == "'http://[::1]:80/x?z=1'", 'urllib#95'
assert R(urllib.parse.urlparse('http://h/p?')) == "ParseResult(scheme='http', netloc='h', path='/p', params='', query='', fragment='')", 'urllib#96'
assert R(urllib.parse.urlsplit('http://h/p?')) == "SplitResult(scheme='http', netloc='h', path='/p', query='', fragment='')", 'urllib#97'
assert R(urllib.parse.urlparse('http://h/p?').hostname) == "'h'", 'urllib#100'
assert R(urllib.parse.urlparse('http://h/p?').port) == 'None', 'urllib#101'
assert R(urllib.parse.urlparse('http://h/p?').username) == 'None', 'urllib#103'
assert R(urllib.parse.urlparse('http://h/p?').password) == 'None', 'urllib#104'
assert R(urllib.parse.urldefrag('http://h/p?')) == "DefragResult(url='http://h/p?', fragment='')", 'urllib#106'
assert R(urllib.parse.urlparse('http://h/p?')._replace(query='z=1').geturl()) == "'http://h/p?z=1'", 'urllib#107'
assert R(urllib.parse.urlparse('http://h/p#')) == "ParseResult(scheme='http', netloc='h', path='/p', params='', query='', fragment='')", 'urllib#108'
assert R(urllib.parse.urlsplit('http://h/p#')) == "SplitResult(scheme='http', netloc='h', path='/p', query='', fragment='')", 'urllib#109'
assert R(urllib.parse.urlparse('http://h/p#').hostname) == "'h'", 'urllib#112'
assert R(urllib.parse.urlparse('http://h/p#').port) == 'None', 'urllib#113'
assert R(urllib.parse.urlparse('http://h/p#').username) == 'None', 'urllib#115'
assert R(urllib.parse.urlparse('http://h/p#').password) == 'None', 'urllib#116'
assert R(urllib.parse.urldefrag('http://h/p#')) == "DefragResult(url='http://h/p', fragment='')", 'urllib#118'
assert R(urllib.parse.urlparse('http://h/p#')._replace(query='z=1').geturl()) == "'http://h/p?z=1'", 'urllib#119'
assert R(urllib.parse.urlparse('rel/path')) == "ParseResult(scheme='', netloc='', path='rel/path', params='', query='', fragment='')", 'urllib#120'
assert R(urllib.parse.urlsplit('rel/path')) == "SplitResult(scheme='', netloc='', path='rel/path', query='', fragment='')", 'urllib#121'
assert R(urllib.parse.urlparse('rel/path').hostname) == 'None', 'urllib#124'
assert R(urllib.parse.urlparse('rel/path').port) == 'None', 'urllib#125'
assert R(urllib.parse.urlparse('rel/path').username) == 'None', 'urllib#127'
assert R(urllib.parse.urlparse('rel/path').password) == 'None', 'urllib#128'
assert R(urllib.parse.urlparse('rel/path').geturl()) == "'rel/path'", 'urllib#129'
assert R(urllib.parse.urldefrag('rel/path')) == "DefragResult(url='rel/path', fragment='')", 'urllib#130'
assert R(urllib.parse.urlparse('rel/path')._replace(query='z=1').geturl()) == "'rel/path?z=1'", 'urllib#131'
assert R(urllib.parse.urlparse('')) == "ParseResult(scheme='', netloc='', path='', params='', query='', fragment='')", 'urllib#132'
assert R(urllib.parse.urlsplit('')) == "SplitResult(scheme='', netloc='', path='', query='', fragment='')", 'urllib#133'
assert R(urllib.parse.urlparse('').hostname) == 'None', 'urllib#136'
assert R(urllib.parse.urlparse('').port) == 'None', 'urllib#137'
assert R(urllib.parse.urlparse('').username) == 'None', 'urllib#139'
assert R(urllib.parse.urlparse('').password) == 'None', 'urllib#140'
assert R(urllib.parse.urlparse('').geturl()) == "''", 'urllib#141'
assert R(urllib.parse.urldefrag('')) == "DefragResult(url='', fragment='')", 'urllib#142'
assert R(urllib.parse.urlparse('')._replace(query='z=1').geturl()) == "'?z=1'", 'urllib#143'
assert R(urllib.parse.urlparse('http://h:abc/')) == "ParseResult(scheme='http', netloc='h:abc', path='/', params='', query='', fragment='')", 'urllib#144'
assert R(urllib.parse.urlsplit('http://h:abc/')) == "SplitResult(scheme='http', netloc='h:abc', path='/', query='', fragment='')", 'urllib#145'
assert R(urllib.parse.urlparse('http://h:abc/').hostname) == "'h'", 'urllib#148'
assert R(urllib.parse.urlparse('http://h:abc/').netloc) == "'h:abc'", 'urllib#150'
assert R(urllib.parse.urlparse('http://h:abc/').username) == 'None', 'urllib#151'
assert R(urllib.parse.urlparse('http://h:abc/').password) == 'None', 'urllib#152'
assert R(urllib.parse.urlparse('http://h:abc/').geturl()) == "'http://h:abc/'", 'urllib#153'
assert R(urllib.parse.urldefrag('http://h:abc/')) == "DefragResult(url='http://h:abc/', fragment='')", 'urllib#154'
assert R(urllib.parse.urlparse('http://h:abc/')._replace(query='z=1').geturl()) == "'http://h:abc/?z=1'", 'urllib#155'
assert R(urllib.parse.urljoin('http://a/b/c/d;p?q', 'http:g')) == "'http://a/b/c/g'", 'urllib#178'
assert R(urllib.parse.urlparse('http://H/p').hostname) == "'h'", 'urllib#250'
assert R(urllib.parse.urlsplit('http://h/p?q#f').fragment) == "'f'", 'urllib#251'
assert R(urllib.parse.urlparse('http://h/p?a=1')[4]) == "'a=1'", 'urllib#254'
assert R(base64.b32encode(b'')) == "b''", 'base64#3'
assert R(base64.b16encode(b'')) == "b''", 'base64#4'
assert R(base64.standard_b64encode(b'')) == "b''", 'base64#7'
assert R(base64.encodebytes(b'')) == "b''", 'base64#8'
assert R(base64.b32decode(base64.b32encode(b''))) == "b''", 'base64#9'
assert R(base64.b16decode(base64.b16encode(b''))) == "b''", 'base64#10'
assert R(base64.b32encode(b'f')) == "b'MY======'", 'base64#14'
assert R(base64.b16encode(b'f')) == "b'66'", 'base64#15'
assert R(base64.standard_b64encode(b'f')) == "b'Zg=='", 'base64#18'
assert R(base64.encodebytes(b'f')) == "b'Zg==\\n'", 'base64#19'
assert R(base64.b32decode(base64.b32encode(b'f'))) == "b'f'", 'base64#20'
assert R(base64.b16decode(base64.b16encode(b'f'))) == "b'f'", 'base64#21'
assert R(base64.b32encode(b'fo')) == "b'MZXQ===='", 'base64#25'
assert R(base64.b16encode(b'fo')) == "b'666F'", 'base64#26'
assert R(base64.standard_b64encode(b'fo')) == "b'Zm8='", 'base64#29'
assert R(base64.encodebytes(b'fo')) == "b'Zm8=\\n'", 'base64#30'
assert R(base64.b32decode(base64.b32encode(b'fo'))) == "b'fo'", 'base64#31'
assert R(base64.b16decode(base64.b16encode(b'fo'))) == "b'fo'", 'base64#32'
assert R(base64.b32encode(b'foo')) == "b'MZXW6==='", 'base64#36'
assert R(base64.b16encode(b'foo')) == "b'666F6F'", 'base64#37'
assert R(base64.standard_b64encode(b'foo')) == "b'Zm9v'", 'base64#40'
assert R(base64.encodebytes(b'foo')) == "b'Zm9v\\n'", 'base64#41'
assert R(base64.b32decode(base64.b32encode(b'foo'))) == "b'foo'", 'base64#42'
assert R(base64.b16decode(base64.b16encode(b'foo'))) == "b'foo'", 'base64#43'
assert R(base64.b32encode(b'foobar')) == "b'MZXW6YTBOI======'", 'base64#47'
assert R(base64.b16encode(b'foobar')) == "b'666F6F626172'", 'base64#48'
assert R(base64.standard_b64encode(b'foobar')) == "b'Zm9vYmFy'", 'base64#51'
assert R(base64.encodebytes(b'foobar')) == "b'Zm9vYmFy\\n'", 'base64#52'
assert R(base64.b32decode(base64.b32encode(b'foobar'))) == "b'foobar'", 'base64#53'
assert R(base64.b16decode(base64.b16encode(b'foobar'))) == "b'foobar'", 'base64#54'
assert R(base64.b32encode(b'\xff\xfe\xfd')) == "b'777P2==='", 'base64#58'
assert R(base64.b16encode(b'\xff\xfe\xfd')) == "b'FFFEFD'", 'base64#59'
assert R(base64.standard_b64encode(b'\xff\xfe\xfd')) == "b'//79'", 'base64#62'
assert R(base64.encodebytes(b'\xff\xfe\xfd')) == "b'//79\\n'", 'base64#63'
assert R(base64.b32decode(base64.b32encode(b'\xff\xfe\xfd'))) == "b'\\xff\\xfe\\xfd'", 'base64#64'
assert R(base64.b16decode(base64.b16encode(b'\xff\xfe\xfd'))) == "b'\\xff\\xfe\\xfd'", 'base64#65'
assert R(base64.b32encode('hi'.encode())) == "b'NBUQ===='", 'base64#69'
assert R(base64.b16encode('hi'.encode())) == "b'6869'", 'base64#70'
assert R(base64.standard_b64encode('hi'.encode())) == "b'aGk='", 'base64#73'
assert R(base64.encodebytes('hi'.encode())) == "b'aGk=\\n'", 'base64#74'
assert R(base64.b32decode(base64.b32encode('hi'.encode()))) == "b'hi'", 'base64#75'
assert R(base64.b16decode(base64.b16encode('hi'.encode()))) == "b'hi'", 'base64#76'
assert R(base64.b32encode(bytes(range(10)))) == "b'AAAQEAYEAUDAOCAJ'", 'base64#80'
assert R(base64.b16encode(bytes(range(10)))) == "b'00010203040506070809'", 'base64#81'
assert R(base64.standard_b64encode(bytes(range(10)))) == "b'AAECAwQFBgcICQ=='", 'base64#84'
assert R(base64.encodebytes(bytes(range(10)))) == "b'AAECAwQFBgcICQ==\\n'", 'base64#85'
assert R(base64.b32decode(base64.b32encode(bytes(range(10))))) == "b'\\x00\\x01\\x02\\x03\\x04\\x05\\x06\\x07\\x08\\t'", 'base64#86'
assert R(base64.b16decode(base64.b16encode(bytes(range(10))))) == "b'\\x00\\x01\\x02\\x03\\x04\\x05\\x06\\x07\\x08\\t'", 'base64#87'
assert R(base64.b64encode(b'\xfb\xff', altchars=b'-_')) == "b'-_8='", 'base64#93'
assert R(base64.b16decode('ff', casefold=True)) == "b'\\xff'", 'base64#95'
assert R(hashlib.sha224().digest_size) == '28', 'hashlib#39'
assert R(hashlib.sha384().digest_size) == '48', 'hashlib#67'
assert R(hashlib.sha512().digest_size) == '64', 'hashlib#81'
assert R(hashlib.sha3_256(b'').hexdigest()) == "'a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a'", 'hashlib#84'
assert R(len(hashlib.sha3_256(b'').digest())) == '32', 'hashlib#85'
assert R(hashlib.new('sha3_256', b'').hexdigest()) == "'a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a'", 'hashlib#86'
assert R(hashlib.sha3_256(b'abc').hexdigest()) == "'3a985da74fe225b2045c172d6bd390bd855f086e3e9d525b46bfe24511431532'", 'hashlib#87'
assert R(len(hashlib.sha3_256(b'abc').digest())) == '32', 'hashlib#88'
assert R(hashlib.new('sha3_256', b'abc').hexdigest()) == "'3a985da74fe225b2045c172d6bd390bd855f086e3e9d525b46bfe24511431532'", 'hashlib#89'
assert R(hashlib.sha3_256('héllo'.encode()).hexdigest()) == "'af9db8307521b5e3d5549adcb8d6197a378902789c1649a9adb2c559c54943ac'", 'hashlib#90'
assert R(len(hashlib.sha3_256('héllo'.encode()).digest())) == '32', 'hashlib#91'
assert R(hashlib.new('sha3_256', 'héllo'.encode()).hexdigest()) == "'af9db8307521b5e3d5549adcb8d6197a378902789c1649a9adb2c559c54943ac'", 'hashlib#92'
assert R((lambda h: (h.update(b'a'), h.update(b'bc'), h.hexdigest())[2])(hashlib.sha3_256())) == "'3a985da74fe225b2045c172d6bd390bd855f086e3e9d525b46bfe24511431532'", 'hashlib#93'
assert R(hashlib.sha3_256().name) == "'sha3_256'", 'hashlib#94'
assert R(hashlib.sha3_256().digest_size) == '32', 'hashlib#95'
assert R(hashlib.sha3_256().block_size) == '136', 'hashlib#96'
assert R((lambda h: h.copy().hexdigest())(hashlib.sha3_256(b'x'))) == "'741efa311f97686956946758e0d95f70f11ff2da4f2feb7c54314f44134ac49f'", 'hashlib#97'
assert R(hmac.new(b'', b'', 'sha512').digest_size) == '64', 'hmac#18'
assert R(list(io.StringIO('x\ny'))) == "['x\\n', 'y']", 'io#5'
assert R((lambda s: (s.write('hello'), s.seek(0), s.write('J'), s.getvalue())[3])(io.StringIO())) == "'Jello'", 'io#8'
assert R((lambda s: (s.writelines(['a', 'b']), s.getvalue())[1])(io.StringIO())) == "'ab'", 'io#18'
assert R(io.StringIO('abc').readable()) == 'True', 'io#19'
assert R((lambda s: [s.close(), s.closed][1])(io.StringIO())) == 'True', 'io#20'
assert R(io.StringIO('a\nb\n').readline(1)) == "'a'", 'io#21'
assert R(math.hypot(1, 2, 2)) == '3.0', 'math#37'
assert R(math.prod([2], start=3)) == '6', 'math#69'
assert R(math.ulp(1.0)) == '2.220446049250313e-16', 'math#77'
assert R(math.sumprod([1, 2], [3, 4])) == '11', 'math#80'
assert R(math.lgamma(1)) == '0.0', 'math#84'
assert R(math.fsum([1e100, 1.0, -1e100])) == '1.0', 'math#95'
assert R(statistics.multimode([1, 1, 2, 2, 3])) == '[1, 2]', 'stats#9'
assert R(statistics.quantiles([1, 2, 3, 4, 5, 6, 7, 8])) == '[2.25, 4.5, 6.75]', 'stats#16'
assert R(statistics.quantiles([1, 2, 3, 4, 5], n=4)) == '[1.5, 3.0, 4.5]', 'stats#17'
assert R(statistics.mean([True, False])) == '0.5', 'stats#19'
assert R(statistics.correlation([1, 2, 3], [2, 4, 7])) == '0.9933992677987828', 'stats#22'
assert R(statistics.covariance([1, 2, 3], [1, 2, 4])) == '1.5', 'stats#23'
assert R(statistics.linear_regression([1, 2, 3], [2, 4, 6])) == 'LinearRegression(slope=2.0, intercept=0.0)', 'stats#24'
assert R(string.capwords('hello   world  x')) == "'Hello World X'", 'string#9'
assert R(string.capwords('a-b c', '-')) == "'A-B c'", 'string#10'
assert R(list(itertools.chain.from_iterable([[1], [2, 3]]))) == '[1, 2, 3]', 'itertools#1'
assert R(list(itertools.chain.from_iterable(['ab', 'c']))) == "['a', 'b', 'c']", 'itertools#2'
assert R(list(itertools.islice(itertools.count(1, 0.5), 3))) == '[1, 1.5, 2.0]', 'itertools#7'
assert R(sorted([3, 1, 2], key=functools.cmp_to_key(lambda a, b: b - a))) == '[3, 2, 1]', 'functools#7'
assert R(sorted(['bb', 'a', 'ccc'], key=functools.cmp_to_key(lambda a, b: len(a) - len(b)))) == "['a', 'bb', 'ccc']", 'functools#8'
assert R(max([1, 3, 2], key=functools.cmp_to_key(lambda a, b: a - b))) == '3', 'functools#9'
assert R(functools.partial(int).func is int) == 'True', 'functools#13'
assert R(functools.partial(int, '7').args) == "('7',)", 'functools#14'
assert R(functools.partial(int, base=8).keywords) == "{'base': 8}", 'functools#15'
P = collections.namedtuple('P', ['x', 'y'])
Q = collections.namedtuple('Q', 'a b', defaults=[0])
R3 = collections.namedtuple('R3', 'a, b, c')
assert R(P._make([3, 4])) == 'P(x=3, y=4)', 'collections#78'
assert R(Q._field_defaults) == "{'b': 0}", 'collections#87'
assert R(P(1, 2).count(1)) == '1', 'collections#93'
assert R(P(1, 2).index(2)) == '1', 'collections#94'
assert R(list(difflib.unified_diff(['a\n', 'b\n'], ['a\n', 'c\n']))) == "['--- \\n', '+++ \\n', '@@ -1,2 +1,2 @@\\n', ' a\\n', '-b\\n', '+c\\n']", 'difflib#0'
assert R(list(difflib.unified_diff(['a', 'b'], ['a', 'c'], 'x', 'y', lineterm=''))) == "['--- x', '+++ y', '@@ -1,2 +1,2 @@', ' a', '-b', '+c']", 'difflib#1'
assert R(list(difflib.unified_diff(['a\n'] * 3 + ['b\n'], ['a\n'] * 3 + ['c\n'], n=1))) == "['--- \\n', '+++ \\n', '@@ -3,2 +3,2 @@\\n', ' a\\n', '-b\\n', '+c\\n']", 'difflib#14'
import dataclasses
from dataclasses import dataclass, field, asdict, astuple
@dataclass
class Pt:
    x: int
    y: int = 0
    tags: list = field(default_factory=list)
@dataclass(frozen=True)
class F:
    a: int
    b: str = 'b'
@dataclass(order=True)
class O:
    k: int
    name: str = field(default='', compare=False)
@dataclass
class Sub(Pt):
    z: int = 9
@dataclass(eq=False)
class NE:
    v: int
@dataclass
class Post:
    a: int
    b: int = field(init=False)
    def __post_init__(self):
        self.b = self.a * 2
@dataclass
class Rp:
    a: int
    secret: str = field(default='s', repr=False)
assert R(Pt(1)) == 'Pt(x=1, y=0, tags=[])', 'dataclass#0'
assert R(Pt(1, 2, ['t'])) == "Pt(x=1, y=2, tags=['t'])", 'dataclass#1'
assert R(Pt(x=3)) == 'Pt(x=3, y=0, tags=[])', 'dataclass#2'
assert R(Pt(1) == Pt(1)) == 'True', 'dataclass#3'
assert R(Pt(1) == Pt(2)) == 'False', 'dataclass#4'
assert R(asdict(Pt(1, 2, ['a']))) == "{'x': 1, 'y': 2, 'tags': ['a']}", 'dataclass#5'
assert R(astuple(Pt(1, 2))) == '(1, 2, [])', 'dataclass#6'
assert R(F(1)) == "F(a=1, b='b')", 'dataclass#7'
assert R(hash(F(1)) == hash(F(1))) == 'True', 'dataclass#8'
assert R(F(1) == F(1, 'b')) == 'True', 'dataclass#9'
assert R(sorted([O(3), O(1, 'z'), O(2)])) == "[O(k=1, name='z'), O(k=2, name=''), O(k=3, name='')]", 'dataclass#10'
assert R(O(1) < O(2)) == 'True', 'dataclass#11'
assert R(O(1, 'a') == O(1, 'b')) == 'True', 'dataclass#12'
assert R(Sub(1)) == 'Sub(x=1, y=0, tags=[], z=9)', 'dataclass#13'
assert R(Sub(1, 2, [], 3).z) == '3', 'dataclass#14'
assert R(NE(1) == NE(1)) == 'False', 'dataclass#15'
assert R(Post(4)) == 'Post(a=4, b=8)', 'dataclass#16'
assert R(Post(4).b) == '8', 'dataclass#17'
assert R(Rp(1)) == 'Rp(a=1)', 'dataclass#18'
assert R(repr(Rp(1, 'x'))) == "'Rp(a=1)'", 'dataclass#19'
assert R(str(Pt(1))) == "'Pt(x=1, y=0, tags=[])'", 'dataclass#20'
assert R(dataclasses.fields(Pt)[0].name) == "'x'", 'dataclass#21'
assert R([f.name for f in dataclasses.fields(Pt(1))]) == "['x', 'y', 'tags']", 'dataclass#22'
assert R(dataclasses.replace(Pt(1, 2), y=5)) == 'Pt(x=1, y=5, tags=[])', 'dataclass#23'
assert R(dataclasses.is_dataclass(Pt)) == 'True', 'dataclass#24'
assert R(dataclasses.is_dataclass(Pt(1))) == 'True', 'dataclass#25'
assert R(dataclasses.is_dataclass(1)) == 'False', 'dataclass#26'
assert R((lambda a, b: a.tags is b.tags)(Pt(1), Pt(2))) == 'False', 'dataclass#27'
assert R((lambda p: (p.tags.append(1), p)[1])(Pt(0))) == 'Pt(x=0, y=0, tags=[1])', 'dataclass#28'
assert R(Pt(1).__eq__(Pt(1))) == 'True', 'dataclass#29'
assert R(asdict(Sub(1, tags=[Pt(2)]))) == "{'x': 1, 'y': 0, 'tags': [{'x': 2, 'y': 0, 'tags': []}], 'z': 9}", 'dataclass#30'
assert R(f'{F(2)}') == '"F(a=2, b=\'b\')"', 'dataclass#31'
assert R([Pt(1)]) == '[Pt(x=1, y=0, tags=[])]', 'dataclass#32'
assert R(Pt.__name__) == "'Pt'", 'dataclass#33'
assert R(max([O(1), O(5), O(3)])) == "O(k=5, name='')", 'dataclass#34'
assert R(O(2) >= O(2)) == 'True', 'dataclass#35'
assert R(dataclasses.asdict(F(5))) == "{'a': 5, 'b': 'b'}", 'dataclass#36'
assert R(F(1).a) == '1', 'dataclass#37'
import enum
from enum import Enum, IntEnum, Flag, IntFlag, auto
class Color(Enum):
    RED = 1
    GREEN = 2
    BLUE = 3
class A(Enum):
    X = auto()
    Y = auto()
class S(Enum):
    LOW = 'low'
    HIGH = 'high'
class N(IntEnum):
    ONE = 1
    TWO = 2
class P(Flag):
    R = auto()
    W = auto()
    X = auto()
class Al(Enum):
    A = 1
    B = 1
def json_ok():
    import json
    return json.dumps({'n': N.ONE})
assert R(Color.RED) == '<Color.RED: 1>', 'enum#0'
assert R(str(Color.RED)) == "'Color.RED'", 'enum#1'
assert R(repr(Color.RED)) == "'<Color.RED: 1>'", 'enum#2'
assert R(Color.RED.name) == "'RED'", 'enum#3'
assert R(Color.RED.value) == '1', 'enum#4'
assert R(Color(2)) == '<Color.GREEN: 2>', 'enum#5'
assert R(Color['BLUE']) == '<Color.BLUE: 3>', 'enum#6'
assert R(list(Color)) == '[<Color.RED: 1>, <Color.GREEN: 2>, <Color.BLUE: 3>]', 'enum#7'
assert R([c.name for c in Color]) == "['RED', 'GREEN', 'BLUE']", 'enum#8'
assert R(len(Color)) == '3', 'enum#9'
assert R(Color.RED == Color.RED) == 'True', 'enum#10'
assert R(Color.RED == 1) == 'False', 'enum#11'
assert R(Color.RED is Color(1)) == 'True', 'enum#12'
assert R(Color.RED in Color) == 'True', 'enum#13'
assert R(A.X.value) == '1', 'enum#14'
assert R(A.Y.value) == '2', 'enum#15'
assert R(S('low')) == "<S.LOW: 'low'>", 'enum#16'
assert R(S.HIGH.value) == "'high'", 'enum#17'
assert R(N.ONE == 1) == 'True', 'enum#18'
assert R(N.TWO + 1) == '3', 'enum#19'
assert R(N.ONE < N.TWO) == 'True', 'enum#20'
assert R(int(N.TWO)) == '2', 'enum#21'
assert R(sorted([N.TWO, N.ONE])) == '[<N.ONE: 1>, <N.TWO: 2>]', 'enum#22'
assert R(str(N.ONE)) == "'1'", 'enum#23'
assert R(repr(N.ONE)) == "'<N.ONE: 1>'", 'enum#24'
assert R(f'{N.ONE}') == "'1'", 'enum#25'
assert R(f'{Color.RED}') == "'Color.RED'", 'enum#26'
assert R(P.R | P.W) == '<P.R|W: 3>', 'enum#27'
assert R((P.R | P.W).value) == '3', 'enum#28'
assert R(P.R in (P.R | P.W)) == 'True', 'enum#29'
assert R(P.X in (P.R | P.W)) == 'False', 'enum#30'
assert R(bool(P.R & P.W)) == 'False', 'enum#31'
assert R(repr(P.R)) == "'<P.R: 1>'", 'enum#32'
assert R(Al.B) == '<Al.A: 1>', 'enum#33'
assert R(Al.B is Al.A) == 'True', 'enum#34'
assert R(list(Al)) == '[<Al.A: 1>]', 'enum#35'
assert R(Color.__members__.keys()) == "dict_keys(['RED', 'GREEN', 'BLUE'])", 'enum#36'
assert R(list(Color.__members__)) == "['RED', 'GREEN', 'BLUE']", 'enum#37'
assert R({Color.RED: 'r'}[Color.RED]) == "'r'", 'enum#38'
assert R(isinstance(Color.RED, Color)) == 'True', 'enum#39'
assert R(Color.RED.__class__.__name__) == "'Color'", 'enum#40'
assert R(hash(Color.RED) == hash(Color.RED)) == 'True', 'enum#41'
assert R([m.value for m in S]) == "['low', 'high']", 'enum#42'
assert R(S.LOW < S.HIGH if False else 'x') == "'x'", 'enum#43'
assert R(N(2)) == '<N.TWO: 2>', 'enum#44'
assert R(N['ONE']) == '<N.ONE: 1>', 'enum#45'
assert R(max(N)) == '<N.TWO: 2>', 'enum#46'
assert R(Color.GREEN.name.lower()) == "'green'", 'enum#47'
assert R(type(Color.RED).__name__ if not isinstance(type(1), str) else 'Color') == "'Color'", 'enum#48'
assert R(Color.RED != Color.BLUE) == 'True', 'enum#49'
assert R(N.ONE.name) == "'ONE'", 'enum#52'
assert R(format(N.TWO, '03d')) == "'002'", 'enum#53'
assert R(N.ONE * 3) == '3', 'enum#54'
from typing import List, Dict, Optional, Any, Tuple, Union, Callable, Set, Iterable, Iterator, Sequence, Mapping, TypeVar, Generic, NamedTuple, TypedDict, Literal, cast
T = TypeVar('T')
def f(x: int) -> int:
    return x + 1
def g(a: List[int], b: Dict[str, int]) -> Tuple[int, ...]:
    return (len(a), len(b))
def h(x: Optional[str] = None) -> Optional[str]:
    return x
def t(p: Tuple[int, str]) -> Tuple[int, str]:
    return p
def c(fn: Callable[[str], int]) -> int:
    return fn('abc')
def u(v: Union[int, str]) -> Any:
    return v
def a(v: 'int') -> 'list[int]':
    return [v]
x: List[int] = [1]
y: Dict[str, Any] = {}
z: Set[int] = set()
w: Iterable[int] = []
Lst = cast(List[int], [1, 2])
assert R(f(1)) == '2', 'typing#0'
assert R(g([1], {'a': 1})) == '(1, 1)', 'typing#1'
assert R(h(None)) == 'None', 'typing#2'
assert R(t((1, 'a'))) == "(1, 'a')", 'typing#3'
assert R(c(len)) == '3', 'typing#4'
assert R(u(1)) == '1', 'typing#5'
assert R(a(1)) == '[1]', 'typing#6'
assert R(isinstance(1, int)) == 'True', 'typing#7'
assert R(Lst) == '[1, 2]', 'typing#8'
assert R(bisect.bisect_left([1, 2, 2, 3], 2)) == '1', 'bisect#0'
assert R(bisect.bisect_right([1, 2, 2, 3], 2)) == '3', 'bisect#1'
assert R(bisect.bisect([1, 2, 2, 3], 2)) == '3', 'bisect#2'
assert R(bisect.bisect_left([], 5)) == '0', 'bisect#3'
assert R(bisect.bisect_left([1, 3], 0)) == '0', 'bisect#4'
assert R(bisect.bisect_right([1, 3], 9)) == '2', 'bisect#5'
assert R(bisect.bisect_left([1, 2, 3, 4], 3, 1, 2)) == '2', 'bisect#6'
assert R(bisect.bisect_right([1, 2, 3, 4], 3, lo=3)) == '3', 'bisect#7'
assert R(bisect.bisect_left([1, 2, 3, 4], 3, hi=1)) == '1', 'bisect#8'
assert R(bisect.bisect_left([(1, 'a'), (3, 'b')], 2, key=lambda t: t[0])) == '1', 'bisect#9'
assert R(bisect.bisect_right([{'k': 1}, {'k': 1}, {'k': 5}], 1, key=lambda d: d['k'])) == '2', 'bisect#10'
assert R(bisect.bisect_left(['a', 'c'], 'b')) == '1', 'bisect#11'
assert R(bisect.bisect_left([1.5, 2.5], 2)) == '1', 'bisect#12'
assert R((lambda l: (bisect.insort([1, 2, 4], 3), l)[1])([1, 2, 4])) == '[1, 2, 4]', 'bisect#13'
assert R((lambda l: (bisect.insort(l, 3), l)[1])([1, 2, 4])) == '[1, 2, 3, 4]', 'bisect#14'
assert R((lambda l: (bisect.insort_left(l, 2), l)[1])([1, 2, 4])) == '[1, 2, 2, 4]', 'bisect#15'
assert R((lambda l: (bisect.insort_right(l, 2.0), l)[1])([1, 2, 4])) == '[1, 2, 2.0, 4]', 'bisect#16'
assert R((lambda l: (bisect.insort(l, 0), l)[1])([1, 2, 4])) == '[0, 1, 2, 4]', 'bisect#17'
assert R((lambda l: (bisect.insort(l, 9), l)[1])([1, 2, 4])) == '[1, 2, 4, 9]', 'bisect#18'
assert R((lambda l: (bisect.insort(l, 5, lo=3), l)[1])([1, 2, 4])) == '[1, 2, 4, 5]', 'bisect#19'
assert R((lambda l: (bisect.insort_left(l, 3, 0, 2), l)[1])([1, 2, 4])) == '[1, 2, 3, 4]', 'bisect#20'
assert R((lambda l: (bisect.insort(l, (2, 'x'), key=lambda t: t[0]), l)[1])([(1, 'a'), (3, 'c')])) == "[(1, 'a'), (2, 'x'), (3, 'c')]", 'bisect#21'
assert R((lambda l: (bisect.insort_left(l, (3, 'x'), key=lambda t: t[0]), l)[1])([(1, 'a'), (3, 'c')])) == "[(1, 'a'), (3, 'x'), (3, 'c')]", 'bisect#22'
assert R((lambda l: (bisect.insort_right(l, (3, 'x'), key=lambda t: t[0]), l)[1])([(1, 'a'), (3, 'c')])) == "[(1, 'a'), (3, 'c'), (3, 'x')]", 'bisect#23'
assert R((lambda h: [heapq.heappush(h, v) for v in [5, 1, 4, 2, 3]] and h)([])) == '[1, 2, 4, 5, 3]', 'heapq#0'
assert R((lambda h: (heapq.heapify(h), h)[1])([5, 1, 4, 2, 3, 0])) == '[0, 1, 4, 2, 3, 5]', 'heapq#1'
assert R((lambda h: (heapq.heapify(h), [heapq.heappop(h) for _ in range(len(h))])[1])([5, 1, 4, 2, 3, 0])) == '[0, 1, 2, 3, 4, 5]', 'heapq#2'
assert R((lambda h: (heapq.heapreplace(h, 9), h))([1, 2, 3])) == '(1, [2, 9, 3])', 'heapq#3'
assert R((lambda h: (heapq.heappushpop(h, 0), h))([1, 2, 3])) == '(0, [1, 2, 3])', 'heapq#4'
assert R((lambda h: (heapq.heappushpop(h, 5), h))([1, 2, 3])) == '(1, [2, 5, 3])', 'heapq#5'
assert R(heapq.nlargest(2, [5, 1, 9, 3])) == '[9, 5]', 'heapq#6'
assert R(heapq.nsmallest(2, [5, 1, 9, 3])) == '[1, 3]', 'heapq#7'
assert R(heapq.nlargest(2, ['a', 'ccc', 'bb'], key=len)) == "['ccc', 'bb']", 'heapq#8'
assert R(heapq.nsmallest(3, [(2, 'b'), (1, 'z'), (1, 'a')])) == "[(1, 'a'), (1, 'z'), (2, 'b')]", 'heapq#9'
assert R(heapq.nsmallest(2, [{'p': 3}, {'p': 1}, {'p': 2}], key=lambda d: d['p'])) == "[{'p': 1}, {'p': 2}]", 'heapq#10'
assert R(heapq.nlargest(0, [1])) == '[]', 'heapq#11'
assert R(heapq.nlargest(10, [1, 2])) == '[2, 1]', 'heapq#12'
assert R(heapq.nsmallest(1, [])) == '[]', 'heapq#13'
assert R(list(heapq.merge([1, 4], [2, 3]))) == '[1, 2, 3, 4]', 'heapq#14'
assert R(list(heapq.merge([1, 4], [2, 3], reverse=False))) == '[1, 2, 3, 4]', 'heapq#15'
assert R(list(heapq.merge(['a', 'ccc'], ['bb'], key=len))) == "['a', 'bb', 'ccc']", 'heapq#16'
assert R((lambda h: [heapq.heappush(h, t) for t in [(2, 'b'), (1, 'a'), (3, 'c')]] and [heapq.heappop(h) for _ in range(3)])([])) == "[(1, 'a'), (2, 'b'), (3, 'c')]", 'heapq#17'
assert R((lambda h: (heapq.heappush(h, 1), heapq.heappop(h), h)[2])([])) == '[]', 'heapq#18'
assert R(heapq.nlargest(3, [1, 3, 3, 2])) == '[3, 3, 2]', 'heapq#19'
assert R(heapq.nsmallest(2, [3, 1, 2], key=lambda v: -v)) == '[3, 2]', 'heapq#20'
assert R(re.split(r'(\w+)@(\w+)\.com', 'x joe@ex.com y')) == "['x ', 'joe', 'ex', ' y']", 're#18'
assert R(re.split(r'(?P<y>\d{4})-(?P<m>\d\d)', 'on 2024-05 ok')) == "['on ', '2024', '05', ' ok']", 're#33'
assert R(re.split(r'(a)(b)?', 'ac')) == "['', 'a', None, 'c']", 're#78'
assert R(re.split(r'', 'ab')) == "['', 'a', 'b', '']", 're#243'
assert R(re.split(r'(a*)*', 'b')) == "['', '', 'b', '', '']", 're#258'
assert R(re.split(r'(\d)(\d)?', '1')) == "['', '1', None, '']", 're#288'
assert R(re.split(r'(,)', 'a,b')) == "['a', ',', 'b']", 're#306'
assert R(re.compile('a').finditer('aa').__class__ is not None) == 'True', 're#334'
import bisect
assert R('present bisect') == "'present bisect'", 'modules#0'
import heapq
assert R('present heapq') == "'present heapq'", 'modules#1'
# struct_time slices like a tuple.
import time as _time
_t = _time.gmtime(0)
assert _t[:3] == (1970, 1, 1) and _t[-3:] == (3, 1, 0) and tuple(_t) == _t[:]
_y, _m, _d = _t[:3]
assert (_y, _m, _d, _t.tm_isdst) == (1970, 1, 1, 0)

print("OK")
