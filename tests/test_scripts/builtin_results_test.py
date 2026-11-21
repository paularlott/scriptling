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

import datetime
import math
assert R(''.isascii()) == 'True', 'str#31'
assert R('hello world'.isascii()) == 'True', 'str#105'
assert R('hello world'.translate({108: None})) == "'heo word'", 'str#132'
assert R('Hello World'.isascii()) == 'True', 'str#179'
assert R('Hello World'.translate({108: None})) == "'Heo Word'", 'str#206'
assert R('  padded  '.isascii()) == 'True', 'str#253'
assert R('a,b,,c'.isascii()) == 'True', 'str#327'
assert R('ÄÖü straße'.casefold()) == "'äöü strasse'", 'str#372'
assert R('ÄÖü straße'.swapcase()) == "'äöÜ STRASSE'", 'str#375'
assert R('ÄÖü straße'.islower()) == 'False', 'str#396'
assert R('ÄÖü straße'.isascii()) == 'False', 'str#401'
assert R('ÄÖü straße'.title().swapcase()) == "'äÖÜ sTRASSE'", 'str#437'
assert R('ǅungla'.capitalize()) == "'ǅungla'", 'str#447'
assert R('ǅungla'.swapcase()) == "'ǅUNGLA'", 'str#449'
assert R('ǅungla'.islower()) == 'False', 'str#470'
assert R('ǅungla'.istitle()) == 'True', 'str#472'
assert R('ǅungla'.isidentifier()) == 'True', 'str#473'
assert R('ǅungla'.isascii()) == 'False', 'str#475'
assert R('ǅungla'.translate({108: None})) == "'ǅunga'", 'str#502'
assert R('ǅungla'.title().swapcase()) == "'ǅUNGLA'", 'str#511'
assert R('123'.isascii()) == 'True', 'str#549'
assert R('12.5'.isascii()) == 'True', 'str#623'
assert R('abc123'.isascii()) == 'True', 'str#697'
assert R('\t tab\n line\r\n'.isascii()) == 'True', 'str#771'
assert R('\t tab\n line\r\n'.translate({108: None})) == "'\\t tab\\n ine\\r\\n'", 'str#798'
assert R('MiXeD cAsE'.isascii()) == 'True', 'str#845'
assert R('ﬁ'.upper()) == "'FI'", 'str#888'
assert R('ﬁ'.casefold()) == "'fi'", 'str#890'
assert R('ﬁ'.capitalize()) == "'Fi'", 'str#891'
assert R('ﬁ'.swapcase()) == "'FI'", 'str#893'
assert R('ﬁ'.islower()) == 'True', 'str#914'
assert R('ﬁ'.isidentifier()) == 'True', 'str#917'
assert R('ﬁ'.isascii()) == 'False', 'str#919'
assert R('İ'.lower()) == "'i̇'", 'str#963'
assert R('İ'.casefold()) == "'i̇'", 'str#964'
assert R('İ'.swapcase()) == "'i̇'", 'str#967'
assert R('İ'.isupper()) == 'True', 'str#989'
assert R('İ'.istitle()) == 'True', 'str#990'
assert R('İ'.isidentifier()) == 'True', 'str#991'
assert R('İ'.isascii()) == 'False', 'str#993'
assert R('İ'.title().swapcase()) == "'i̇'", 'str#1029'
assert R('xxx'.isascii()) == 'True', 'str#1067'
assert R('αβγ'.swapcase()) == "'ΑΒΓ'", 'str#1115'
assert R('αβγ'.islower()) == 'True', 'str#1136'
assert R('αβγ'.isidentifier()) == 'True', 'str#1139'
assert R('αβγ'.isascii()) == 'False', 'str#1141'
assert R('αβγ'.title().swapcase()) == "'αΒΓ'", 'str#1177'
assert R('hello_world-foo bar'.title()) == "'Hello_World-Foo Bar'", 'str#1188'
assert R('hello_world-foo bar'.isascii()) == 'True', 'str#1215'
assert R('hello_world-foo bar'.translate({108: None})) == "'heo_word-foo bar'", 'str#1242'
assert R('hello_world-foo bar'.title().swapcase()) == "'hELLO_wORLD-fOO bAR'", 'str#1251'
assert R('٣'.swapcase()) == "'٣'", 'str#1263'
assert R('٣'.isascii()) == 'False', 'str#1289'
assert R('٣'.title().swapcase()) == "'٣'", 'str#1325'
assert R('½'.swapcase()) == "'½'", 'str#1337'
assert R('½'.isascii()) == 'False', 'str#1363'
assert R('½'.title().swapcase()) == "'½'", 'str#1399'
assert R('Ⅻ'.swapcase()) == "'ⅻ'", 'str#1411'
assert R('Ⅻ'.isupper()) == 'True', 'str#1433'
assert R('Ⅻ'.istitle()) == 'True', 'str#1434'
assert R('Ⅻ'.isidentifier()) == 'True', 'str#1435'
assert R('Ⅻ'.isascii()) == 'False', 'str#1437'
assert R('Ⅻ'.title().swapcase()) == "'ⅻ'", 'str#1473'
assert R('a\u00a0b'.swapcase()) == "'A\\xa0B'", 'str#1485'
assert R('a\u00a0b'.isascii()) == 'False', 'str#1511'
assert R('a\u00a0b'.title().swapcase()) == "'a\\xa0b'", 'str#1547'
assert R(str.maketrans({'a': 'b'})) == "{97: 'b'}", 'str#1560'
assert R(str.maketrans('ab', 'cd', 'e')) == '{97: 99, 98: 100, 101: None}', 'str#1561'
assert R('ß'.casefold()) == "'ss'", 'str#1572'
assert R('ΣΑΣ'.lower()) == "'σας'", 'str#1573'
assert R('İ'.lower()) == "'i̇'", 'str#1574'
assert R(len('İ'.lower())) == '2', 'str#1575'
assert R('ﬃ'.upper()) == "'FFI'", 'str#1576'
assert R('a\x0bb\x1cc'.splitlines()) == "['a', 'b', 'c']", 'str#1581'
assert R('héllo'.encode('ascii', 'xmlcharrefreplace')) == "b'h&#233;llo'", 'str#1593'
assert R('héllo'.encode('ascii', 'backslashreplace')) == "b'h\\\\xe9llo'", 'str#1594'
assert R('they\'re bill\'s'.title()) == '"They\'Re Bill\'S"', 'str#1610'
assert R('²'.isdigit()) == 'True', 'str#1627'
assert R('一'.isnumeric()) == 'True', 'str#1629'
assert R('%*d' % (5, 42)) == "'   42'", 'str#1652'
assert R('%.*f' % (2, 3.14159)) == "'3.14'", 'str#1653'
assert R('%a' % 'é') == '"\'\\\\xe9\'"', 'str#1657'
assert R('{:=+8.2f}'.format(2)) == "'+   2.00'", 'str#1690'
assert R('{:.3}'.format(3.14159)) == "'3.14'", 'str#1699'
assert R('{:f}'.format(float('inf'))) == "'inf'", 'str#1708'
assert R('{:F}'.format(float('nan'))) == "'NAN'", 'str#1709'
assert R('{:e}'.format(float('-inf'))) == "'-inf'", 'str#1710'
assert R('{:#.3g}'.format(1.0)) == "'1.00'", 'str#1712'
assert R('{:0=5}'.format(-3)) == "'-0003'", 'str#1719'
assert R('{:05}'.format('ab')) == "'ab000'", 'str#1721'
assert R('{:%Y}'.format(datetime.date(2024,1,1))) == "'2024'", 'str#1728'
assert R(b''.upper()) == "b''", 'bytes#0'
assert R(b''.lower()) == "b''", 'bytes#1'
assert R(b''.strip()) == "b''", 'bytes#2'
assert R(b''.find(b'o')) == '-1', 'bytes#7'
assert R(b''.count(b'l')) == '0', 'bytes#8'
assert R(b''.replace(b'l', b'L')) == "b''", 'bytes#9'
assert R(b''.startswith(b'he')) == 'False', 'bytes#10'
assert R(b''.endswith(b'd')) == 'False', 'bytes#11'
assert R(b''.isalpha()) == 'False', 'bytes#12'
assert R(b''.isdigit()) == 'False', 'bytes#13'
assert R(b''.isspace()) == 'False', 'bytes#14'
assert R(b''.title()) == "b''", 'bytes#15'
assert R(b''.capitalize()) == "b''", 'bytes#16'
assert R(b''.swapcase()) == "b''", 'bytes#17'
assert R(b''.center(14, b'*')) == "b'**************'", 'bytes#18'
assert R(b''.ljust(12)) == "b'            '", 'bytes#19'
assert R(b''.rjust(12, b'0')) == "b'000000000000'", 'bytes#20'
assert R(b''.zfill(6)) == "b'000000'", 'bytes#21'
assert R(b''.partition(b' ')) == "(b'', b'', b'')", 'bytes#22'
assert R(b''.rpartition(b' ')) == "(b'', b'', b'')", 'bytes#23'
assert R(b''.splitlines()) == '[]', 'bytes#27'
assert R(b''.isalnum()) == 'False', 'bytes#28'
assert R(b''.islower()) == 'False', 'bytes#29'
assert R(b''.isupper()) == 'False', 'bytes#30'
assert R(b''.lstrip(b' h')) == "b''", 'bytes#31'
assert R(b''.index(b'') ) == '0', 'bytes#33'
assert R(b''.removeprefix(b'he')) == "b''", 'bytes#34'
assert R(b''.removesuffix(b'ld')) == "b''", 'bytes#35'
assert R(b''.hex(':')) == "''", 'bytes#36'
assert R(b''.expandtabs()) == "b''", 'bytes#37'
assert R(b''.istitle()) == 'False', 'bytes#38'
assert R(b''.isascii()) == 'True', 'bytes#39'
assert R(b'hello world'.upper()) == "b'HELLO WORLD'", 'bytes#42'
assert R(b'hello world'.lower()) == "b'hello world'", 'bytes#43'
assert R(b'hello world'.strip()) == "b'hello world'", 'bytes#44'
assert R(b'hello world'.find(b'o')) == '4', 'bytes#49'
assert R(b'hello world'.count(b'l')) == '3', 'bytes#50'
assert R(b'hello world'.replace(b'l', b'L')) == "b'heLLo worLd'", 'bytes#51'
assert R(b'hello world'.startswith(b'he')) == 'True', 'bytes#52'
assert R(b'hello world'.endswith(b'd')) == 'True', 'bytes#53'
assert R(b'hello world'.isalpha()) == 'False', 'bytes#54'
assert R(b'hello world'.isdigit()) == 'False', 'bytes#55'
assert R(b'hello world'.isspace()) == 'False', 'bytes#56'
assert R(b'hello world'.title()) == "b'Hello World'", 'bytes#57'
assert R(b'hello world'.capitalize()) == "b'Hello world'", 'bytes#58'
assert R(b'hello world'.swapcase()) == "b'HELLO WORLD'", 'bytes#59'
assert R(b'hello world'.center(14, b'*')) == "b'*hello world**'", 'bytes#60'
assert R(b'hello world'.ljust(12)) == "b'hello world '", 'bytes#61'
assert R(b'hello world'.rjust(12, b'0')) == "b'0hello world'", 'bytes#62'
assert R(b'hello world'.zfill(6)) == "b'hello world'", 'bytes#63'
assert R(b'hello world'.partition(b' ')) == "(b'hello', b' ', b'world')", 'bytes#64'
assert R(b'hello world'.rpartition(b' ')) == "(b'hello', b' ', b'world')", 'bytes#65'
assert R(b'hello world'.splitlines()) == "[b'hello world']", 'bytes#69'
assert R(b'hello world'.isalnum()) == 'False', 'bytes#70'
assert R(b'hello world'.islower()) == 'True', 'bytes#71'
assert R(b'hello world'.isupper()) == 'False', 'bytes#72'
assert R(b'hello world'.lstrip(b' h')) == "b'ello world'", 'bytes#73'
assert R(b'hello world'.index(b'') ) == '0', 'bytes#75'
assert R(b'hello world'.removeprefix(b'he')) == "b'llo world'", 'bytes#76'
assert R(b'hello world'.removesuffix(b'ld')) == "b'hello wor'", 'bytes#77'
assert R(b'hello world'.hex(':')) == "'68:65:6c:6c:6f:20:77:6f:72:6c:64'", 'bytes#78'
assert R(b'hello world'.expandtabs()) == "b'hello world'", 'bytes#79'
assert R(b'hello world'.istitle()) == 'False', 'bytes#80'
assert R(b'hello world'.isascii()) == 'True', 'bytes#81'
assert R(b'ABC abc'.upper()) == "b'ABC ABC'", 'bytes#84'
assert R(b'ABC abc'.lower()) == "b'abc abc'", 'bytes#85'
assert R(b'ABC abc'.strip()) == "b'ABC abc'", 'bytes#86'
assert R(b'ABC abc'.find(b'o')) == '-1', 'bytes#91'
assert R(b'ABC abc'.count(b'l')) == '0', 'bytes#92'
assert R(b'ABC abc'.replace(b'l', b'L')) == "b'ABC abc'", 'bytes#93'
assert R(b'ABC abc'.startswith(b'he')) == 'False', 'bytes#94'
assert R(b'ABC abc'.endswith(b'd')) == 'False', 'bytes#95'
assert R(b'ABC abc'.isalpha()) == 'False', 'bytes#96'
assert R(b'ABC abc'.isdigit()) == 'False', 'bytes#97'
assert R(b'ABC abc'.isspace()) == 'False', 'bytes#98'
assert R(b'ABC abc'.title()) == "b'Abc Abc'", 'bytes#99'
assert R(b'ABC abc'.capitalize()) == "b'Abc abc'", 'bytes#100'
assert R(b'ABC abc'.swapcase()) == "b'abc ABC'", 'bytes#101'
assert R(b'ABC abc'.center(14, b'*')) == "b'***ABC abc****'", 'bytes#102'
assert R(b'ABC abc'.ljust(12)) == "b'ABC abc     '", 'bytes#103'
assert R(b'ABC abc'.rjust(12, b'0')) == "b'00000ABC abc'", 'bytes#104'
assert R(b'ABC abc'.zfill(6)) == "b'ABC abc'", 'bytes#105'
assert R(b'ABC abc'.partition(b' ')) == "(b'ABC', b' ', b'abc')", 'bytes#106'
assert R(b'ABC abc'.rpartition(b' ')) == "(b'ABC', b' ', b'abc')", 'bytes#107'
assert R(b'ABC abc'.splitlines()) == "[b'ABC abc']", 'bytes#111'
assert R(b'ABC abc'.isalnum()) == 'False', 'bytes#112'
assert R(b'ABC abc'.islower()) == 'False', 'bytes#113'
assert R(b'ABC abc'.isupper()) == 'False', 'bytes#114'
assert R(b'ABC abc'.lstrip(b' h')) == "b'ABC abc'", 'bytes#115'
assert R(b'ABC abc'.index(b'') ) == '0', 'bytes#117'
assert R(b'ABC abc'.removeprefix(b'he')) == "b'ABC abc'", 'bytes#118'
assert R(b'ABC abc'.removesuffix(b'ld')) == "b'ABC abc'", 'bytes#119'
assert R(b'ABC abc'.hex(':')) == "'41:42:43:20:61:62:63'", 'bytes#120'
assert R(b'ABC abc'.expandtabs()) == "b'ABC abc'", 'bytes#121'
assert R(b'ABC abc'.istitle()) == 'False', 'bytes#122'
assert R(b'ABC abc'.isascii()) == 'True', 'bytes#123'
assert R(b'\x00\xff'.upper()) == "b'\\x00\\xff'", 'bytes#126'
assert R(b'\x00\xff'.lower()) == "b'\\x00\\xff'", 'bytes#127'
assert R(b'\x00\xff'.strip()) == "b'\\x00\\xff'", 'bytes#128'
assert R(b'\x00\xff'.find(b'o')) == '-1', 'bytes#133'
assert R(b'\x00\xff'.count(b'l')) == '0', 'bytes#134'
assert R(b'\x00\xff'.replace(b'l', b'L')) == "b'\\x00\\xff'", 'bytes#135'
assert R(b'\x00\xff'.startswith(b'he')) == 'False', 'bytes#136'
assert R(b'\x00\xff'.endswith(b'd')) == 'False', 'bytes#137'
assert R(b'\x00\xff'.isalpha()) == 'False', 'bytes#138'
assert R(b'\x00\xff'.isdigit()) == 'False', 'bytes#139'
assert R(b'\x00\xff'.isspace()) == 'False', 'bytes#140'
assert R(b'\x00\xff'.title()) == "b'\\x00\\xff'", 'bytes#141'
assert R(b'\x00\xff'.capitalize()) == "b'\\x00\\xff'", 'bytes#142'
assert R(b'\x00\xff'.swapcase()) == "b'\\x00\\xff'", 'bytes#143'
assert R(b'\x00\xff'.center(14, b'*')) == "b'******\\x00\\xff******'", 'bytes#144'
assert R(b'\x00\xff'.ljust(12)) == "b'\\x00\\xff          '", 'bytes#145'
assert R(b'\x00\xff'.rjust(12, b'0')) == "b'0000000000\\x00\\xff'", 'bytes#146'
assert R(b'\x00\xff'.zfill(6)) == "b'0000\\x00\\xff'", 'bytes#147'
assert R(b'\x00\xff'.partition(b' ')) == "(b'\\x00\\xff', b'', b'')", 'bytes#148'
assert R(b'\x00\xff'.rpartition(b' ')) == "(b'', b'', b'\\x00\\xff')", 'bytes#149'
assert R(b'\x00\xff'.splitlines()) == "[b'\\x00\\xff']", 'bytes#153'
assert R(b'\x00\xff'.isalnum()) == 'False', 'bytes#154'
assert R(b'\x00\xff'.islower()) == 'False', 'bytes#155'
assert R(b'\x00\xff'.isupper()) == 'False', 'bytes#156'
assert R(b'\x00\xff'.lstrip(b' h')) == "b'\\x00\\xff'", 'bytes#157'
assert R(b'\x00\xff'.index(b'') ) == '0', 'bytes#159'
assert R(b'\x00\xff'.removeprefix(b'he')) == "b'\\x00\\xff'", 'bytes#160'
assert R(b'\x00\xff'.removesuffix(b'ld')) == "b'\\x00\\xff'", 'bytes#161'
assert R(b'\x00\xff'.hex(':')) == "'00:ff'", 'bytes#162'
assert R(b'\x00\xff'.expandtabs()) == "b'\\x00\\xff'", 'bytes#163'
assert R(b'\x00\xff'.istitle()) == 'False', 'bytes#164'
assert R(b'\x00\xff'.isascii()) == 'False', 'bytes#165'
assert R(b'  x  '.upper()) == "b'  X  '", 'bytes#168'
assert R(b'  x  '.lower()) == "b'  x  '", 'bytes#169'
assert R(b'  x  '.strip()) == "b'x'", 'bytes#170'
assert R(b'  x  '.find(b'o')) == '-1', 'bytes#175'
assert R(b'  x  '.count(b'l')) == '0', 'bytes#176'
assert R(b'  x  '.replace(b'l', b'L')) == "b'  x  '", 'bytes#177'
assert R(b'  x  '.startswith(b'he')) == 'False', 'bytes#178'
assert R(b'  x  '.endswith(b'd')) == 'False', 'bytes#179'
assert R(b'  x  '.isalpha()) == 'False', 'bytes#180'
assert R(b'  x  '.isdigit()) == 'False', 'bytes#181'
assert R(b'  x  '.isspace()) == 'False', 'bytes#182'
assert R(b'  x  '.title()) == "b'  X  '", 'bytes#183'
assert R(b'  x  '.capitalize()) == "b'  x  '", 'bytes#184'
assert R(b'  x  '.swapcase()) == "b'  X  '", 'bytes#185'
assert R(b'  x  '.center(14, b'*')) == "b'****  x  *****'", 'bytes#186'
assert R(b'  x  '.ljust(12)) == "b'  x         '", 'bytes#187'
assert R(b'  x  '.rjust(12, b'0')) == "b'0000000  x  '", 'bytes#188'
assert R(b'  x  '.zfill(6)) == "b'0  x  '", 'bytes#189'
assert R(b'  x  '.partition(b' ')) == "(b'', b' ', b' x  ')", 'bytes#190'
assert R(b'  x  '.rpartition(b' ')) == "(b'  x ', b' ', b'')", 'bytes#191'
assert R(b'  x  '.splitlines()) == "[b'  x  ']", 'bytes#195'
assert R(b'  x  '.isalnum()) == 'False', 'bytes#196'
assert R(b'  x  '.islower()) == 'True', 'bytes#197'
assert R(b'  x  '.isupper()) == 'False', 'bytes#198'
assert R(b'  x  '.lstrip(b' h')) == "b'x  '", 'bytes#199'
assert R(b'  x  '.index(b'') ) == '0', 'bytes#201'
assert R(b'  x  '.removeprefix(b'he')) == "b'  x  '", 'bytes#202'
assert R(b'  x  '.removesuffix(b'ld')) == "b'  x  '", 'bytes#203'
assert R(b'  x  '.hex(':')) == "'20:20:78:20:20'", 'bytes#204'
assert R(b'  x  '.expandtabs()) == "b'  x  '", 'bytes#205'
assert R(b'  x  '.istitle()) == 'False', 'bytes#206'
assert R(b'  x  '.isascii()) == 'True', 'bytes#207'
assert R(b'a,b,c'.upper()) == "b'A,B,C'", 'bytes#210'
assert R(b'a,b,c'.lower()) == "b'a,b,c'", 'bytes#211'
assert R(b'a,b,c'.strip()) == "b'a,b,c'", 'bytes#212'
assert R(b'a,b,c'.find(b'o')) == '-1', 'bytes#217'
assert R(b'a,b,c'.count(b'l')) == '0', 'bytes#218'
assert R(b'a,b,c'.replace(b'l', b'L')) == "b'a,b,c'", 'bytes#219'
assert R(b'a,b,c'.startswith(b'he')) == 'False', 'bytes#220'
assert R(b'a,b,c'.endswith(b'd')) == 'False', 'bytes#221'
assert R(b'a,b,c'.isalpha()) == 'False', 'bytes#222'
assert R(b'a,b,c'.isdigit()) == 'False', 'bytes#223'
assert R(b'a,b,c'.isspace()) == 'False', 'bytes#224'
assert R(b'a,b,c'.title()) == "b'A,B,C'", 'bytes#225'
assert R(b'a,b,c'.capitalize()) == "b'A,b,c'", 'bytes#226'
assert R(b'a,b,c'.swapcase()) == "b'A,B,C'", 'bytes#227'
assert R(b'a,b,c'.center(14, b'*')) == "b'****a,b,c*****'", 'bytes#228'
assert R(b'a,b,c'.ljust(12)) == "b'a,b,c       '", 'bytes#229'
assert R(b'a,b,c'.rjust(12, b'0')) == "b'0000000a,b,c'", 'bytes#230'
assert R(b'a,b,c'.zfill(6)) == "b'0a,b,c'", 'bytes#231'
assert R(b'a,b,c'.partition(b' ')) == "(b'a,b,c', b'', b'')", 'bytes#232'
assert R(b'a,b,c'.rpartition(b' ')) == "(b'', b'', b'a,b,c')", 'bytes#233'
assert R(b'a,b,c'.splitlines()) == "[b'a,b,c']", 'bytes#237'
assert R(b'a,b,c'.isalnum()) == 'False', 'bytes#238'
assert R(b'a,b,c'.islower()) == 'True', 'bytes#239'
assert R(b'a,b,c'.isupper()) == 'False', 'bytes#240'
assert R(b'a,b,c'.lstrip(b' h')) == "b'a,b,c'", 'bytes#241'
assert R(b'a,b,c'.index(b'') ) == '0', 'bytes#243'
assert R(b'a,b,c'.removeprefix(b'he')) == "b'a,b,c'", 'bytes#244'
assert R(b'a,b,c'.removesuffix(b'ld')) == "b'a,b,c'", 'bytes#245'
assert R(b'a,b,c'.hex(':')) == "'61:2c:62:2c:63'", 'bytes#246'
assert R(b'a,b,c'.expandtabs()) == "b'a,b,c'", 'bytes#247'
assert R(b'a,b,c'.istitle()) == 'False', 'bytes#248'
assert R(b'a,b,c'.isascii()) == 'True', 'bytes#249'
assert R(b'123'.upper()) == "b'123'", 'bytes#252'
assert R(b'123'.lower()) == "b'123'", 'bytes#253'
assert R(b'123'.strip()) == "b'123'", 'bytes#254'
assert R(b'123'.find(b'o')) == '-1', 'bytes#259'
assert R(b'123'.count(b'l')) == '0', 'bytes#260'
assert R(b'123'.replace(b'l', b'L')) == "b'123'", 'bytes#261'
assert R(b'123'.startswith(b'he')) == 'False', 'bytes#262'
assert R(b'123'.endswith(b'd')) == 'False', 'bytes#263'
assert R(b'123'.isalpha()) == 'False', 'bytes#264'
assert R(b'123'.isdigit()) == 'True', 'bytes#265'
assert R(b'123'.isspace()) == 'False', 'bytes#266'
assert R(b'123'.title()) == "b'123'", 'bytes#267'
assert R(b'123'.capitalize()) == "b'123'", 'bytes#268'
assert R(b'123'.swapcase()) == "b'123'", 'bytes#269'
assert R(b'123'.center(14, b'*')) == "b'*****123******'", 'bytes#270'
assert R(b'123'.ljust(12)) == "b'123         '", 'bytes#271'
assert R(b'123'.rjust(12, b'0')) == "b'000000000123'", 'bytes#272'
assert R(b'123'.zfill(6)) == "b'000123'", 'bytes#273'
assert R(b'123'.partition(b' ')) == "(b'123', b'', b'')", 'bytes#274'
assert R(b'123'.rpartition(b' ')) == "(b'', b'', b'123')", 'bytes#275'
assert R(b'123'.splitlines()) == "[b'123']", 'bytes#279'
assert R(b'123'.isalnum()) == 'True', 'bytes#280'
assert R(b'123'.islower()) == 'False', 'bytes#281'
assert R(b'123'.isupper()) == 'False', 'bytes#282'
assert R(b'123'.lstrip(b' h')) == "b'123'", 'bytes#283'
assert R(b'123'.index(b'') ) == '0', 'bytes#285'
assert R(b'123'.removeprefix(b'he')) == "b'123'", 'bytes#286'
assert R(b'123'.removesuffix(b'ld')) == "b'123'", 'bytes#287'
assert R(b'123'.hex(':')) == "'31:32:33'", 'bytes#288'
assert R(b'123'.expandtabs()) == "b'123'", 'bytes#289'
assert R(b'123'.istitle()) == 'False', 'bytes#290'
assert R(b'123'.isascii()) == 'True', 'bytes#291'
assert R(bytes.fromhex('de ad be ef')) == "b'\\xde\\xad\\xbe\\xef'", 'bytes#297'
assert R(bytes.maketrans(b'ab', b'xy')[97]) == '120', 'bytes#315'
assert R(b'abc'.translate(bytes.maketrans(b'ab', b'xy'))) == "b'xyc'", 'bytes#316'
assert R(b'abc'.count(98)) == '1', 'bytes#320'
assert R(b'abc'.find(99)) == '2', 'bytes#321'
assert R((0).__class__.__name__) == "'int'", 'num#11'
assert R((1).__class__.__name__) == "'int'", 'num#35'
assert R((-1).__class__.__name__) == "'int'", 'num#59'
assert R((7).__class__.__name__) == "'int'", 'num#83'
assert R((-7).__class__.__name__) == "'int'", 'num#107'
assert R((255).__class__.__name__) == "'int'", 'num#131'
assert R(round(2**62, -1)) == '4611686018427387900', 'num#153'
assert R((2**62).__class__.__name__) == "'int'", 'num#155'
assert R((0.0).__class__.__name__) == "'float'", 'num#203'
assert R((-0.0).__class__.__name__) == "'float'", 'num#227'
assert R((-0.0) % 3) == '0.0', 'num#230'
assert R((0.1).__class__.__name__) == "'float'", 'num#251'
assert R((1.5).__class__.__name__) == "'float'", 'num#275'
assert R((2.5).__class__.__name__) == "'float'", 'num#299'
assert R(round(-2.5, -1)) == '-0.0', 'num#321'
assert R((-2.5).__class__.__name__) == "'float'", 'num#323'
assert R((1e16).__class__.__name__) == "'float'", 'num#347'
assert R((1e-5).__class__.__name__) == "'float'", 'num#371'
assert R((123456789.123).__class__.__name__) == "'float'", 'num#395'
assert R((float('inf')).__class__.__name__) == "'float'", 'num#419'
assert R(divmod(float('inf'), 3)) == '(nan, nan)', 'num#420'
assert R((float('inf')) // 2) == 'nan', 'num#421'
assert R(format(float('inf'), '.3g')) == "'inf'", 'num#429'
assert R('%s|%r|%.2f|%e|%g' % ((float('inf'),) * 5)) == "'inf|inf|inf|inf|inf'", 'num#431'
assert R((float('-inf')).__class__.__name__) == "'float'", 'num#443'
assert R(divmod(float('-inf'), 3)) == '(nan, nan)', 'num#444'
assert R((float('-inf')) // 2) == 'nan', 'num#445'
assert R(format(float('-inf'), '.3g')) == "'-inf'", 'num#453'
assert R('%s|%r|%.2f|%e|%g' % ((float('-inf'),) * 5)) == "'-inf|-inf|-inf|-inf|-inf'", 'num#455'
assert R((float('nan')).__class__.__name__) == "'float'", 'num#467'
assert R(format(float('nan'), '.3g')) == "'nan'", 'num#477'
assert R('%s|%r|%.2f|%e|%g' % ((float('nan'),) * 5)) == "'nan|nan|nan|nan|nan'", 'num#479'
assert R((True).__class__.__name__) == "'bool'", 'num#491'
assert R((False).__class__.__name__) == "'bool'", 'num#515'
assert R((1e300).__class__.__name__) == "'float'", 'num#539'
try:
    R((1e300) ** 2)
    raise AssertionError('num#544')
except OverflowError:
    pass
assert R((5e-324).__class__.__name__) == "'float'", 'num#563'
assert R((0.30000000000000004).__class__.__name__) == "'float'", 'num#587'
assert R((1/3).__class__.__name__) == "'float'", 'num#611'
assert R((100.0).__class__.__name__) == "'float'", 'num#635'
assert R((1e22).__class__.__name__) == "'float'", 'num#659'
assert R(divmod(1e22, 3)) == '(3.3333333333333335e+21, 1.0)', 'num#660'
assert R((1.0e-10).__class__.__name__) == "'float'", 'num#683'
assert R(7 % -0.5) == '-0.0', 'arith#19'
assert R(divmod(7, -0.5)) == '(-14.0, -0.0)', 'arith#20'
assert R(-7 % 0.5) == '0.0', 'arith#49'
assert R(divmod(-7, -0.5)) == '(14.0, -0.0)', 'arith#56'
assert R(7.5 % -0.5) == '-0.0', 'arith#91'
assert R(divmod(7.5, -0.5)) == '(-15.0, -0.0)', 'arith#92'
assert R(-7.5 % 0.5) == '0.0', 'arith#121'
assert R(divmod(-7.5, -0.5)) == '(15.0, -0.0)', 'arith#128'
assert R(0.5 % -0.5) == '-0.0', 'arith#163'
assert R(divmod(0.5, -0.5)) == '(-1.0, -0.0)', 'arith#164'
assert R(3 % -0.5) == '-0.0', 'arith#199'
assert R(divmod(3, -0.5)) == '(-6.0, -0.0)', 'arith#200'
assert R(3 % -3.0) == '-0.0', 'arith#211'
assert R(divmod(3, -3.0)) == '(-1.0, -0.0)', 'arith#212'
assert R(-3 % 0.5) == '0.0', 'arith#229'
assert R(divmod(-3, -0.5)) == '(6.0, -0.0)', 'arith#236'
assert R(divmod(-3, -3.0)) == '(1.0, -0.0)', 'arith#248'
assert R(-0.0 % 1) == '0.0', 'arith#332'
assert R(sum([0.1, 0.2, 0.3])) == '0.6', 'arith#411'
assert R((lambda l: (l.extend('ab'), l)[1])([5, 3, 1, 4])) == "[5, 3, 1, 4, 'a', 'b']", 'list#6'
assert R(sum([0.1] * 10)) == '1.0', 'list#53'
assert R({'a': 1}.keys() & {'a', 'b'}) == "S['a']", 'dict#40'
assert R({'a': 1}.keys() | {'c'}) == "S['a', 'c']", 'dict#41'
assert R({'a': 1, 'b': 2}.keys() - {'a'}) == "S['b']", 'dict#42'
assert R(repr(NotImplemented)) == "'NotImplemented'", 'builtins#116'
class V:
    count = 0
    def __init__(self, x, y=0):
        self.x = x
        self.y = y
        V.count += 1
    def __repr__(self):
        return 'V(%r, %r)' % (self.x, self.y)
    def __str__(self):
        return '<%s,%s>' % (self.x, self.y)
    def __eq__(self, o):
        return isinstance(o, V) and (self.x, self.y) == (o.x, o.y)
    def __hash__(self):
        return hash((self.x, self.y))
    def __lt__(self, o):
        return (self.x, self.y) < (o.x, o.y)
    def __add__(self, o):
        if isinstance(o, V):
            return V(self.x + o.x, self.y + o.y)
        return V(self.x + o, self.y + o)
    def __radd__(self, o):
        return V(self.x + o, self.y + o)
    def __iadd__(self, o):
        self.x += o
        return self
    def __mul__(self, k):
        return V(self.x * k, self.y * k)
    __rmul__ = __mul__
    def __neg__(self):
        return V(-self.x, -self.y)
    def __abs__(self):
        return (self.x ** 2 + self.y ** 2) ** 0.5
    def __bool__(self):
        return bool(self.x or self.y)
    def __len__(self):
        return 2
    def __getitem__(self, i):
        return (self.x, self.y)[i]
    def __contains__(self, v):
        return v in (self.x, self.y)
    def __call__(self, k):
        return self.x * k
    def __format__(self, spec):
        return 'V' + format(self.x, spec)
class Seq:
    def __init__(self, n):
        self.n = n
    def __len__(self):
        return self.n
    def __getitem__(self, i):
        if i >= self.n:
            raise IndexError(i)
        return i * i
class It:
    def __init__(self, n):
        self.i = 0
        self.n = n
    def __iter__(self):
        return self
    def __next__(self):
        if self.i >= self.n:
            raise StopIteration
        self.i += 1
        return self.i
class Store:
    def __init__(self):
        self.d = {}
    def __setitem__(self, k, v):
        self.d[k] = v
    def __getitem__(self, k):
        return self.d[k]
    def __delitem__(self, k):
        del self.d[k]
    def __getattr__(self, name):
        return 'missing:' + name
class Temp:
    def __init__(self):
        self._c = 0
    @property
    def c(self):
        return self._c
    @c.setter
    def c(self, v):
        self._c = v * 2
    @classmethod
    def make(cls):
        return cls.__name__
    @staticmethod
    def st(a):
        return a + 1
class Base:
    kind = 'base'
    def __init__(self, n):
        self.n = n
    def who(self):
        return 'Base' + str(self.n)
    def hello(self):
        return 'hello from ' + self.who()
class Child(Base):
    kind = 'child'
    def __init__(self, n, m):
        super().__init__(n)
        self.m = m
    def who(self):
        return 'Child' + str(self.n) + super().who()
class MyErr(Exception):
    def __init__(self, msg, code):
        super().__init__(msg, code)
        self.code = code
class Simple(Exception):
    pass
class Cmp:
    def __init__(self, v):
        self.v = v
    def __eq__(self, o):
        return self.v == o.v
    def __le__(self, o):
        return self.v <= o.v
    def __gt__(self, o):
        return self.v > o.v
assert R(list(V(7, 8))) == '[7, 8]', 'core#28'
assert R(list(Seq(4))) == '[0, 1, 4, 9]', 'core#33'
assert R(3 in Seq(4)) == 'False', 'core#34'
assert R(tuple(Seq(2))) == '(0, 1)', 'core#41'
assert R(repr(MyErr('m', 3))) == '"MyErr(\'m\', 3)"', 'core#80'
assert R(repr(Simple())) == "'Simple()'", 'core#83'
assert R(repr(Simple('a'))) == '"Simple(\'a\')"', 'core#85'
import functools
def deco(f):
    @functools.wraps(f)
    def w(*a, **k):
        return ('deco', f(*a, **k))
    return w
def rep(n):
    def d(f):
        def w(*a):
            return [f(*a) for _ in range(n)]
        return w
    return d
def plain(f):
    def inner(*a):
        return f(*a)
    return inner
@deco
def add(a, b=2):
    '''adds'''
    return a + b
@rep(2)
@deco
def two(x):
    return x
@plain
def pl(): pass
class M:
    @deco
    def meth(self, v):
        return v * 2
def mk():
    fs = []
    for i in range(3):
        fs.append(lambda: i)
    return [f() for f in fs]
def mk2():
    return [(lambda i=i: i)() for i in range(3)]
def counter():
    n = 0
    def inc():
        nonlocal n
        n += 1
        return n
    return inc
G = 0
def setg():
    global G
    G += 5
    return G
def dflt(a, l=[]):
    l.append(a)
    return l
def kw(a, *args, b=1, **kw):
    return (a, args, b, kw)
def posonly(a, b, /, c):
    return (a, b, c)
def kwonly(*, k):
    return k
def unpack():
    a, *b, c = [1, 2, 3, 4]
    (x, y), z = (1, 2), 3
    first, *rest = 'abc'
    *init, last = range(4)
    return (a, b, c, x, y, z, first, rest, init, last)
def swap():
    a, b = 1, 2
    a, b = b, a
    return (a, b)
def chaincmp(x):
    return (1 < x < 5, x == x == x, 1 < x > 0, 0 < x <= 2 < 3)
def shortc():
    return (0 or 'b', '' and 'x', None or 0, 1 and 2, [] or {}, 'a' or 'b', not '', not [0])
def walrus():
    out = []
    data = [1, 5, 3]
    if (n := len(data)) > 2:
        out.append(n)
    out.append([y for x in data if (y := x * 2) > 4])
    return out
def ternary(x):
    return 'pos' if x > 0 else 'zero' if x == 0 else 'neg'
def comp():
    return ([x * y for x in range(3) for y in range(3) if x != y], {x: x ** 2 for x in range(4) if x % 2}, sorted({c for c in 'hello'}),
            list(x for x in range(3)), [[j for j in range(i)] for i in range(3)], {k: v for k, v in zip('ab', 'cd')})
def mtch(v):
    match v:
        case 0:
            return 'zero'
        case [x, y]:
            return 'pair %s %s' % (x, y)
        case [x, *rest]:
            return 'list starting %s, %d more' % (x, len(rest))
        case {'k': val}:
            return 'dict k=' + str(val)
        case str() as s if len(s) > 3:
            return 'long str ' + s
        case str():
            return 'str'
        case int() | float():
            return 'number'
        case None:
            return 'none'
        case _:
            return 'other'
def strs_iter():
    return [c.upper() for c in 'héllo'] + [len('日本語')] + ['日本語'[1]] + ['ab'[::-1]]
def gen_basic():
    def g(n):
        for i in range(n):
            yield i * i
    x = g(4)
    return (next(x), list(x), sum(g(3)), list(g(0)))
def lam_sort():
    data = [{'n': 'b', 'a': 2}, {'n': 'a', 'a': 2}, {'n': 'c', 'a': 1}]
    return [d['n'] for d in sorted(data, key=lambda d: (d['a'], d['n']))]
def multi_assign():
    a = b = []
    a.append(1)
    x = [1, 2, 3]
    x[0], x[2] = x[2], x[0]
    return (b, x)
def aug():
    s = 'a'
    s += 'b'
    s *= 2
    l = [1]
    l += [2]
    l *= 2
    d = {'k': 1}
    d['k'] += 1
    t = (1,)
    t += (2,)
    n = 7
    n //= 2
    n **= 3
    n %= 5
    f = 1.5
    f -= 0.5
    return (s, l, d, t, n, f)
def is_tests():
    a = [1]
    b = a
    return (a is b, a is not [1], None is None, a == [1], 1 == True, 0 == False, '' == False)
def truthy():
    return [bool(v) for v in [0, 1, '', 'a', [], [0], {}, {0: 0}, set(), None, 0.0, -1, (), (0,), range(0)]]
def scopes():
    x = 'outer'
    def f():
        return x
    x = 'changed'
    return f()
def recursion(n):
    return 1 if n <= 1 else n * recursion(n - 1)
def star_call():
    def f(a, b, c=3, *rest, **kw):
        return (a, b, c, rest, sorted(kw.items()))
    return (f(*[1, 2]), f(1, *(2, 4, 5)), f(**{'a': 1, 'b': 2, 'z': 0}), f(1, 2, 3, 4, d=5))
def dict_order():
    d = {}
    d['b'] = 1
    d['a'] = 2
    d['c'] = 3
    del d['a']
    d['a'] = 4
    return list(d.items())
def nested_fn_default():
    def f(x, y=None):
        if y is None:
            y = []
        y.append(x)
        return y
    return (f(1), f(2))
def while_else():
    i = 0
    out = []
    while i < 3:
        i += 1
        if i == 5:
            break
    else:
        out.append('else ran')
    return out
def strjoin_types():
    return ', '.join(str(x) for x in [1, 2.5, None, True])
def enumerate_unpack():
    return [(i, a, b) for i, (a, b) in enumerate([(1, 2), (3, 4)])]
def dict_iter_mut():
    d = {'a': 1, 'b': 2}
    for k in list(d):
        if k == 'a':
            del d[k]
    return d
def class_attr_mut():
    class C:
        items = []
        def add(self, v):
            self.items.append(v)
    a, b = C(), C()
    a.add(1)
    return (b.items, C.items)
def str_mult_cmp():
    return ('abc' * 2, 'a' < 'b' < 'c', 'abc'.upper().lower(), 'x'.join('yz'), '%s' % 5, 'a' 'b')
def global_in_comp():
    return [G for _ in range(2)]
assert R([mtch(v) for v in [0, [1, 2], [1, 2, 3], {'k': 5, 'o': 1}, 'hello', 'hi', 3.5, None, (1, 2)]]) == "['zero', 'pair 1 2', 'list starting 1, 2 more', 'dict k=5', 'long str hello', 'str', 'number', 'none', 'pair 1 2']", 'core#147'
assert R([f.__name__ for f in (len, str.upper)]) == "['len', 'upper']", 'core#170'
print("OK")
