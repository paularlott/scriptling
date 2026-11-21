# str find/rfind/index/rindex/count, ljust/rjust/center, split/rsplit and the
# bytes split/rsplit: character positions, None bounds, fill characters,
# empty separators and argument types. Everything here behaves the same
# under CPython 3.

def expect(exc, fn):
    try:
        fn()
    except exc:
        return True
    except Exception as e:
        return "wrong: " + type(e).__name__
    return "no error"

# --- find family: positions and bounds are characters, not bytes ---
assert 'hello world'.find('o') == 4
assert 'hello world'.rfind('o') == 7
assert 'hello world'.find('o', 5) == 7
assert 'hello world'.find('o', 5, 7) == -1
assert 'hello world'.count('o') == 2
assert 'hello world'.count('l', 3, -1) == 2
assert '日本日本'.find('日') == 0
assert '日本日本'.rfind('日') == 2
assert '日本日本'.find('本', 2) == 3
assert '日本日本'.find('日', 1, 3) == 2
assert '日本日本'.find('日', 1, 2) == -1
assert '日本日本'.index('本', -2) == 3
assert '日本日本'.rindex('日', 0, -1) == 2
assert '日本日本'.count('日') == 2
assert '日本日本'.count('日', 1) == 1
assert '日本日本'.count('') == 5
assert 'é-é-é'.find('-', -2) == 3
assert 'é-é-é'.rfind('é', 0, 4) == 2
assert 'aé'.find('é', 2) == -1
assert 'aé'.find('', 2) == 2
assert 'aé'.find('', 3) == -1
assert 'abc'.find('', 3) == 3
assert 'abc'.find('', 4) == -1
assert 'abc'.count('', 4) == 0
assert 'abc'.find('', 2, 1) == -1
assert 'abc'.find('c', -100) == 2
assert 'abc'.find('a', 0, -100) == -1
assert 'abc'.find('a', True) == -1
assert expect(ValueError, lambda: '日本'.index('x')) == True
assert expect(ValueError, lambda: '日本'.rindex('日', 1)) == True

# --- find family: None means "not given" ---
assert 'abc'.find('b', None) == 1
assert 'abc'.find('b', None, None) == 1
assert 'abcb'.rfind('b', None, 3) == 1
assert 'abc'.index('c', 1, None) == 2
assert 'abcb'.rindex('b', None, None) == 3
assert 'abcb'.count('b', None, 2) == 1
assert '日本日本'.find('日', 1, None) == 2
assert '日本日本'.count('本', None, 2) == 1

# --- find family: wrong argument types are TypeError ---
assert expect(TypeError, lambda: 'abc'.find(1)) == True
assert expect(TypeError, lambda: 'abc'.rfind(None)) == True
assert expect(TypeError, lambda: 'abc'.index('a', 'x')) == True
assert expect(TypeError, lambda: 'abc'.rindex('a', 0, 1.5)) == True
assert expect(TypeError, lambda: 'abc'.count(b'a')) == True
assert expect(TypeError, lambda: '日本'.find('日', 1.0)) == True

# --- startswith/endswith with character bounds ---
assert '日本x'.startswith('x', 2) == True
assert '日本x'.endswith('本', 0, 2) == True
assert '日本x'.startswith('本', 1, 2) == True
assert 'abc'.startswith('c', -1) == True
assert 'abc'.startswith('', 3) == True
assert 'abc'.startswith('', 4) == False
assert 'abc'.endswith('b', None, -1) == True

# --- zfill pads to a width in characters ---
assert '日'.zfill(3) == '00日'
assert '-日'.zfill(4) == '-00日'
assert '42'.zfill(5) == '00042'

