# PEP 701 (Python 3.12) f-strings: the enclosing quote may be reused inside a
# replacement field, f-strings nest, and fields may span lines.
d = {"key": "v", "a b": 1}
xs = ["p", "q"]
x = "hi"
w = 6
n = 3.14159

assert f"{d["key"]}" == "v"
assert f'{d['key']}' == "v"
assert f"{", ".join(xs)}" == "p, q"
assert f"{x!r:>{w}}|" == "  'hi'|"
assert f"{f"{x}"}" == "hi"
assert f"{f"{d["key"]}"}" == "v"
assert f"{f'{f"{x}"}'}" == "hi"
assert f"""{
    x +
    "!"
}""" == "hi!"
assert f"""a{
d["key"]
}b""" == "avb"
assert f"{n:.{2}f} {n:{w}.2f}|" == "3.14   3.14|"
assert f"{'{'}{'}'}" == "{}"
assert f"{{literal}} {x}" == "{literal} hi"
assert f"{x=}, {d["a b"]=}" == "x='hi', d[\"a b\"]=1"
assert f"{'\n'.join(xs)}" == "p\nq"
assert f"{xs[0]!s:^5}|{len(xs):03d}" == "  p  |002"
assert fr"{d["key"]}\n" == "v\\n"
assert rf"\d{xs[1]}" == "\\dq"
assert f"{"a" if x else "b"}" == "a"
assert f"{(lambda y: y * 2)(3)}" == "6"
assert f"{d.get("missing", "def")}" == "def"
assert f'{"\'"}' == "'"
assert f"{1, 2}" == "(1, 2)"
assert F"{x}" f"{x}" "{x}" == "hihi{x}"
assert f"""{x}""" + f'''{d['key']}''' == "hiv"
assert f"{ {'a': 1}['a'] }" == "1"
assert f"{[f"{i}" for i in range(3)]}" == "['0', '1', '2']"
assert f"{"}"}" == "}"
assert f"{"{"}{x}" == "{hi"
assert f"{d["key"]!r:>5}" == "  'v'"
assert f"{'"'}" == '"'
assert f'''{d["key"] + d['key']}''' == "vv"
assert f"{"""tri"""}" == "tri"
rows = [("a", 1), ("bb", 22)]
assert "\n".join(f"{name:<3}|{val:>3}" for name, val in rows) == "a  |  1\nbb | 22"
assert f"{'x' * 3}{"y" * 2}" == "xxxyy"

print("OK")
