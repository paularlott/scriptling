# `except E as e` unbinds e when the handler ends, and exceptions raised
# while handling another carry __context__ / __cause__ / __suppress_context__.


def unbound_after_handler():
    try:
        raise ValueError("a")
    except ValueError as e:
        assert str(e) == "a"
    try:
        e
        return "bound"
    except NameError:
        return "deleted"


assert unbound_after_handler() == "deleted"


def shadowed_local():
    e = 5
    try:
        raise ValueError()
    except ValueError as e:
        pass
    return "e" in locals()


assert shadowed_local() is False

x = 1
try:
    raise ValueError
except ValueError as x:
    pass
try:
    x
    assert False
except NameError:
    pass


def returns_exception():
    try:
        raise KeyError("k")
    except KeyError as e:
        return e


assert isinstance(returns_exception(), KeyError)


def implicit_context():
    try:
        try:
            raise ValueError("a")
        except ValueError:
            raise KeyError("b")
    except KeyError as e:
        return (type(e.__context__).__name__, e.__cause__, e.__suppress_context__)


assert implicit_context() == ("ValueError", None, False)


def explicit_cause():
    try:
        try:
            raise ValueError("a")
        except ValueError as v:
            raise KeyError("b") from v
    except KeyError as e:
        return (type(e.__cause__).__name__, type(e.__context__).__name__, e.__suppress_context__)


assert explicit_cause() == ("ValueError", "ValueError", True)


def from_none():
    try:
        raise KeyError("b") from None
    except KeyError as e:
        return (e.__cause__, e.__context__, e.__suppress_context__)


assert from_none() == (None, None, True)


def no_context():
    try:
        raise ValueError("x")
    except ValueError as e:
        return e.__context__


assert no_context() is None


class MyErr(Exception):
    pass


def operation_error_context():
    try:
        try:
            {}["k"]
        except KeyError:
            raise MyErr("m")
    except MyErr as e:
        return type(e.__context__).__name__


assert operation_error_context() == "KeyError"


def error_in_handler():
    try:
        try:
            raise ValueError("v")
        except ValueError:
            {}["missing"]
    except KeyError as e:
        return type(e.__context__).__name__


assert error_in_handler() == "ValueError"


def finally_no_context():
    try:
        try:
            raise ValueError("a")
        finally:
            pass
    except ValueError as e:
        return e.__context__


assert finally_no_context() is None


def nested_handled():
    try:
        try:
            1 / 0
        except ZeroDivisionError:
            try:
                raise ValueError("inner")
            except ValueError:
                pass
            raise KeyError("k")
    except KeyError as e:
        return type(e.__context__).__name__


assert nested_handled() == "ZeroDivisionError"


def reraise_keeps_no_self_context():
    try:
        try:
            raise ValueError("a")
        except ValueError:
            raise
    except ValueError as e:
        return e.__context__


assert reraise_keeps_no_self_context() is None


def gen():
    try:
        raise ValueError("g")
    except ValueError as e:
        yield str(e)
    try:
        e
        yield "bound"
    except NameError:
        yield "deleted"
    try:
        try:
            raise ValueError("a")
        except ValueError:
            yield "handling"
            raise KeyError("b")
    except KeyError as k:
        yield type(k.__context__).__name__


assert list(gen()) == ["g", "deleted", "handling", "ValueError"]

print("OK")