# --- ljust/rjust/center pad to a width in characters ---
assert 'ab'.ljust(5) == 'ab   '
assert 'ab'.rjust(5, '*') == '***ab'
assert 'ab'.center(7, '*') == '***ab**'
assert 'x'.center(6) == '  x   '
assert '日本'.ljust(4) == '日本  '
assert '日本'.rjust(4, '.') == '..日本'
assert '日本'.center(5, '-') == '--日本-'
assert '日本'.ljust(2) == '日本'
assert 'abc'.ljust(5, 'é') == 'abcéé'
assert 'abc'.rjust(5, '日') == '日日abc'
assert 'é'.center(4, '本') == '本é本本'
assert 'abc'.ljust(-1) == 'abc'

# --- the fill character must be exactly one character, even when no padding is needed ---
assert expect(TypeError, lambda: 'abc'.ljust(2, 'ab')) == True
assert expect(TypeError, lambda: 'abc'.rjust(5, '')) == True
assert expect(TypeError, lambda: 'abc'.center(1, '日本')) == True
assert expect(TypeError, lambda: '日本'.ljust(1, 'xy')) == True
assert expect(TypeError, lambda: 'abc'.ljust(5, None)) == True
assert expect(TypeError, lambda: 'abc'.ljust('5')) == True
assert expect(TypeError, lambda: 'abc'.center(5.0)) == True

# --- split/rsplit: an empty separator is a ValueError ---
assert expect(ValueError, lambda: 'abc'.split('')) == True
assert expect(ValueError, lambda: 'abc'.rsplit('')) == True
assert expect(ValueError, lambda: 'abc'.split(sep='')) == True
assert expect(ValueError, lambda: '日本'.rsplit(sep='', maxsplit=1)) == True
assert expect(ValueError, lambda: b'abc'.split(b'')) == True
assert expect(ValueError, lambda: b'abc'.rsplit(b'')) == True

# --- split/rsplit: wrong argument types are TypeError ---
assert expect(TypeError, lambda: 'abc'.split(1)) == True
assert expect(TypeError, lambda: 'abc'.rsplit(b'b')) == True
assert expect(TypeError, lambda: 'a b'.split(maxsplit=None)) == True
assert expect(TypeError, lambda: 'a b'.rsplit(' ', '1')) == True
assert expect(TypeError, lambda: b'a b'.split(1)) == True
assert expect(TypeError, lambda: b'a b'.split(maxsplit=None)) == True
assert expect(TypeError, lambda: b'a b'.rsplit('b')) == True

# --- split/rsplit results ---
assert 'a,b,c'.split(',') == ['a', 'b', 'c']
assert 'a,b,c'.rsplit(',', 1) == ['a,b', 'c']
assert ' a  b c '.split() == ['a', 'b', 'c']
assert ' a  b c '.rsplit(None, 1) == [' a  b', 'c']
assert '日,本,日'.split(',', 1) == ['日', '本,日']
assert '日 本 日'.rsplit(maxsplit=1) == ['日 本', '日']

# --- bytes.split and bytes.rsplit ---
assert b'a,b,c'.split(b',') == [b'a', b'b', b'c']
assert b'a,b,c'.split(b',', 1) == [b'a', b'b,c']
assert b'a,b,c'.rsplit(b',') == [b'a', b'b', b'c']
assert b'a,b,c'.rsplit(b',', 1) == [b'a,b', b'c']
assert b'a,b,c'.rsplit(sep=b',', maxsplit=1) == [b'a,b', b'c']
assert b' a  b c '.split() == [b'a', b'b', b'c']
assert b' a  b c '.split(None, 1) == [b'a', b'b c ']
assert b' a  b c '.rsplit() == [b'a', b'b', b'c']
assert b' a  b c '.rsplit(None, 1) == [b' a  b', b'c']
assert b' a  b c '.rsplit(maxsplit=0) == [b' a  b c']
assert b''.rsplit() == []
assert b''.rsplit(b',') == [b'']
assert expect(TypeError, lambda: b'a'.rsplit(b',', 1, 2)) == True
assert expect(TypeError, lambda: b'a'.rsplit(x=1)) == True
