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
import time
assert R(datetime.datetime.strptime('06/05/24', '%d/%m/%y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#0'
assert R(str(datetime.datetime.strptime('06/05/24', '%d/%m/%y'))) == "'2024-05-06 00:00:00'", 'dtparse#1'
assert R(datetime.datetime.strptime('06/05/69', '%d/%m/%y')) == 'datetime.datetime(1969, 5, 6, 0, 0)', 'dtparse#2'
assert R(str(datetime.datetime.strptime('06/05/69', '%d/%m/%y'))) == "'1969-05-06 00:00:00'", 'dtparse#3'
assert R(datetime.datetime.strptime('06/05/68', '%d/%m/%y')) == 'datetime.datetime(2068, 5, 6, 0, 0)', 'dtparse#4'
assert R(str(datetime.datetime.strptime('06/05/68', '%d/%m/%y'))) == "'2068-05-06 00:00:00'", 'dtparse#5'
assert R(datetime.datetime.strptime('06/05/00', '%d/%m/%y')) == 'datetime.datetime(2000, 5, 6, 0, 0)', 'dtparse#6'
assert R(str(datetime.datetime.strptime('06/05/00', '%d/%m/%y'))) == "'2000-05-06 00:00:00'", 'dtparse#7'
assert R(datetime.datetime.strptime('May 6 2024', '%b %d %Y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#8'
assert R(str(datetime.datetime.strptime('May 6 2024', '%b %d %Y'))) == "'2024-05-06 00:00:00'", 'dtparse#9'
assert R(datetime.datetime.strptime('may 06 2024', '%b %d %Y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#10'
assert R(str(datetime.datetime.strptime('may 06 2024', '%b %d %Y'))) == "'2024-05-06 00:00:00'", 'dtparse#11'
assert R(datetime.datetime.strptime('MAY 6 2024', '%B %d %Y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#12'
assert R(str(datetime.datetime.strptime('MAY 6 2024', '%B %d %Y'))) == "'2024-05-06 00:00:00'", 'dtparse#13'
assert R(datetime.datetime.strptime('May 6 2024', '%B %d %Y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#14'
assert R(str(datetime.datetime.strptime('May 6 2024', '%B %d %Y'))) == "'2024-05-06 00:00:00'", 'dtparse#15'
assert R(datetime.datetime.strptime('September 30 2024', '%B %d %Y')) == 'datetime.datetime(2024, 9, 30, 0, 0)', 'dtparse#16'
assert R(str(datetime.datetime.strptime('September 30 2024', '%B %d %Y'))) == "'2024-09-30 00:00:00'", 'dtparse#17'
assert R(datetime.datetime.strptime('Sep 30 2024', '%b %d %Y')) == 'datetime.datetime(2024, 9, 30, 0, 0)', 'dtparse#18'
assert R(str(datetime.datetime.strptime('Sep 30 2024', '%b %d %Y'))) == "'2024-09-30 00:00:00'", 'dtparse#19'
assert R(datetime.datetime.strptime('Mon May 06 2024', '%a %b %d %Y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#20'
assert R(str(datetime.datetime.strptime('Mon May 06 2024', '%a %b %d %Y'))) == "'2024-05-06 00:00:00'", 'dtparse#21'
assert R(datetime.datetime.strptime('monday, 6 may 2024', '%A, %d %B %Y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#22'
assert R(str(datetime.datetime.strptime('monday, 6 may 2024', '%A, %d %B %Y'))) == "'2024-05-06 00:00:00'", 'dtparse#23'
assert R(datetime.datetime.strptime('2024-127', '%Y-%j')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#24'
assert R(str(datetime.datetime.strptime('2024-127', '%Y-%j'))) == "'2024-05-06 00:00:00'", 'dtparse#25'
assert R(datetime.datetime.strptime('2024-1', '%Y-%j')) == 'datetime.datetime(2024, 1, 1, 0, 0)', 'dtparse#26'
assert R(str(datetime.datetime.strptime('2024-1', '%Y-%j'))) == "'2024-01-01 00:00:00'", 'dtparse#27'
assert R(datetime.datetime.strptime('2024-05-06 07:08:09.123456', '%Y-%m-%d %H:%M:%S.%f')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, 123456)', 'dtparse#28'
assert R(str(datetime.datetime.strptime('2024-05-06 07:08:09.123456', '%Y-%m-%d %H:%M:%S.%f'))) == "'2024-05-06 07:08:09.123456'", 'dtparse#29'
assert R(datetime.datetime.strptime('2024-05-06 07:08:09.1', '%Y-%m-%d %H:%M:%S.%f')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, 100000)', 'dtparse#30'
assert R(str(datetime.datetime.strptime('2024-05-06 07:08:09.1', '%Y-%m-%d %H:%M:%S.%f'))) == "'2024-05-06 07:08:09.100000'", 'dtparse#31'
assert R(datetime.datetime.strptime('2024-05-06 07:08:09.000123', '%Y-%m-%d %H:%M:%S.%f')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, 123)', 'dtparse#32'
assert R(str(datetime.datetime.strptime('2024-05-06 07:08:09.000123', '%Y-%m-%d %H:%M:%S.%f'))) == "'2024-05-06 07:08:09.000123'", 'dtparse#33'
assert R(datetime.datetime.strptime('07:08 PM', '%I:%M %p')) == 'datetime.datetime(1900, 1, 1, 19, 8)', 'dtparse#34'
assert R(str(datetime.datetime.strptime('07:08 PM', '%I:%M %p'))) == "'1900-01-01 19:08:00'", 'dtparse#35'
assert R(datetime.datetime.strptime('07:08 am', '%I:%M %p')) == 'datetime.datetime(1900, 1, 1, 7, 8)', 'dtparse#36'
assert R(str(datetime.datetime.strptime('07:08 am', '%I:%M %p'))) == "'1900-01-01 07:08:00'", 'dtparse#37'
assert R(datetime.datetime.strptime('12:00 AM', '%I:%M %p')) == 'datetime.datetime(1900, 1, 1, 0, 0)', 'dtparse#38'
assert R(str(datetime.datetime.strptime('12:00 AM', '%I:%M %p'))) == "'1900-01-01 00:00:00'", 'dtparse#39'
assert R(datetime.datetime.strptime('12:30 PM', '%I:%M %p')) == 'datetime.datetime(1900, 1, 1, 12, 30)', 'dtparse#40'
assert R(str(datetime.datetime.strptime('12:30 PM', '%I:%M %p'))) == "'1900-01-01 12:30:00'", 'dtparse#41'
assert R(datetime.datetime.strptime('7 pm', '%I %p')) == 'datetime.datetime(1900, 1, 1, 19, 0)', 'dtparse#42'
assert R(str(datetime.datetime.strptime('7 pm', '%I %p'))) == "'1900-01-01 19:00:00'", 'dtparse#43'
assert R(datetime.datetime.strptime('19:08', '%H:%M')) == 'datetime.datetime(1900, 1, 1, 19, 8)', 'dtparse#44'
assert R(str(datetime.datetime.strptime('19:08', '%H:%M'))) == "'1900-01-01 19:08:00'", 'dtparse#45'
assert R(datetime.datetime.strptime('2024-05-06 +0200', '%Y-%m-%d %z')) == 'datetime.datetime(2024, 5, 6, 0, 0, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))', 'dtparse#46'
assert R(str(datetime.datetime.strptime('2024-05-06 +0200', '%Y-%m-%d %z'))) == "'2024-05-06 00:00:00+02:00'", 'dtparse#47'
assert R(datetime.datetime.strptime('2024-05-06 -0530', '%Y-%m-%d %z')) == 'datetime.datetime(2024, 5, 6, 0, 0, tzinfo=datetime.timezone(datetime.timedelta(days=-1, seconds=66600)))', 'dtparse#48'
assert R(str(datetime.datetime.strptime('2024-05-06 -0530', '%Y-%m-%d %z'))) == "'2024-05-06 00:00:00-05:30'", 'dtparse#49'
assert R(datetime.datetime.strptime('2024-05-06 +02:00', '%Y-%m-%d %z')) == 'datetime.datetime(2024, 5, 6, 0, 0, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))', 'dtparse#50'
assert R(str(datetime.datetime.strptime('2024-05-06 +02:00', '%Y-%m-%d %z'))) == "'2024-05-06 00:00:00+02:00'", 'dtparse#51'
assert R(datetime.datetime.strptime('2024-05-06Z', '%Y-%m-%d%z')) == 'datetime.datetime(2024, 5, 6, 0, 0, tzinfo=datetime.timezone.utc)', 'dtparse#52'
assert R(str(datetime.datetime.strptime('2024-05-06Z', '%Y-%m-%d%z'))) == "'2024-05-06 00:00:00+00:00'", 'dtparse#53'
assert R(datetime.datetime.strptime('2024-05-06 Z', '%Y-%m-%d %z')) == 'datetime.datetime(2024, 5, 6, 0, 0, tzinfo=datetime.timezone.utc)', 'dtparse#54'
assert R(str(datetime.datetime.strptime('2024-05-06 Z', '%Y-%m-%d %z'))) == "'2024-05-06 00:00:00+00:00'", 'dtparse#55'
assert R(datetime.datetime.strptime('2024-05-06 UTC', '%Y-%m-%d %Z')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#56'
assert R(str(datetime.datetime.strptime('2024-05-06 UTC', '%Y-%m-%d %Z'))) == "'2024-05-06 00:00:00'", 'dtparse#57'
assert R(datetime.datetime.strptime('2024-05-06 GMT', '%Y-%m-%d %Z')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#58'
assert R(str(datetime.datetime.strptime('2024-05-06 GMT', '%Y-%m-%d %Z'))) == "'2024-05-06 00:00:00'", 'dtparse#59'
assert R(datetime.datetime.strptime('2024-05-06T07:08:09+0000', '%Y-%m-%dT%H:%M:%S%z')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, tzinfo=datetime.timezone.utc)', 'dtparse#60'
assert R(str(datetime.datetime.strptime('2024-05-06T07:08:09+0000', '%Y-%m-%dT%H:%M:%S%z'))) == "'2024-05-06 07:08:09+00:00'", 'dtparse#61'
assert R(datetime.datetime.strptime('2024-05-06T07:08:09.5+01:00', '%Y-%m-%dT%H:%M:%S.%f%z')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, 500000, tzinfo=datetime.timezone(datetime.timedelta(seconds=3600)))', 'dtparse#62'
assert R(str(datetime.datetime.strptime('2024-05-06T07:08:09.5+01:00', '%Y-%m-%dT%H:%M:%S.%f%z'))) == "'2024-05-06 07:08:09.500000+01:00'", 'dtparse#63'
assert R(datetime.datetime.strptime('2024-05-06', '%Y-%m-%d')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#64'
assert R(str(datetime.datetime.strptime('2024-05-06', '%Y-%m-%d'))) == "'2024-05-06 00:00:00'", 'dtparse#65'
assert R(datetime.datetime.strptime('2024-5-6', '%Y-%m-%d')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#66'
assert R(str(datetime.datetime.strptime('2024-5-6', '%Y-%m-%d'))) == "'2024-05-06 00:00:00'", 'dtparse#67'
assert R(datetime.datetime.strptime('20240506', '%Y%m%d')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#68'
assert R(str(datetime.datetime.strptime('20240506', '%Y%m%d'))) == "'2024-05-06 00:00:00'", 'dtparse#69'
assert R(datetime.datetime.strptime('240506', '%y%m%d')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#70'
assert R(str(datetime.datetime.strptime('240506', '%y%m%d'))) == "'2024-05-06 00:00:00'", 'dtparse#71'
assert R(datetime.datetime.strptime('2024-05-06   07:08', '%Y-%m-%d %H:%M')) == 'datetime.datetime(2024, 5, 6, 7, 8)', 'dtparse#72'
assert R(str(datetime.datetime.strptime('2024-05-06   07:08', '%Y-%m-%d %H:%M'))) == "'2024-05-06 07:08:00'", 'dtparse#73'
assert R(datetime.datetime.strptime('2024-05-06\t07:08', '%Y-%m-%d %H:%M')) == 'datetime.datetime(2024, 5, 6, 7, 8)', 'dtparse#74'
assert R(str(datetime.datetime.strptime('2024-05-06\t07:08', '%Y-%m-%d %H:%M'))) == "'2024-05-06 07:08:00'", 'dtparse#75'
assert R(datetime.datetime.strptime('2024-05-06 07:08', '%Y-%m-%d  %H:%M')) == 'datetime.datetime(2024, 5, 6, 7, 8)', 'dtparse#76'
assert R(str(datetime.datetime.strptime('2024-05-06 07:08', '%Y-%m-%d  %H:%M'))) == "'2024-05-06 07:08:00'", 'dtparse#77'
try:
    R(datetime.datetime.strptime(' 2024-05-06', '%Y-%m-%d'))
    raise AssertionError('dtparse#78')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime(' 2024-05-06', '%Y-%m-%d')))
    raise AssertionError('dtparse#79')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('2024-05-06 ', '%Y-%m-%d'))
    raise AssertionError('dtparse#80')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-05-06 ', '%Y-%m-%d')))
    raise AssertionError('dtparse#81')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('2024-05-06x', '%Y-%m-%d'))
    raise AssertionError('dtparse#82')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-05-06x', '%Y-%m-%d')))
    raise AssertionError('dtparse#83')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('2024-05-06 07', '%Y-%m-%d'))
    raise AssertionError('dtparse#84')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-05-06 07', '%Y-%m-%d')))
    raise AssertionError('dtparse#85')
except ValueError:
    pass
assert R(datetime.datetime.strptime('2024', '%Y')) == 'datetime.datetime(2024, 1, 1, 0, 0)', 'dtparse#86'
assert R(str(datetime.datetime.strptime('2024', '%Y'))) == "'2024-01-01 00:00:00'", 'dtparse#87'
assert R(datetime.datetime.strptime('5', '%m')) == 'datetime.datetime(1900, 5, 1, 0, 0)', 'dtparse#88'
assert R(str(datetime.datetime.strptime('5', '%m'))) == "'1900-05-01 00:00:00'", 'dtparse#89'
assert R(datetime.datetime.strptime('1:2:3', '%H:%M:%S')) == 'datetime.datetime(1900, 1, 1, 1, 2, 3)', 'dtparse#92'
assert R(str(datetime.datetime.strptime('1:2:3', '%H:%M:%S'))) == "'1900-01-01 01:02:03'", 'dtparse#93'
try:
    R(datetime.datetime.strptime('23:59:60', '%H:%M:%S'))
    raise AssertionError('dtparse#94')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('23:59:60', '%H:%M:%S')))
    raise AssertionError('dtparse#95')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('100%', '%d%%'))
    raise AssertionError('dtparse#96')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('100%', '%d%%')))
    raise AssertionError('dtparse#97')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('2024-13-01', '%Y-%m-%d'))
    raise AssertionError('dtparse#98')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-13-01', '%Y-%m-%d')))
    raise AssertionError('dtparse#99')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('2024-02-30', '%Y-%m-%d'))
    raise AssertionError('dtparse#100')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-02-30', '%Y-%m-%d')))
    raise AssertionError('dtparse#101')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('2023-02-29', '%Y-%m-%d'))
    raise AssertionError('dtparse#102')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2023-02-29', '%Y-%m-%d')))
    raise AssertionError('dtparse#103')
except ValueError:
    pass
assert R(datetime.datetime.strptime('2024-02-29', '%Y-%m-%d')) == 'datetime.datetime(2024, 2, 29, 0, 0)', 'dtparse#104'
assert R(str(datetime.datetime.strptime('2024-02-29', '%Y-%m-%d'))) == "'2024-02-29 00:00:00'", 'dtparse#105'
try:
    R(datetime.datetime.strptime('abc', '%Y'))
    raise AssertionError('dtparse#106')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('abc', '%Y')))
    raise AssertionError('dtparse#107')
except ValueError:
    pass
assert R(datetime.datetime.strptime('', '')) == 'datetime.datetime(1900, 1, 1, 0, 0)', 'dtparse#108'
assert R(str(datetime.datetime.strptime('', ''))) == "'1900-01-01 00:00:00'", 'dtparse#109'
assert R(datetime.datetime.strptime('x', 'x')) == 'datetime.datetime(1900, 1, 1, 0, 0)', 'dtparse#110'
assert R(str(datetime.datetime.strptime('x', 'x'))) == "'1900-01-01 00:00:00'", 'dtparse#111'
try:
    R(datetime.datetime.strptime('2024-05-06', '%Y-%m-%d %H'))
    raise AssertionError('dtparse#112')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-05-06', '%Y-%m-%d %H')))
    raise AssertionError('dtparse#113')
except ValueError:
    pass
assert R(datetime.datetime.strptime('12', '%I')) == 'datetime.datetime(1900, 1, 1, 0, 0)', 'dtparse#114'
assert R(str(datetime.datetime.strptime('12', '%I'))) == "'1900-01-01 00:00:00'", 'dtparse#115'
try:
    R(datetime.datetime.strptime('0', '%I'))
    raise AssertionError('dtparse#116')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('0', '%I')))
    raise AssertionError('dtparse#117')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('13', '%I'))
    raise AssertionError('dtparse#118')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('13', '%I')))
    raise AssertionError('dtparse#119')
except ValueError:
    pass
assert R(datetime.datetime.strptime('2024 19 1', '%G %V %u')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#120'
assert R(str(datetime.datetime.strptime('2024 19 1', '%G %V %u'))) == "'2024-05-06 00:00:00'", 'dtparse#121'
assert R(datetime.datetime.strptime('2024 18 1', '%Y %U %w')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#122'
assert R(str(datetime.datetime.strptime('2024 18 1', '%Y %U %w'))) == "'2024-05-06 00:00:00'", 'dtparse#123'
assert R(datetime.datetime.strptime('2024 19 1', '%Y %W %w')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#124'
assert R(str(datetime.datetime.strptime('2024 19 1', '%Y %W %w'))) == "'2024-05-06 00:00:00'", 'dtparse#125'
assert R(datetime.datetime.strptime('Tue, 07 May 2024 10:00:00 GMT', '%a, %d %b %Y %H:%M:%S %Z')) == 'datetime.datetime(2024, 5, 7, 10, 0)', 'dtparse#126'
assert R(str(datetime.datetime.strptime('Tue, 07 May 2024 10:00:00 GMT', '%a, %d %b %Y %H:%M:%S %Z'))) == "'2024-05-07 10:00:00'", 'dtparse#127'
assert R(datetime.datetime.strptime('Tue, 07 May 2024 10:00:00 +0000', '%a, %d %b %Y %H:%M:%S %z')) == 'datetime.datetime(2024, 5, 7, 10, 0, tzinfo=datetime.timezone.utc)', 'dtparse#128'
assert R(str(datetime.datetime.strptime('Tue, 07 May 2024 10:00:00 +0000', '%a, %d %b %Y %H:%M:%S %z'))) == "'2024-05-07 10:00:00+00:00'", 'dtparse#129'
assert R(datetime.datetime.strptime('06.05.2024 07:08', '%d.%m.%Y %H:%M')) == 'datetime.datetime(2024, 5, 6, 7, 8)', 'dtparse#130'
assert R(str(datetime.datetime.strptime('06.05.2024 07:08', '%d.%m.%Y %H:%M'))) == "'2024-05-06 07:08:00'", 'dtparse#131'
assert R(datetime.datetime.strptime('6/5/2024', '%d/%m/%Y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#132'
assert R(str(datetime.datetime.strptime('6/5/2024', '%d/%m/%Y'))) == "'2024-05-06 00:00:00'", 'dtparse#133'
assert R(datetime.datetime.strptime('2024-05-06T07:08:09', '%Y-%m-%dT%H:%M:%S')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9)', 'dtparse#134'
assert R(str(datetime.datetime.strptime('2024-05-06T07:08:09', '%Y-%m-%dT%H:%M:%S'))) == "'2024-05-06 07:08:09'", 'dtparse#135'
assert R(datetime.datetime.strptime('07:08:09', '%H:%M:%S')) == 'datetime.datetime(1900, 1, 1, 7, 8, 9)', 'dtparse#136'
assert R(str(datetime.datetime.strptime('07:08:09', '%H:%M:%S'))) == "'1900-01-01 07:08:09'", 'dtparse#137'
try:
    R(datetime.datetime.strptime('1/1/1', '%d/%m/%Y'))
    raise AssertionError('dtparse#138')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('1/1/1', '%d/%m/%Y')))
    raise AssertionError('dtparse#139')
except ValueError:
    pass
assert R(datetime.datetime.strptime('0001-01-01', '%Y-%m-%d')) == 'datetime.datetime(1, 1, 1, 0, 0)', 'dtparse#140'
assert R(str(datetime.datetime.strptime('0001-01-01', '%Y-%m-%d'))) == "'0001-01-01 00:00:00'", 'dtparse#141'
assert R(datetime.datetime.strptime('9999-12-31 23:59:59', '%Y-%m-%d %H:%M:%S')) == 'datetime.datetime(9999, 12, 31, 23, 59, 59)', 'dtparse#142'
assert R(str(datetime.datetime.strptime('9999-12-31 23:59:59', '%Y-%m-%d %H:%M:%S'))) == "'9999-12-31 23:59:59'", 'dtparse#143'
assert R(datetime.datetime.strptime('Mon', '%a')) == 'datetime.datetime(1900, 1, 1, 0, 0)', 'dtparse#144'
assert R(str(datetime.datetime.strptime('Mon', '%a'))) == "'1900-01-01 00:00:00'", 'dtparse#145'
assert R(datetime.datetime.strptime('Thursday', '%A')) == 'datetime.datetime(1900, 1, 1, 0, 0)', 'dtparse#146'
assert R(str(datetime.datetime.strptime('Thursday', '%A'))) == "'1900-01-01 00:00:00'", 'dtparse#147'
try:
    R(datetime.datetime.strptime('2024-05-06 7', '%Y-%m-%d %w'))
    raise AssertionError('dtparse#148')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-05-06 7', '%Y-%m-%d %w')))
    raise AssertionError('dtparse#149')
except ValueError:
    pass
assert R(datetime.datetime.strptime('12/31/99 11:59:59 PM', '%m/%d/%y %I:%M:%S %p')) == 'datetime.datetime(1999, 12, 31, 23, 59, 59)', 'dtparse#150'
assert R(str(datetime.datetime.strptime('12/31/99 11:59:59 PM', '%m/%d/%y %I:%M:%S %p'))) == "'1999-12-31 23:59:59'", 'dtparse#151'
try:
    R(datetime.datetime.strptime('2024-05-06 07:08:09.1234567', '%Y-%m-%d %H:%M:%S.%f'))
    raise AssertionError('dtparse#152')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-05-06 07:08:09.1234567', '%Y-%m-%d %H:%M:%S.%f')))
    raise AssertionError('dtparse#153')
except ValueError:
    pass
try:
    R(datetime.datetime.strptime('2024-05-06 +0200x', '%Y-%m-%d %z'))
    raise AssertionError('dtparse#154')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('2024-05-06 +0200x', '%Y-%m-%d %z')))
    raise AssertionError('dtparse#155')
except ValueError:
    pass
assert R(datetime.datetime.strptime('2024-W19-1', '%G-W%V-%u')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#156'
assert R(str(datetime.datetime.strptime('2024-W19-1', '%G-W%V-%u'))) == "'2024-05-06 00:00:00'", 'dtparse#157'
assert R(datetime.datetime.strptime('May  6 2024', '%b %d %Y')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtparse#158'
assert R(str(datetime.datetime.strptime('May  6 2024', '%b %d %Y'))) == "'2024-05-06 00:00:00'", 'dtparse#159'
try:
    R(datetime.datetime.strptime('29 Feb', '%d %b'))
    raise AssertionError('dtparse#162')
except ValueError:
    pass
try:
    R(str(datetime.datetime.strptime('29 Feb', '%d %b')))
    raise AssertionError('dtparse#163')
except ValueError:
    pass
assert R(tuple(time.strptime('06/05/24', '%d/%m/%y'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#164'
assert R(tuple(time.strptime('06/05/69', '%d/%m/%y'))[:8]) == '(1969, 5, 6, 0, 0, 0, 1, 126)', 'dtparse#165'
assert R(tuple(time.strptime('06/05/68', '%d/%m/%y'))[:8]) == '(2068, 5, 6, 0, 0, 0, 6, 127)', 'dtparse#166'
assert R(tuple(time.strptime('06/05/00', '%d/%m/%y'))[:8]) == '(2000, 5, 6, 0, 0, 0, 5, 127)', 'dtparse#167'
assert R(tuple(time.strptime('May 6 2024', '%b %d %Y'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#168'
assert R(tuple(time.strptime('may 06 2024', '%b %d %Y'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#169'
assert R(tuple(time.strptime('MAY 6 2024', '%B %d %Y'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#170'
assert R(tuple(time.strptime('May 6 2024', '%B %d %Y'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#171'
assert R(tuple(time.strptime('September 30 2024', '%B %d %Y'))[:8]) == '(2024, 9, 30, 0, 0, 0, 0, 274)', 'dtparse#172'
assert R(tuple(time.strptime('Sep 30 2024', '%b %d %Y'))[:8]) == '(2024, 9, 30, 0, 0, 0, 0, 274)', 'dtparse#173'
assert R(tuple(time.strptime('Mon May 06 2024', '%a %b %d %Y'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#174'
assert R(tuple(time.strptime('monday, 6 may 2024', '%A, %d %B %Y'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#175'
assert R(tuple(time.strptime('2024-127', '%Y-%j'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#176'
assert R(tuple(time.strptime('2024-1', '%Y-%j'))[:8]) == '(2024, 1, 1, 0, 0, 0, 0, 1)', 'dtparse#177'
assert R(tuple(time.strptime('2024-05-06 07:08:09.123456', '%Y-%m-%d %H:%M:%S.%f'))[:8]) == '(2024, 5, 6, 7, 8, 9, 0, 127)', 'dtparse#178'
assert R(tuple(time.strptime('2024-05-06 07:08:09.1', '%Y-%m-%d %H:%M:%S.%f'))[:8]) == '(2024, 5, 6, 7, 8, 9, 0, 127)', 'dtparse#179'
assert R(tuple(time.strptime('2024-05-06 07:08:09.000123', '%Y-%m-%d %H:%M:%S.%f'))[:8]) == '(2024, 5, 6, 7, 8, 9, 0, 127)', 'dtparse#180'
assert R(tuple(time.strptime('07:08 PM', '%I:%M %p'))[:8]) == '(1900, 1, 1, 19, 8, 0, 0, 1)', 'dtparse#181'
assert R(tuple(time.strptime('07:08 am', '%I:%M %p'))[:8]) == '(1900, 1, 1, 7, 8, 0, 0, 1)', 'dtparse#182'
assert R(tuple(time.strptime('12:00 AM', '%I:%M %p'))[:8]) == '(1900, 1, 1, 0, 0, 0, 0, 1)', 'dtparse#183'
assert R(tuple(time.strptime('12:30 PM', '%I:%M %p'))[:8]) == '(1900, 1, 1, 12, 30, 0, 0, 1)', 'dtparse#184'
assert R(tuple(time.strptime('7 pm', '%I %p'))[:8]) == '(1900, 1, 1, 19, 0, 0, 0, 1)', 'dtparse#185'
assert R(tuple(time.strptime('19:08', '%H:%M'))[:8]) == '(1900, 1, 1, 19, 8, 0, 0, 1)', 'dtparse#186'
assert R(tuple(time.strptime('2024-05-06 +0200', '%Y-%m-%d %z'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#187'
assert R(tuple(time.strptime('2024-05-06 -0530', '%Y-%m-%d %z'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#188'
assert R(tuple(time.strptime('2024-05-06 +02:00', '%Y-%m-%d %z'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#189'
assert R(tuple(time.strptime('2024-05-06Z', '%Y-%m-%d%z'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#190'
assert R(tuple(time.strptime('2024-05-06 Z', '%Y-%m-%d %z'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#191'
assert R(tuple(time.strptime('2024-05-06 UTC', '%Y-%m-%d %Z'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#192'
assert R(tuple(time.strptime('2024-05-06 GMT', '%Y-%m-%d %Z'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#193'
assert R(tuple(time.strptime('2024-05-06T07:08:09+0000', '%Y-%m-%dT%H:%M:%S%z'))[:8]) == '(2024, 5, 6, 7, 8, 9, 0, 127)', 'dtparse#194'
assert R(tuple(time.strptime('2024-05-06T07:08:09.5+01:00', '%Y-%m-%dT%H:%M:%S.%f%z'))[:8]) == '(2024, 5, 6, 7, 8, 9, 0, 127)', 'dtparse#195'
assert R(tuple(time.strptime('2024-05-06', '%Y-%m-%d'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#196'
assert R(tuple(time.strptime('2024-5-6', '%Y-%m-%d'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#197'
assert R(tuple(time.strptime('20240506', '%Y%m%d'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#198'
assert R(tuple(time.strptime('240506', '%y%m%d'))[:8]) == '(2024, 5, 6, 0, 0, 0, 0, 127)', 'dtparse#199'
assert R(tuple(time.strptime('2024-05-06   07:08', '%Y-%m-%d %H:%M'))[:8]) == '(2024, 5, 6, 7, 8, 0, 0, 127)', 'dtparse#200'
assert R(tuple(time.strptime('2024-05-06\t07:08', '%Y-%m-%d %H:%M'))[:8]) == '(2024, 5, 6, 7, 8, 0, 0, 127)', 'dtparse#201'
assert R(tuple(time.strptime('2024-05-06 07:08', '%Y-%m-%d  %H:%M'))[:8]) == '(2024, 5, 6, 7, 8, 0, 0, 127)', 'dtparse#202'
try:
    R(tuple(time.strptime(' 2024-05-06', '%Y-%m-%d'))[:8])
    raise AssertionError('dtparse#203')
except ValueError:
    pass
try:
    R(tuple(time.strptime('2024-05-06 ', '%Y-%m-%d'))[:8])
    raise AssertionError('dtparse#204')
except ValueError:
    pass
try:
    R(tuple(time.strptime('2024-05-06x', '%Y-%m-%d'))[:8])
    raise AssertionError('dtparse#205')
except ValueError:
    pass
try:
    R(tuple(time.strptime('2024-05-06 07', '%Y-%m-%d'))[:8])
    raise AssertionError('dtparse#206')
except ValueError:
    pass
assert R(tuple(time.strptime('2024', '%Y'))[:8]) == '(2024, 1, 1, 0, 0, 0, 0, 1)', 'dtparse#207'
assert R(tuple(time.strptime('5', '%m'))[:8]) == '(1900, 5, 1, 0, 0, 0, 1, 121)', 'dtparse#208'
assert R(tuple(time.strptime('1:2:3', '%H:%M:%S'))[:8]) == '(1900, 1, 1, 1, 2, 3, 0, 1)', 'dtparse#210'
assert R(tuple(time.strptime('23:59:60', '%H:%M:%S'))[:8]) == '(1900, 1, 1, 23, 59, 60, 0, 1)', 'dtparse#211'
try:
    R(tuple(time.strptime('100%', '%d%%'))[:8])
    raise AssertionError('dtparse#212')
except ValueError:
    pass
try:
    R(tuple(time.strptime('2024-13-01', '%Y-%m-%d'))[:8])
    raise AssertionError('dtparse#213')
except ValueError:
    pass
try:
    R(tuple(time.strptime('2024-02-30', '%Y-%m-%d'))[:8])
    raise AssertionError('dtparse#214')
except ValueError:
    pass
try:
    R(tuple(time.strptime('2023-02-29', '%Y-%m-%d'))[:8])
    raise AssertionError('dtparse#215')
except ValueError:
    pass
assert R(tuple(time.strptime('2024-02-29', '%Y-%m-%d'))[:8]) == '(2024, 2, 29, 0, 0, 0, 3, 60)', 'dtparse#216'
try:
    R(tuple(time.strptime('abc', '%Y'))[:8])
    raise AssertionError('dtparse#217')
except ValueError:
    pass
assert R(tuple(time.strptime('', ''))[:8]) == '(1900, 1, 1, 0, 0, 0, 0, 1)', 'dtparse#218'
assert R(tuple(time.strptime('x', 'x'))[:8]) == '(1900, 1, 1, 0, 0, 0, 0, 1)', 'dtparse#219'
try:
    R(tuple(time.strptime('2024-05-06', '%Y-%m-%d %H'))[:8])
    raise AssertionError('dtparse#220')
except ValueError:
    pass
assert R(tuple(time.strptime('12', '%I'))[:8]) == '(1900, 1, 1, 0, 0, 0, 0, 1)', 'dtparse#221'
try:
    R(tuple(time.strptime('0', '%I'))[:8])
    raise AssertionError('dtparse#222')
except ValueError:
    pass
try:
    R(tuple(time.strptime('13', '%I'))[:8])
    raise AssertionError('dtparse#223')
except ValueError:
    pass
assert R(time.strptime('2024-05-06', '%Y-%m-%d').tm_yday) == '127', 'dtparse#224'
assert R(time.strptime('2024-05-06', '%Y-%m-%d').tm_wday) == '0', 'dtparse#225'
assert R(time.strptime('07:08', '%H:%M').tm_year) == '1900', 'dtparse#226'
assert R(datetime.datetime.strptime('2024-05-06 +0200', '%Y-%m-%d %z').utcoffset()) == 'datetime.timedelta(seconds=7200)', 'dtparse#227'
assert R(datetime.datetime.strptime('2024-05-06 +0200', '%Y-%m-%d %z').tzinfo) == 'datetime.timezone(datetime.timedelta(seconds=7200))', 'dtparse#228'
assert R(datetime.datetime.strptime('2024-05-06 UTC', '%Y-%m-%d %Z').tzinfo) == 'None', 'dtparse#229'
assert R(datetime.datetime.strptime('2024-05-06', '%Y-%m-%d').tzinfo) == 'None', 'dtparse#230'
assert R(datetime.datetime.strptime('2024-05-06', '%Y-%m-%d').timestamp() == datetime.datetime(2024,5,6).timestamp()) == 'True', 'dtparse#231'
assert R(datetime.datetime.strptime('2024-05-06', '%Y-%m-%d') == datetime.datetime(2024,5,6)) == 'True', 'dtparse#232'
assert R(datetime.datetime.strptime('2024-05-06', '%Y-%m-%d').date()) == 'datetime.date(2024, 5, 6)', 'dtparse#233'
assert R(datetime.datetime.strptime('2024-05-06 +0200', '%Y-%m-%d %z').isoformat()) == "'2024-05-06T00:00:00+02:00'", 'dtparse#234'
assert R(datetime.datetime.strptime('2024-05-06 +0200', '%Y-%m-%d %z').strftime('%z %Z')) == "'+0200 UTC+02:00'", 'dtparse#235'
assert R(datetime.datetime.strptime('2024-05-06 UTC', '%Y-%m-%d %Z').strftime('%z %Z')) == "' '", 'dtparse#236'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456)) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, 123456)', 'dtobj#0'
assert R([datetime.datetime(2024, 5, 6, 7, 8, 9, 123456)]) == '[datetime.datetime(2024, 5, 6, 7, 8, 9, 123456)]', 'dtobj#2'
assert R(f"{datetime.datetime(2024, 5, 6, 7, 8, 9, 123456)!r}") == "'datetime.datetime(2024, 5, 6, 7, 8, 9, 123456)'", 'dtobj#4'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0)) == 'datetime.datetime(2023, 1, 1, 0, 0)', 'dtobj#5'
assert R([datetime.datetime(2023, 1, 1, 0, 0, 0)]) == '[datetime.datetime(2023, 1, 1, 0, 0)]', 'dtobj#7'
assert R(f"{datetime.datetime(2023, 1, 1, 0, 0, 0)!r}") == "'datetime.datetime(2023, 1, 1, 0, 0)'", 'dtobj#9'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59)) == 'datetime.datetime(2024, 12, 31, 23, 59, 59)', 'dtobj#10'
assert R([datetime.datetime(2024, 12, 31, 23, 59, 59)]) == '[datetime.datetime(2024, 12, 31, 23, 59, 59)]', 'dtobj#12'
assert R(f"{datetime.datetime(2024, 12, 31, 23, 59, 59)!r}") == "'datetime.datetime(2024, 12, 31, 23, 59, 59)'", 'dtobj#14'
assert R(datetime.datetime(2021, 1, 3, 12, 30)) == 'datetime.datetime(2021, 1, 3, 12, 30)', 'dtobj#15'
assert R([datetime.datetime(2021, 1, 3, 12, 30)]) == '[datetime.datetime(2021, 1, 3, 12, 30)]', 'dtobj#17'
assert R(f"{datetime.datetime(2021, 1, 3, 12, 30)!r}") == "'datetime.datetime(2021, 1, 3, 12, 30)'", 'dtobj#19'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7)) == 'datetime.datetime(1999, 2, 28, 13, 5, 1, 7)', 'dtobj#20'
assert R([datetime.datetime(1999, 2, 28, 13, 5, 1, 7)]) == '[datetime.datetime(1999, 2, 28, 13, 5, 1, 7)]', 'dtobj#22'
assert R(f"{datetime.datetime(1999, 2, 28, 13, 5, 1, 7)!r}") == "'datetime.datetime(1999, 2, 28, 13, 5, 1, 7)'", 'dtobj#24'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999)) == 'datetime.datetime(2020, 2, 29, 0, 0, 0, 999999)', 'dtobj#25'
assert R([datetime.datetime(2020, 2, 29, 0, 0, 0, 999999)]) == '[datetime.datetime(2020, 2, 29, 0, 0, 0, 999999)]', 'dtobj#27'
assert R(f"{datetime.datetime(2020, 2, 29, 0, 0, 0, 999999)!r}") == "'datetime.datetime(2020, 2, 29, 0, 0, 0, 999999)'", 'dtobj#29'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2)))) == 'datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))', 'dtobj#30'
assert R([datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2)))]) == '[datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))]', 'dtobj#32'
assert R(f"{datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2)))!r}") == "'datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))'", 'dtobj#34'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc)) == 'datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc)', 'dtobj#35'
assert R([datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc)]) == '[datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc)]', 'dtobj#37'
assert R(f"{datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc)!r}") == "'datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc)'", 'dtobj#39'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST'))) == "datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(days=-1, seconds=66600), 'EST'))", 'dtobj#40'
assert R([datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST'))]) == "[datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(days=-1, seconds=66600), 'EST'))]", 'dtobj#42'
assert R(f"{datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST'))!r}") == '"datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(days=-1, seconds=66600), \'EST\'))"', 'dtobj#44'
assert R(datetime.date(2024, 5, 6)) == 'datetime.date(2024, 5, 6)', 'dtobj#45'
assert R([datetime.date(2024, 5, 6)]) == '[datetime.date(2024, 5, 6)]', 'dtobj#47'
assert R(f"{datetime.date(2024, 5, 6)!r}") == "'datetime.date(2024, 5, 6)'", 'dtobj#49'
assert R(datetime.date(2023, 1, 1)) == 'datetime.date(2023, 1, 1)', 'dtobj#50'
assert R([datetime.date(2023, 1, 1)]) == '[datetime.date(2023, 1, 1)]', 'dtobj#52'
assert R(f"{datetime.date(2023, 1, 1)!r}") == "'datetime.date(2023, 1, 1)'", 'dtobj#54'
assert R(datetime.date(2020, 12, 31)) == 'datetime.date(2020, 12, 31)', 'dtobj#55'
assert R([datetime.date(2020, 12, 31)]) == '[datetime.date(2020, 12, 31)]', 'dtobj#57'
assert R(f"{datetime.date(2020, 12, 31)!r}") == "'datetime.date(2020, 12, 31)'", 'dtobj#59'
assert R(datetime.date(1, 1, 1)) == 'datetime.date(1, 1, 1)', 'dtobj#60'
assert R([datetime.date(1, 1, 1)]) == '[datetime.date(1, 1, 1)]', 'dtobj#62'
assert R(f"{datetime.date(1, 1, 1)!r}") == "'datetime.date(1, 1, 1)'", 'dtobj#64'
assert R(datetime.timedelta(days=1, seconds=3661)) == 'datetime.timedelta(days=1, seconds=3661)', 'dtobj#65'
assert R([datetime.timedelta(days=1, seconds=3661)]) == '[datetime.timedelta(days=1, seconds=3661)]', 'dtobj#67'
assert R(f"{datetime.timedelta(days=1, seconds=3661)!r}") == "'datetime.timedelta(days=1, seconds=3661)'", 'dtobj#69'
assert R(datetime.timedelta(0)) == 'datetime.timedelta(0)', 'dtobj#70'
assert R([datetime.timedelta(0)]) == '[datetime.timedelta(0)]', 'dtobj#72'
assert R(f"{datetime.timedelta(0)!r}") == "'datetime.timedelta(0)'", 'dtobj#74'
assert R(datetime.timedelta(seconds=-1)) == 'datetime.timedelta(days=-1, seconds=86399)', 'dtobj#75'
assert R([datetime.timedelta(seconds=-1)]) == '[datetime.timedelta(days=-1, seconds=86399)]', 'dtobj#77'
assert R(f"{datetime.timedelta(seconds=-1)!r}") == "'datetime.timedelta(days=-1, seconds=86399)'", 'dtobj#79'
assert R(datetime.timedelta(microseconds=5)) == 'datetime.timedelta(microseconds=5)', 'dtobj#80'
assert R([datetime.timedelta(microseconds=5)]) == '[datetime.timedelta(microseconds=5)]', 'dtobj#82'
assert R(f"{datetime.timedelta(microseconds=5)!r}") == "'datetime.timedelta(microseconds=5)'", 'dtobj#84'
assert R(datetime.timedelta(weeks=2, hours=3)) == 'datetime.timedelta(days=14, seconds=10800)', 'dtobj#85'
assert R([datetime.timedelta(weeks=2, hours=3)]) == '[datetime.timedelta(days=14, seconds=10800)]', 'dtobj#87'
assert R(f"{datetime.timedelta(weeks=2, hours=3)!r}") == "'datetime.timedelta(days=14, seconds=10800)'", 'dtobj#89'
assert R(datetime.timedelta(days=-3, microseconds=1)) == 'datetime.timedelta(days=-3, microseconds=1)', 'dtobj#90'
assert R([datetime.timedelta(days=-3, microseconds=1)]) == '[datetime.timedelta(days=-3, microseconds=1)]', 'dtobj#92'
assert R(f"{datetime.timedelta(days=-3, microseconds=1)!r}") == "'datetime.timedelta(days=-3, microseconds=1)'", 'dtobj#94'
assert R(datetime.timedelta(milliseconds=1500)) == 'datetime.timedelta(seconds=1, microseconds=500000)', 'dtobj#95'
assert R([datetime.timedelta(milliseconds=1500)]) == '[datetime.timedelta(seconds=1, microseconds=500000)]', 'dtobj#97'
assert R(f"{datetime.timedelta(milliseconds=1500)!r}") == "'datetime.timedelta(seconds=1, microseconds=500000)'", 'dtobj#99'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456).date()) == 'datetime.date(2024, 5, 6)', 'dtobj#112'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456).time()) == 'datetime.time(7, 8, 9, 123456)', 'dtobj#113'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456).replace(year=2000)) == 'datetime.datetime(2000, 5, 6, 7, 8, 9, 123456)', 'dtobj#119'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456).replace(microsecond=0)) == 'datetime.datetime(2024, 5, 6, 7, 8, 9)', 'dtobj#120'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456).dst()) == 'None', 'dtobj#132'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456).timetz()) == 'datetime.time(7, 8, 9, 123456)', 'dtobj#133'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456).isoformat(timespec='seconds')) == "'2024-05-06T07:08:09'", 'dtobj#134'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, 123456).isoformat(timespec='milliseconds')) == "'2024-05-06T07:08:09.123'", 'dtobj#135'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0).date()) == 'datetime.date(2023, 1, 1)', 'dtobj#140'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0).time()) == 'datetime.time(0, 0)', 'dtobj#141'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0).replace(year=2000)) == 'datetime.datetime(2000, 1, 1, 0, 0)', 'dtobj#147'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0).replace(microsecond=0)) == 'datetime.datetime(2023, 1, 1, 0, 0)', 'dtobj#148'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0).dst()) == 'None', 'dtobj#160'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0).timetz()) == 'datetime.time(0, 0)', 'dtobj#161'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0).isoformat(timespec='seconds')) == "'2023-01-01T00:00:00'", 'dtobj#162'
assert R(datetime.datetime(2023, 1, 1, 0, 0, 0).isoformat(timespec='milliseconds')) == "'2023-01-01T00:00:00.000'", 'dtobj#163'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59).date()) == 'datetime.date(2024, 12, 31)', 'dtobj#168'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59).time()) == 'datetime.time(23, 59, 59)', 'dtobj#169'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59).replace(year=2000)) == 'datetime.datetime(2000, 12, 31, 23, 59, 59)', 'dtobj#175'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59).replace(microsecond=0)) == 'datetime.datetime(2024, 12, 31, 23, 59, 59)', 'dtobj#176'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59).dst()) == 'None', 'dtobj#188'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59).timetz()) == 'datetime.time(23, 59, 59)', 'dtobj#189'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59).isoformat(timespec='seconds')) == "'2024-12-31T23:59:59'", 'dtobj#190'
assert R(datetime.datetime(2024, 12, 31, 23, 59, 59).isoformat(timespec='milliseconds')) == "'2024-12-31T23:59:59.000'", 'dtobj#191'
assert R(datetime.datetime(2021, 1, 3, 12, 30).date()) == 'datetime.date(2021, 1, 3)', 'dtobj#196'
assert R(datetime.datetime(2021, 1, 3, 12, 30).time()) == 'datetime.time(12, 30)', 'dtobj#197'
assert R(datetime.datetime(2021, 1, 3, 12, 30).replace(year=2000)) == 'datetime.datetime(2000, 1, 3, 12, 30)', 'dtobj#203'
assert R(datetime.datetime(2021, 1, 3, 12, 30).replace(microsecond=0)) == 'datetime.datetime(2021, 1, 3, 12, 30)', 'dtobj#204'
assert R(datetime.datetime(2021, 1, 3, 12, 30).dst()) == 'None', 'dtobj#216'
assert R(datetime.datetime(2021, 1, 3, 12, 30).timetz()) == 'datetime.time(12, 30)', 'dtobj#217'
assert R(datetime.datetime(2021, 1, 3, 12, 30).isoformat(timespec='seconds')) == "'2021-01-03T12:30:00'", 'dtobj#218'
assert R(datetime.datetime(2021, 1, 3, 12, 30).isoformat(timespec='milliseconds')) == "'2021-01-03T12:30:00.000'", 'dtobj#219'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7).date()) == 'datetime.date(1999, 2, 28)', 'dtobj#224'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7).time()) == 'datetime.time(13, 5, 1, 7)', 'dtobj#225'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7).replace(year=2000)) == 'datetime.datetime(2000, 2, 28, 13, 5, 1, 7)', 'dtobj#231'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7).replace(microsecond=0)) == 'datetime.datetime(1999, 2, 28, 13, 5, 1)', 'dtobj#232'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7).dst()) == 'None', 'dtobj#244'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7).timetz()) == 'datetime.time(13, 5, 1, 7)', 'dtobj#245'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7).isoformat(timespec='seconds')) == "'1999-02-28T13:05:01'", 'dtobj#246'
assert R(datetime.datetime(1999, 2, 28, 13, 5, 1, 7).isoformat(timespec='milliseconds')) == "'1999-02-28T13:05:01.000'", 'dtobj#247'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999).date()) == 'datetime.date(2020, 2, 29)', 'dtobj#252'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999).time()) == 'datetime.time(0, 0, 0, 999999)', 'dtobj#253'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999).replace(year=2000)) == 'datetime.datetime(2000, 2, 29, 0, 0, 0, 999999)', 'dtobj#259'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999).replace(microsecond=0)) == 'datetime.datetime(2020, 2, 29, 0, 0)', 'dtobj#260'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999).dst()) == 'None', 'dtobj#272'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999).timetz()) == 'datetime.time(0, 0, 0, 999999)', 'dtobj#273'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999).isoformat(timespec='seconds')) == "'2020-02-29T00:00:00'", 'dtobj#274'
assert R(datetime.datetime(2020, 2, 29, 0, 0, 0, 999999).isoformat(timespec='milliseconds')) == "'2020-02-29T00:00:00.999'", 'dtobj#275'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).date()) == 'datetime.date(2024, 1, 1)', 'dtobj#280'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).time()) == 'datetime.time(12, 0)', 'dtobj#281'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).replace(year=2000)) == 'datetime.datetime(2000, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))', 'dtobj#287'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).replace(microsecond=0)) == 'datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))', 'dtobj#288'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).utcoffset()) == 'datetime.timedelta(seconds=7200)', 'dtobj#298'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).dst()) == 'None', 'dtobj#300'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).timetz()) == 'datetime.time(12, 0, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))', 'dtobj#301'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).isoformat(timespec='seconds')) == "'2024-01-01T12:00:00+02:00'", 'dtobj#302'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2))).isoformat(timespec='milliseconds')) == "'2024-01-01T12:00:00.000+02:00'", 'dtobj#303'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).date()) == 'datetime.date(2024, 1, 1)', 'dtobj#308'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).time()) == 'datetime.time(12, 0)', 'dtobj#309'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).replace(year=2000)) == 'datetime.datetime(2000, 1, 1, 12, 0, tzinfo=datetime.timezone.utc)', 'dtobj#315'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).replace(microsecond=0)) == 'datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc)', 'dtobj#316'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).utcoffset()) == 'datetime.timedelta(0)', 'dtobj#326'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).dst()) == 'None', 'dtobj#328'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).timetz()) == 'datetime.time(12, 0, tzinfo=datetime.timezone.utc)', 'dtobj#329'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).isoformat(timespec='seconds')) == "'2024-01-01T12:00:00+00:00'", 'dtobj#330'
assert R(datetime.datetime(2024, 1, 1, 12, 0, tzinfo=datetime.timezone.utc).isoformat(timespec='milliseconds')) == "'2024-01-01T12:00:00.000+00:00'", 'dtobj#331'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).date()) == 'datetime.date(2024, 7, 4)', 'dtobj#336'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).time()) == 'datetime.time(1, 2, 3)', 'dtobj#337'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).replace(year=2000)) == "datetime.datetime(2000, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(days=-1, seconds=66600), 'EST'))", 'dtobj#343'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).replace(microsecond=0)) == "datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(days=-1, seconds=66600), 'EST'))", 'dtobj#344'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).utcoffset()) == 'datetime.timedelta(days=-1, seconds=66600)', 'dtobj#354'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).dst()) == 'None', 'dtobj#356'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).timetz()) == "datetime.time(1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(days=-1, seconds=66600), 'EST'))", 'dtobj#357'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).isoformat(timespec='seconds')) == "'2024-07-04T01:02:03-05:30'", 'dtobj#358'
assert R(datetime.datetime(2024, 7, 4, 1, 2, 3, tzinfo=datetime.timezone(datetime.timedelta(hours=-5, minutes=-30), 'EST')).isoformat(timespec='milliseconds')) == "'2024-07-04T01:02:03.000-05:30'", 'dtobj#359'
assert R(datetime.date(2024, 5, 6).replace(day=2)) == 'datetime.date(2024, 5, 2)', 'dtobj#368'
assert R(datetime.date(2023, 1, 1).replace(day=2)) == 'datetime.date(2023, 1, 2)', 'dtobj#379'
assert R(datetime.date(2020, 12, 31).replace(day=2)) == 'datetime.date(2020, 12, 2)', 'dtobj#390'
assert R(datetime.date(1, 1, 1).replace(day=2)) == 'datetime.date(1, 1, 2)', 'dtobj#401'
assert R(datetime.datetime(2024,1,31) + datetime.timedelta(days=1)) == 'datetime.datetime(2024, 2, 1, 0, 0)', 'dtobj#406'
assert R(datetime.date(2024,1,31) + datetime.timedelta(days=1)) == 'datetime.date(2024, 2, 1)', 'dtobj#407'
assert R(datetime.date(2024,3,1) - datetime.date(2024,2,1)) == 'datetime.timedelta(days=29)', 'dtobj#408'
assert R(datetime.datetime(2024,3,1,12) - datetime.datetime(2024,2,1)) == 'datetime.timedelta(days=29, seconds=43200)', 'dtobj#409'
assert R(datetime.timedelta(days=1) // datetime.timedelta(hours=1)) == '24', 'dtobj#413'
assert R(datetime.timedelta(days=1) % datetime.timedelta(hours=5)) == 'datetime.timedelta(seconds=14400)', 'dtobj#415'
assert R(datetime.timedelta(days=1) // 7) == 'datetime.timedelta(seconds=12342, microseconds=857142)', 'dtobj#416'
assert R(datetime.timedelta(days=1) * 1.5) == 'datetime.timedelta(days=1, seconds=43200)', 'dtobj#417'
assert R(-datetime.timedelta(hours=1)) == 'datetime.timedelta(days=-1, seconds=82800)', 'dtobj#418'
assert R(abs(datetime.timedelta(hours=-1))) == 'datetime.timedelta(seconds=3600)', 'dtobj#419'
assert R(sorted([datetime.date(2024,1,2), datetime.date(2023,1,1)])) == '[datetime.date(2023, 1, 1), datetime.date(2024, 1, 2)]', 'dtobj#422'
assert R(max(datetime.datetime(2024,1,2), datetime.datetime(2023,1,1))) == 'datetime.datetime(2024, 1, 2, 0, 0)', 'dtobj#423'
assert R(datetime.datetime.fromisoformat('2024-05-06T07:08:09')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9)', 'dtobj#425'
assert R(datetime.datetime.fromisoformat('2024-05-06T07:08:09.5')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, 500000)', 'dtobj#426'
assert R(datetime.datetime.fromisoformat('2024-05-06T07:08:09+02:00')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, tzinfo=datetime.timezone(datetime.timedelta(seconds=7200)))', 'dtobj#427'
assert R(datetime.datetime.fromisoformat('2024-05-06')) == 'datetime.datetime(2024, 5, 6, 0, 0)', 'dtobj#428'
assert R(datetime.datetime.fromisoformat('2024-05-06T07:08:09Z')) == 'datetime.datetime(2024, 5, 6, 7, 8, 9, tzinfo=datetime.timezone.utc)', 'dtobj#429'
assert R(datetime.date.fromisoformat('2024-05-06')) == 'datetime.date(2024, 5, 6)', 'dtobj#430'
assert R(datetime.datetime(2024,5,6, 1, 2, 3, 500000, tzinfo=datetime.timezone.utc).timestamp()) == '1714957323.5', 'dtobj#434'
assert R(datetime.date.fromordinal(738000)) == 'datetime.date(2021, 7, 29)', 'dtobj#435'
assert R(datetime.datetime.combine(datetime.date(2024,1,2), datetime.time(3,4))) == 'datetime.datetime(2024, 1, 2, 3, 4)', 'dtobj#436'
assert R(datetime.time(3, 4, 5)) == 'datetime.time(3, 4, 5)', 'dtobj#437'
assert R(str(datetime.time(3, 4, 5, 6))) == "'03:04:05.000006'", 'dtobj#438'
assert R(datetime.time(3,4).isoformat()) == "'03:04:00'", 'dtobj#439'
assert R(datetime.time(13,4).strftime('%I:%M %p')) == "'01:04 PM'", 'dtobj#440'
assert R(datetime.MINYEAR) == '1', 'dtobj#445'
assert R(datetime.MAXYEAR) == '9999', 'dtobj#446'
assert R(datetime.datetime(2024,5,6) == datetime.date(2024,5,6)) == 'False', 'dtobj#448'
assert R(datetime.timedelta(days=1.5)) == 'datetime.timedelta(days=1, seconds=43200)', 'dtobj#451'
assert R(datetime.timedelta(seconds=1.5)) == 'datetime.timedelta(seconds=1, microseconds=500000)', 'dtobj#452'
assert R(datetime.timedelta(minutes=-1)) == 'datetime.timedelta(days=-1, seconds=86340)', 'dtobj#453'
assert R(bool(datetime.timedelta(0))) == 'False', 'dtobj#454'
assert R(datetime.timedelta(days=1) + datetime.timedelta(seconds=1)) == 'datetime.timedelta(days=1, seconds=1)', 'dtobj#456'
assert R(datetime.timedelta(days=1) - datetime.timedelta(days=2)) == 'datetime.timedelta(days=-1)', 'dtobj#457'
assert R(datetime.datetime(2024,1,1) - datetime.timedelta(seconds=1)) == 'datetime.datetime(2023, 12, 31, 23, 59, 59)', 'dtobj#458'
assert R(datetime.datetime(2024, 5, 6, 7, 8, 9, tzinfo=datetime.timezone.utc) - datetime.datetime(2024, 5, 6, tzinfo=datetime.timezone(datetime.timedelta(hours=2)))) == 'datetime.timedelta(seconds=32889)', 'dtobj#460'
print("OK")
