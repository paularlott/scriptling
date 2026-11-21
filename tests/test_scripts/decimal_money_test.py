# Money arithmetic with decimal; expected values from python3.
from decimal import Decimal, getcontext, InvalidOperation, DivisionByZero, ROUND_HALF_UP, ROUND_HALF_EVEN, ROUND_DOWN, ROUND_UP, ROUND_CEILING, ROUND_FLOOR, ROUND_HALF_DOWN, ROUND_05UP
import json

D = Decimal
CENT = Decimal('0.01')

assert repr(D('19.99') * 3) == "Decimal('59.97')"
assert repr((D('19.99') * 3).quantize(CENT)) == "Decimal('59.97')"
assert repr(D('0.10') * 7) == "Decimal('0.70')"
assert repr((D('0.10') * 7).quantize(CENT)) == "Decimal('0.70')"
assert repr(D('1234.50') * 2) == "Decimal('2469.00')"
assert repr((D('1234.50') * 2).quantize(CENT)) == "Decimal('2469.00')"
assert repr(D('2.49') * 12) == "Decimal('29.88')"
assert repr((D('2.49') * 12).quantize(CENT)) == "Decimal('29.88')"
assert repr(D('0.333') * 3) == "Decimal('0.999')"
assert repr((D('0.333') * 3).quantize(CENT)) == "Decimal('1.00')"
assert repr(D('99.95') * 1) == "Decimal('99.95')"
assert repr((D('99.95') * 1).quantize(CENT)) == "Decimal('99.95')"
assert repr(D('5.00') * 0) == "Decimal('0.00')"
assert repr((D('5.00') * 0).quantize(CENT)) == "Decimal('0.00')"
assert repr(D('12.345') * 4) == "Decimal('49.380')"
assert repr((D('12.345') * 4).quantize(CENT)) == "Decimal('49.38')"
assert repr((D('19.99') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('1.50')"
assert repr((D('19.99') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('1.50')"
assert repr((D('19.99') * D('0.075')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('1.49')"
assert repr((D('19.99') * D('0.075')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('1.50')"
assert repr((D('19.99') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('2.00')"
assert repr((D('19.99') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('2.00')"
assert repr((D('19.99') * D('0.10')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('1.99')"
assert repr((D('19.99') * D('0.10')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('2.00')"
assert repr((D('19.99') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('4.00')"
assert repr((D('19.99') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('4.00')"
assert repr((D('19.99') * D('0.20')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('3.99')"
assert repr((D('19.99') * D('0.20')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('4.00')"
assert repr((D('-19.99') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('-1.50')"
assert repr((D('-19.99') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('-1.50')"
assert repr((D('-19.99') * D('0.075')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('-1.49')"
assert repr((D('-19.99') * D('0.075')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('-1.50')"
assert repr((D('-19.99') * D('0.075')).quantize(CENT, rounding=ROUND_CEILING)) == "Decimal('-1.49')"
assert repr((D('-19.99') * D('0.075')).quantize(CENT, rounding=ROUND_FLOOR)) == "Decimal('-1.50')"
assert repr((D('-19.99') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_DOWN)) == "Decimal('-1.50')"
assert repr((D('-19.99') * D('0.075')).quantize(CENT, rounding=ROUND_05UP)) == "Decimal('-1.49')"
assert repr((D('-19.99') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('-2.00')"
assert repr((D('-19.99') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('-2.00')"
assert repr((D('-19.99') * D('0.10')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('-1.99')"
assert repr((D('-19.99') * D('0.10')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('-2.00')"
assert repr((D('-19.99') * D('0.10')).quantize(CENT, rounding=ROUND_CEILING)) == "Decimal('-1.99')"
assert repr((D('-19.99') * D('0.10')).quantize(CENT, rounding=ROUND_FLOOR)) == "Decimal('-2.00')"
assert repr((D('-19.99') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_DOWN)) == "Decimal('-2.00')"
assert repr((D('-19.99') * D('0.10')).quantize(CENT, rounding=ROUND_05UP)) == "Decimal('-1.99')"
assert repr((D('-19.99') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('-4.00')"
assert repr((D('-19.99') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('-4.00')"
assert repr((D('-19.99') * D('0.20')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('-3.99')"
assert repr((D('-19.99') * D('0.20')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('-4.00')"
assert repr((D('-19.99') * D('0.20')).quantize(CENT, rounding=ROUND_CEILING)) == "Decimal('-3.99')"
assert repr((D('-19.99') * D('0.20')).quantize(CENT, rounding=ROUND_FLOOR)) == "Decimal('-4.00')"
assert repr((D('-19.99') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_DOWN)) == "Decimal('-4.00')"
assert repr((D('-19.99') * D('0.20')).quantize(CENT, rounding=ROUND_05UP)) == "Decimal('-3.99')"
assert repr((D('100.05') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('7.50')"
assert repr((D('100.05') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('7.50')"
assert repr((D('100.05') * D('0.075')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('7.50')"
assert repr((D('100.05') * D('0.075')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('7.51')"
assert repr((D('100.05') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('10.01')"
assert repr((D('100.05') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('10.00')"
assert repr((D('100.05') * D('0.10')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('10.00')"
assert repr((D('100.05') * D('0.10')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('10.01')"
assert repr((D('100.05') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('20.01')"
assert repr((D('100.05') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('20.01')"
assert repr((D('100.05') * D('0.20')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('20.01')"
assert repr((D('100.05') * D('0.20')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('20.01')"
assert repr((D('0.15') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('0.01')"
assert repr((D('0.15') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('0.01')"
assert repr((D('0.15') * D('0.075')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('0.01')"
assert repr((D('0.15') * D('0.075')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('0.02')"
assert repr((D('0.15') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('0.02')"
assert repr((D('0.15') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('0.02')"
assert repr((D('0.15') * D('0.10')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('0.01')"
assert repr((D('0.15') * D('0.10')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('0.02')"
assert repr((D('0.15') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('0.03')"
assert repr((D('0.15') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('0.03')"
assert repr((D('0.15') * D('0.20')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('0.03')"
assert repr((D('0.15') * D('0.20')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('0.03')"
assert repr((D('33.33') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('2.50')"
assert repr((D('33.33') * D('0.075')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('2.50')"
assert repr((D('33.33') * D('0.075')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('2.49')"
assert repr((D('33.33') * D('0.075')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('2.50')"
assert repr((D('33.33') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('3.33')"
assert repr((D('33.33') * D('0.10')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('3.33')"
assert repr((D('33.33') * D('0.10')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('3.33')"
assert repr((D('33.33') * D('0.10')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('3.34')"
assert repr((D('33.33') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('6.67')"
assert repr((D('33.33') * D('0.20')).quantize(CENT, rounding=ROUND_HALF_EVEN)) == "Decimal('6.67')"
assert repr((D('33.33') * D('0.20')).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('6.66')"
assert repr((D('33.33') * D('0.20')).quantize(CENT, rounding=ROUND_UP)) == "Decimal('6.67')"
assert repr((D('100.00') / 3).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('33.33')"
assert repr(D('100.00') - (D('100.00') / 3).quantize(CENT, rounding=ROUND_DOWN) * 3) == "Decimal('0.01')"
assert repr(divmod(D('100.00') * 100, 3)) == "(Decimal('3333'), Decimal('1.00'))"
assert repr((D('10.00') / 7).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('1.42')"
assert repr(D('10.00') - (D('10.00') / 7).quantize(CENT, rounding=ROUND_DOWN) * 7) == "Decimal('0.06')"
assert repr(divmod(D('10.00') * 100, 7)) == "(Decimal('142'), Decimal('6.00'))"
assert repr((D('250.75') / 4).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('62.68')"
assert repr(D('250.75') - (D('250.75') / 4).quantize(CENT, rounding=ROUND_DOWN) * 4) == "Decimal('0.03')"
assert repr(divmod(D('250.75') * 100, 4)) == "(Decimal('6268'), Decimal('3.00'))"
assert repr((D('0.05') / 2).quantize(CENT, rounding=ROUND_DOWN)) == "Decimal('0.02')"
assert repr(D('0.05') - (D('0.05') / 2).quantize(CENT, rounding=ROUND_DOWN) * 2) == "Decimal('0.01')"
assert repr(divmod(D('0.05') * 100, 2)) == "(Decimal('2'), Decimal('1.00'))"
assert repr(sum([D('19.99'), D('5.01'), D('0.10')])) == "Decimal('25.10')"
assert repr(sum([D('0.1')] * 10)) == "Decimal('1.0')"
assert repr(sum([D('0.1')] * 10) == D('1')) == 'True'
assert repr(sum((D(p) * q for p, q in [('2.50', 4), ('1.25', 2)]), D('0'))) == "Decimal('12.50')"
assert repr(sum([D('1.10'), D('2.20'), D('3.30')]) / 3) == "Decimal('2.20')"
assert repr(sum([D('-5.00'), D('5.00')])) == "Decimal('0.00')"
assert repr((D('80.00') * (1 - D('15') / 100)).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('68.00')"
assert repr(D('80.00') * D('15') / 100) == "Decimal('12.00')"
assert repr((D('19.99') * (1 - D('10') / 100)).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('17.99')"
assert repr(D('19.99') * D('10') / 100) == "Decimal('1.999')"
assert repr((D('49.95') * (1 - D('33') / 100)).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('33.47')"
assert repr(D('49.95') * D('33') / 100) == "Decimal('16.4835')"
assert repr((D('1000') * (1 - D('12.5') / 100)).quantize(CENT, rounding=ROUND_HALF_UP)) == "Decimal('875.00')"
assert repr(D('1000') * D('12.5') / 100) == "Decimal('125.0')"
assert repr(D('1.10') == D('1.1')) == 'True'
assert repr(D('1.10') < D('1.2')) == 'True'
assert repr(D('-0') == D('0')) == 'True'
assert repr(D('10') > 9) == 'True'
assert repr(9.5 < D('10')) == 'True'
assert repr(D('0.1') == 0.1) == 'False'
assert repr(D('0.5') == 0.5) == 'True'
assert repr(sorted([D('10.00'), D('9.99'), D('-1'), D('0.001')])) == "[Decimal('-1'), Decimal('0.001'), Decimal('9.99'), Decimal('10.00')]"
assert repr(max([D('1.50'), D('1.5'), D('1.49')])) == "Decimal('1.50')"
assert repr(min(D('3'), D('2.999'))) == "Decimal('2.999')"
assert repr(sorted([D('2'), 1, D('1.5')])) == "[1, Decimal('1.5'), Decimal('2')]"
assert repr(D('2.00') in {D('2'): 1}) == 'True'
assert repr({D('2.50'): 'x'}.get(D('2.5'))) == "'x'"
assert repr(len({D('1.0'), D('1.00'), D(1)})) == '1'
assert repr(hash(D('2.00')) == hash(2)) == 'True'
assert repr(bool(D('0.00'))) == 'False'
assert repr(bool(D('0.01'))) == 'True'
assert repr(f"{D('1234.5'):.2f}") == "'1234.50'"
assert repr(f"{D('1234567.891'):,.2f}") == "'1,234,567.89'"
assert repr('{:>10}'.format(D('3.14'))) == "'      3.14'"
assert repr('{:<10}|'.format(D('3.14'))) == "'3.14      |'"
assert repr('{:^10}'.format(D('3.14'))) == "'   3.14   '"
assert repr(f"{D('-0.005'):.2f}") == "'-0.00'"
assert repr(f"{D('0.125'):.2f}") == "'0.12'"
assert repr(f"{D('0.135'):.2f}") == "'0.14'"
assert repr(f"{D('12.5'):.0%}") == "'1250%'"
assert repr(f"{D('0.075'):.1%}") == "'7.5%'"
assert repr(f"{D('1E+3'):f}") == "'1000'"
assert repr(f"{D('123.456'):e}") == "'1.23456e+2'"
assert repr(f"{D('123.456'):.2e}") == "'1.23e+2'"
assert repr(f"{D('0.000012'):g}") == "'0.000012'"
assert repr(f"{D('42'):08.2f}") == "'00042.00'"
assert repr(f"{D('-42'):+.1f}") == "'-42.0'"
assert repr(f"{D('42'):+.1f}") == "'+42.0'"
assert repr('%s' % D('1.50')) == "'1.50'"
assert repr('{}'.format(D('1.50'))) == "'1.50'"
assert repr(str(D('1.50'))) == "'1.50'"
assert repr('%.2f' % D('2.5')) == "'2.50'"
assert repr(D('1.10') + D('2.20')) == "Decimal('3.30')"
assert repr(D('1.20') * D('3')) == "Decimal('3.60')"
assert repr(str(D('1E+3'))) == "'1E+3'"
assert repr(D('0.00')) == "Decimal('0.00')"
assert repr(D('-0')) == "Decimal('-0')"
assert repr(D('-0.00') + D('0')) == "Decimal('0.00')"
assert repr(-D('0.00')) == "Decimal('0.00')"
assert repr(D('1.0') * D('1.00')) == "Decimal('1.000')"
assert repr(D('5') - D('5.00')) == "Decimal('0.00')"
assert repr(D('1.30') - D('1.10')) == "Decimal('0.20')"
assert repr(D('100') / D('10')) == "Decimal('10')"
assert repr(D('100.0') / D('10')) == "Decimal('10.0')"
assert repr(D('1E+2') + D('1')) == "Decimal('101')"
assert repr(D('1E+2') * 1) == "Decimal('1E+2')"
assert repr(D('2.50') * D('4')) == "Decimal('10.00')"
assert repr(D('3.000') / D('1.5')) == "Decimal('2.00')"
assert repr(D(10) ** 2) == "Decimal('100')"
assert repr(D('1.1') ** 2) == "Decimal('1.21')"
assert repr(D('2') ** -2) == "Decimal('0.25')"
assert repr(D('0.1') ** 3) == "Decimal('0.001')"
assert repr(D(1) / D(3)) == "Decimal('0.3333333333333333333333333333')"
assert repr(D(2) / D(4)) == "Decimal('0.5')"
assert repr(D(2) / D(3)) == "Decimal('0.6666666666666666666666666667')"
assert repr(D('10.00') / D('4')) == "Decimal('2.50')"
assert repr(D(1) / 8) == "Decimal('0.125')"
assert repr(D('-7') / D('2')) == "Decimal('-3.5')"
assert repr(D(1) / D('0.3')) == "Decimal('3.333333333333333333333333333')"
assert repr(D(7) // D(2)) == "Decimal('3')"
assert repr(D(-7) // D(2)) == "Decimal('-3')"
assert repr(D(-7) % D(2)) == "Decimal('-1')"
assert repr(D('7.5') % D('2')) == "Decimal('1.5')"
assert repr(D('1E+2') // 7) == "Decimal('14')"
assert repr(round(D('2.675'), 2)) == "Decimal('2.68')"
assert repr(round(D('2.665'), 2)) == "Decimal('2.66')"
assert repr(round(D('2.5'))) == '2'
assert repr(round(D('3.5'))) == '4'
assert repr(round(D('-2.5'))) == '-2'
assert repr(int(D('19.99'))) == '19'
assert repr(int(D('-19.99'))) == '-19'
assert repr(float(D('19.99'))) == '19.99'
assert repr(D(19.99)) == "Decimal('19.989999999999998436805981327779591083526611328125')"
assert repr(D(0.5)) == "Decimal('0.5')"
assert repr(D(3)) == "Decimal('3')"
assert repr(D('  42  ')) == "Decimal('42')"
assert repr(D('1_000.50')) == "Decimal('1000.50')"
assert repr(D('-1.5e3')) == "Decimal('-1.5E+3')"
assert repr(D('.5')) == "Decimal('0.5')"
assert repr(D('5.')) == "Decimal('5')"
assert repr(D('1.2345').quantize(D('0.01'))) == "Decimal('1.23')"
assert repr(D('123.456').quantize(D('1'))) == "Decimal('123')"
assert repr(D('123.456').quantize(D('1E+1'))) == "Decimal('1.2E+2')"
assert repr(D('1.20').normalize()) == "Decimal('1.2')"
assert repr(D('120').normalize()) == "Decimal('1.2E+2')"
assert repr(D('3.7').to_integral_value()) == "Decimal('4')"
assert repr(D('3.5').to_integral_value(rounding=ROUND_HALF_UP)) == "Decimal('4')"
assert repr(D('-3.5').to_integral_value(rounding=ROUND_DOWN)) == "Decimal('-3')"
assert repr(D('-1').is_signed()) == 'True'
assert repr(D('0').is_zero()) == 'True'
assert repr(D('-2.5').copy_abs()) == "Decimal('2.5')"
assert repr(D('2.5').copy_negate()) == "Decimal('-2.5')"
assert repr(abs(D('-2.50'))) == "Decimal('2.50')"
assert repr(+D('1.000')) == "Decimal('1.000')"
assert repr(-D('1.50')) == "Decimal('-1.50')"
assert repr(D(5) + 1) == "Decimal('6')"
assert repr(1 + D(5)) == "Decimal('6')"
assert repr(2 * D('1.25')) == "Decimal('2.50')"
assert repr(10 - D('0.01')) == "Decimal('9.99')"
assert repr(100 / D(8)) == "Decimal('12.5')"
assert repr(7 // D(2)) == "Decimal('3')"
assert repr(7 % D(2)) == "Decimal('1')"
assert repr(2 ** D(3)) == "Decimal('8')"
assert repr(getcontext().prec) == '28'
assert repr(getcontext().rounding) == "'ROUND_HALF_EVEN'"
assert repr(type(D(1)).__name__) == "'Decimal'"
assert repr(isinstance(D(1), Decimal)) == 'True'
assert repr(D(1) == True) == 'True'

def raises(exc, fn):
    try:
        fn()
    except exc:
        return True
    return False

assert raises(DivisionByZero, lambda: D(1) / D(0))
assert raises(ZeroDivisionError, lambda: D('19.99') / 0)
assert raises(InvalidOperation, lambda: D(0) / D(0))
assert raises(InvalidOperation, lambda: D('12.3.4'))
assert raises(InvalidOperation, lambda: D('abc'))
assert raises(ArithmeticError, lambda: D('1e30').quantize(CENT))
assert raises(TypeError, lambda: D('1.5') + 1.5)
assert raises(TypeError, lambda: 1.5 * D('2'))
assert raises(TypeError, lambda: json.dumps(D('1.5')))
assert json.dumps({'total': D('1.50')}, default=str) == '{"total": "1.50"}'
assert raises(DivisionByZero, lambda: D(5) // 0)
assert raises(InvalidOperation, lambda: D(5) % 0)

# The context: precision and rounding are settable
ctx = getcontext()
ctx.prec = 6
assert repr(D(1) / D(7)) == "Decimal('0.142857')"
ctx.rounding = ROUND_DOWN
assert repr(D(2) / D(3)) == "Decimal('0.666666')"
ctx.prec = 28
ctx.rounding = ROUND_HALF_EVEN
assert repr(D(2) / D(3)) == "Decimal('0.6666666666666666666666666667')"

print('OK')
