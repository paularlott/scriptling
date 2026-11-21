import string
from string import Template

s = Template("$who likes ${what}! Cost: $$5 $who")
assert s.substitute(who="tim", what="kung pao") == "tim likes kung pao! Cost: $5 tim"
assert s.substitute({"who": "ann", "what": 3}) == "ann likes 3! Cost: $5 ann"
assert s.substitute({"who": "ann", "what": 3}, who="bob") == "bob likes 3! Cost: $5 bob"
assert s.safe_substitute(who="tim") == "tim likes ${what}! Cost: $5 tim"
assert s.template == "$who likes ${what}! Cost: $$5 $who"
assert s.get_identifiers() == ["who", "what"]
assert s.is_valid() and not Template("$").is_valid()

try:
    s.substitute(who="tim")
    assert False, "expected KeyError"
except KeyError as e:
    assert "what" in str(e)

try:
    Template("abc $ x").substitute()
    assert False, "expected ValueError"
except ValueError as e:
    assert "Invalid placeholder" in str(e)

assert Template("$1 and ${bad").safe_substitute() == "$1 and ${bad"
assert Template("${a}b$a_c").substitute(a=1, a_c=2.5) == "1b2.5"
assert string.Template("x$y").substitute(y=None) == "xNone"
assert Template("${x} $x").safe_substitute({}) == "${x} $x"


class P:
    def __str__(self):
        return "P!"


assert Template("<$p>").substitute(p=P()) == "<P!>"


class Percent(Template):
    delimiter = "%"


assert Percent("%name is %% here").substitute(name="x") == "x is % here"

print("OK")
