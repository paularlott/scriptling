package stdlib

import (
	"strconv"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Python's strftime and strptime, directive by directive, in the C locale,
// for datetime, date, time.strftime and time.strptime.

var (
	shortDays   = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	longDays    = [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	shortMonths = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	longMonths  = [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
)

// zoneMode says what %z and %Z show.
type zoneMode int

const (
	zoneNone  zoneMode = iota // naive datetime or date: both empty
	zoneAware                 // aware datetime: its offset and tzname()
	zoneLocal                 // time.strftime: the local zone
)

// pyStrftime formats t as Python's strftime does in the C locale. Unknown
// directives are copied through unchanged, as glibc does.
func pyStrftime(t time.Time, format string, zm zoneMode) string {
	var b strings.Builder
	b.Grow(len(format) + 16)
	pad := func(n, width int, padChar byte) {
		s := strconv.Itoa(n)
		for i := len(s); i < width; i++ {
			b.WriteByte(padChar)
		}
		b.WriteString(s)
	}
	hour12 := func() int {
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		return h
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(format) {
			b.WriteByte('%')
			break
		}
		i++
		d := format[i]
		// %-d and friends drop the padding (glibc and BSD); E and O are
		// locale modifiers with no effect in the C locale.
		noPad := false
		if d == '-' && i+1 < len(format) {
			noPad = true
			i++
			d = format[i]
		} else if (d == 'E' || d == 'O') && i+1 < len(format) {
			i++
			d = format[i]
		}
		w := func(width int) int {
			if noPad {
				return 0
			}
			return width
		}
		switch d {
		case 'a':
			b.WriteString(shortDays[t.Weekday()])
		case 'A':
			b.WriteString(longDays[t.Weekday()])
		case 'b', 'h':
			b.WriteString(shortMonths[t.Month()-1])
		case 'B':
			b.WriteString(longMonths[t.Month()-1])
		case 'c':
			b.WriteString(pyStrftime(t, "%a %b %e %H:%M:%S %Y", zm))
		case 'C':
			pad(t.Year()/100, w(2), '0')
		case 'd':
			pad(t.Day(), w(2), '0')
		case 'D', 'x':
			b.WriteString(pyStrftime(t, "%m/%d/%y", zm))
		case 'e':
			pad(t.Day(), w(2), ' ')
		case 'f':
			pad(t.Nanosecond()/1000, 6, '0')
		case 'F':
			b.WriteString(pyStrftime(t, "%Y-%m-%d", zm))
		case 'G':
			y, _ := t.ISOWeek()
			pad(y, w(4), '0')
		case 'g':
			y, _ := t.ISOWeek()
			pad(y%100, w(2), '0')
		case 'H':
			pad(t.Hour(), w(2), '0')
		case 'I':
			pad(hour12(), w(2), '0')
		case 'j':
			pad(t.YearDay(), w(3), '0')
		case 'k':
			pad(t.Hour(), w(2), ' ')
		case 'l':
			pad(hour12(), w(2), ' ')
		case 'm':
			pad(int(t.Month()), w(2), '0')
		case 'M':
			pad(t.Minute(), w(2), '0')
		case 'n':
			b.WriteByte('\n')
		case 'p':
			if t.Hour() < 12 {
				b.WriteString("AM")
			} else {
				b.WriteString("PM")
			}
		case 'r':
			b.WriteString(pyStrftime(t, "%I:%M:%S %p", zm))
		case 'R':
			b.WriteString(pyStrftime(t, "%H:%M", zm))
		case 's':
			// The fields read as local time, as the C library's mktime
			// does for Python, whatever the datetime's zone.
			local := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
			b.WriteString(strconv.FormatInt(local.Unix(), 10))
		case 'S':
			pad(t.Second(), w(2), '0')
		case 't':
			b.WriteByte('\t')
		case 'T', 'X':
			b.WriteString(pyStrftime(t, "%H:%M:%S", zm))
		case 'u':
			b.WriteString(strconv.Itoa(int(isoWeekday(t))))
		case 'U':
			pad((t.YearDay()+6-int(t.Weekday()))/7, w(2), '0')
		case 'V':
			_, wk := t.ISOWeek()
			pad(wk, w(2), '0')
		case 'w':
			b.WriteString(strconv.Itoa(int(t.Weekday())))
		case 'W':
			pad((t.YearDay()+6-(int(t.Weekday())+6)%7)/7, w(2), '0')
		case 'y':
			pad(t.Year()%100, w(2), '0')
		case 'Y':
			pad(t.Year(), w(4), '0')
		case 'z':
			if zm != zoneNone {
				_, off := t.Zone()
				b.WriteString(formatUTCOffset(off, false))
			}
		case ':':
			// %:z is the offset with a colon.
			if i+1 < len(format) && format[i+1] == 'z' {
				i++
				if zm != zoneNone {
					_, off := t.Zone()
					b.WriteString(formatUTCOffset(off, true))
				}
			} else {
				b.WriteString("%:")
			}
		case 'Z':
			switch zm {
			case zoneAware:
				if k, ok := awareKey(t.Location()); ok {
					b.WriteString(tzDisplayName(k))
				}
			case zoneLocal:
				name, _ := t.Zone()
				b.WriteString(name)
			}
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			if noPad {
				b.WriteByte('-')
			}
			b.WriteByte(d)
		}
	}
	return b.String()
}

// strftimeZone is the zone mode for a datetime or date's time.
func strftimeZone(t time.Time) zoneMode {
	if _, aware := awareKey(t.Location()); aware {
		return zoneAware
	}
	return zoneNone
}

// strptimeResult holds the fields strptime parsed.
type strptimeResult struct {
	year, month, day, hour, minute, second, micro int
	yday, wday                                    int // -1 when not given; wday is Monday=0
	weekU, weekW, isoYear, isoWeek                int // -1 when not given
	hour12, ampm                                  int // hour12 -1 when not given; ampm 0 none, 1 AM, 2 PM
	hasOffset                                     bool
	offset                                        int // seconds east of UTC
	tzname                                        string
}

type strptimeError struct{ msg string }

// pyStrptime parses text against format as Python's strptime does: numeric
// fields take one or more digits up to their width, names match whatever
// their case, whitespace in the format matches any run of whitespace, and
// the whole text must be consumed.
func pyStrptime(text, format string) (*strptimeResult, object.Object) {
	r := &strptimeResult{year: 1900, month: 1, day: 1, yday: -1, wday: -1, weekU: -1, weekW: -1, isoYear: -1, isoWeek: -1, hour12: -1}
	noMatch := func() object.Object {
		return errors.NewValueError("time data %s does not match format %s", object.ReprString(text), object.ReprString(format))
	}
	// A day of the month needs a year (else 29 February cannot parse), as
	// Python 3.15 requires.
	if dirs := strptimeDirectives(format); dirs['d'] && !dirs['Y'] && !dirs['y'] && !dirs['G'] && !dirs['c'] && !dirs['x'] {
		return nil, errors.NewValueError("Day of month directive '%%d' may not be used without a year directive. Parsing dates involving a day of month without a year is ambiguous and fails to parse leap day. Add a year to the input and format.")
	}
	pos, errObj := strptimeInto(r, text, 0, format, format, noMatch)
	if errObj != nil {
		return nil, errObj
	}
	if pos < len(text) {
		return nil, errors.NewValueError("unconverted data remains: %s", text[pos:])
	}
	if r.hour12 >= 0 {
		r.hour = r.hour12
		switch r.ampm {
		case 2:
			if r.hour != 12 {
				r.hour += 12
			}
		default:
			if r.hour == 12 {
				r.hour = 0
			}
		}
	}
	// A day of the year, or a week and weekday, fixes the date.
	switch {
	case r.yday >= 1:
		d := time.Date(r.year, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, r.yday-1)
		r.month, r.day = int(d.Month()), d.Day()
	case r.isoYear >= 0 && r.isoWeek >= 0 && r.wday >= 0:
		// ISO week 1 holds January 4th.
		jan4 := time.Date(r.isoYear, 1, 4, 0, 0, 0, 0, time.UTC)
		monday := jan4.AddDate(0, 0, -((int(jan4.Weekday()) + 6) % 7))
		d := monday.AddDate(0, 0, (r.isoWeek-1)*7+r.wday)
		r.year, r.month, r.day = d.Year(), int(d.Month()), d.Day()
	case (r.weekU >= 0 || r.weekW >= 0) && r.wday >= 0:
		jan1 := time.Date(r.year, 1, 1, 0, 0, 0, 0, time.UTC)
		jan1wd := (int(jan1.Weekday()) + 6) % 7 // Monday=0
		var yday int
		if r.weekW >= 0 {
			firstMonday := (7 - jan1wd) % 7
			if r.weekW == 0 {
				yday = r.wday - jan1wd
			} else {
				yday = firstMonday + (r.weekW-1)*7 + r.wday
			}
		} else {
			sunWd := (r.wday + 1) % 7 // Sunday=0
			jan1Sun := (jan1wd + 1) % 7
			firstSunday := (7 - jan1Sun) % 7
			if r.weekU == 0 {
				yday = sunWd - jan1Sun
			} else {
				yday = firstSunday + (r.weekU-1)*7 + sunWd
			}
		}
		d := jan1.AddDate(0, 0, yday)
		r.year, r.month, r.day = d.Year(), int(d.Month()), d.Day()
	}
	return r, nil
}

// strptimeInto matches format against text from pos, filling r. top is the
// user's format, for error messages.
func strptimeInto(r *strptimeResult, text string, pos int, format, top string, noMatch func() object.Object) (int, object.Object) {
	isSpace := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f' }
	// num reads between 1 and maxDigits digits whose value lies in
	// [lo, hi], preferring the longest; a leading space may stand for a
	// leading zero where spaceOK.
	num := func(maxDigits, lo, hi int, spaceOK bool) (int, bool) {
		p := pos
		if spaceOK && p < len(text) && text[p] == ' ' && p+1 < len(text) && text[p+1] >= '1' && text[p+1] <= '9' {
			p++
		}
		n := 0
		for n < maxDigits && p+n < len(text) && text[p+n] >= '0' && text[p+n] <= '9' {
			n++
		}
		for ; n >= 1; n-- {
			v, _ := strconv.Atoi(text[p : p+n])
			if v >= lo && v <= hi {
				pos = p + n
				return v, true
			}
		}
		return 0, false
	}
	exact := func(digits int) (int, bool) {
		if pos+digits > len(text) {
			return 0, false
		}
		v := 0
		for k := 0; k < digits; k++ {
			c := text[pos+k]
			if c < '0' || c > '9' {
				return 0, false
			}
			v = v*10 + int(c-'0')
		}
		pos += digits
		return v, true
	}
	// name matches the longest of names at pos, ignoring case.
	name := func(names ...[]string) (int, bool) {
		best, bestLen := -1, 0
		for _, list := range names {
			for idx, nm := range list {
				if len(nm) > bestLen && pos+len(nm) <= len(text) && strings.EqualFold(text[pos:pos+len(nm)], nm) {
					best, bestLen = idx, len(nm)
				}
			}
		}
		if best < 0 {
			return 0, false
		}
		pos += bestLen
		return best, true
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if isSpace(c) {
			// Whitespace in the format matches a run of whitespace (and,
			// as with Python's \s+, at least one character).
			for i+1 < len(format) && isSpace(format[i+1]) {
				i++
			}
			if pos >= len(text) || !isSpace(text[pos]) {
				return pos, noMatch()
			}
			for pos < len(text) && isSpace(text[pos]) {
				pos++
			}
			continue
		}
		if c != '%' {
			if pos < len(text) && (text[pos] == c || strings.EqualFold(text[pos:pos+1], format[i:i+1])) {
				pos++
				continue
			}
			return pos, noMatch()
		}
		if i+1 >= len(format) {
			return pos, errors.NewValueError("stray %% in format %s", object.ReprString(top))
		}
		i++
		d := format[i]
		var ok bool
		var v int
		switch d {
		case 'd':
			r.day, ok = num(2, 1, 31, true)
		case 'm':
			r.month, ok = num(2, 1, 12, false)
		case 'y':
			if v, ok = exact(2); ok {
				if v < 69 {
					r.year = 2000 + v
				} else {
					r.year = 1900 + v
				}
			}
		case 'Y':
			r.year, ok = exact(4)
		case 'H':
			r.hour, ok = num(2, 0, 23, false)
		case 'I':
			r.hour12, ok = num(2, 1, 12, true)
		case 'M':
			r.minute, ok = num(2, 0, 59, false)
		case 'S':
			r.second, ok = num(2, 0, 61, false)
		case 'f':
			start := pos
			for pos < len(text) && pos-start < 6 && text[pos] >= '0' && text[pos] <= '9' {
				pos++
			}
			if ok = pos > start; ok {
				digits := text[start:pos] + strings.Repeat("0", 6-(pos-start))
				r.micro, _ = strconv.Atoi(digits)
			}
		case 'j':
			r.yday, ok = num(3, 1, 366, false)
		case 'U':
			r.weekU, ok = num(2, 0, 53, false)
		case 'W':
			r.weekW, ok = num(2, 0, 53, false)
		case 'V':
			r.isoWeek, ok = num(2, 1, 53, false)
		case 'G':
			r.isoYear, ok = exact(4)
		case 'w':
			if v, ok = num(1, 0, 6, false); ok {
				r.wday = (v + 6) % 7
			}
		case 'u':
			if v, ok = num(1, 1, 7, false); ok {
				r.wday = v - 1
			}
		case 'a':
			if v, ok = name(shortDays[:]); ok {
				r.wday = (v + 6) % 7
			}
		case 'A':
			if v, ok = name(longDays[:]); ok {
				r.wday = (v + 6) % 7
			}
		case 'b', 'h':
			if v, ok = name(shortMonths[:]); ok {
				r.month = v + 1
			}
		case 'B':
			if v, ok = name(longMonths[:]); ok {
				r.month = v + 1
			}
		case 'p':
			if v, ok = name([]string{"AM", "PM"}); ok {
				r.ampm = v + 1
			}
		case 'z':
			ok = parseStrptimeOffset(text, &pos, r)
		case 'Z':
			localStd, _ := time.Date(r.year, 1, 1, 0, 0, 0, 0, time.Local).Zone()
			localDst, _ := time.Date(r.year, 7, 1, 0, 0, 0, 0, time.Local).Zone()
			start := pos
			if _, ok = name([]string{"UTC", "GMT", localStd, localDst}); ok {
				r.tzname = text[start:pos]
			}
		case 'c':
			pos, ok = strptimeSub(r, text, pos, "%a %b %d %H:%M:%S %Y", top)
		case 'x':
			pos, ok = strptimeSub(r, text, pos, "%m/%d/%y", top)
		case 'X':
			pos, ok = strptimeSub(r, text, pos, "%H:%M:%S", top)
		case '%':
			if ok = pos < len(text) && text[pos] == '%'; ok {
				pos++
			}
		default:
			return pos, errors.NewValueError("'%c' is a bad directive in format %s", d, object.ReprString(top))
		}
		if !ok {
			return pos, noMatch()
		}
	}
	return pos, nil
}

func strptimeSub(r *strptimeResult, text string, pos int, format, top string) (int, bool) {
	p, errObj := strptimeInto(r, text, pos, format, top, func() object.Object { return errors.NewValueError("no match") })
	return p, errObj == nil
}

// parseStrptimeOffset reads a %z offset: Z, or +HHMM, +HH:MM with optional
// seconds and microseconds.
func parseStrptimeOffset(text string, pos *int, r *strptimeResult) bool {
	p := *pos
	if p < len(text) && text[p] == 'Z' {
		*pos = p + 1
		r.hasOffset, r.offset = true, 0
		return true
	}
	if p >= len(text) || (text[p] != '+' && text[p] != '-') {
		return false
	}
	sign := 1
	if text[p] == '-' {
		sign = -1
	}
	p++
	two := func() (int, bool) {
		if p+2 > len(text) || text[p] < '0' || text[p] > '9' || text[p+1] < '0' || text[p+1] > '9' {
			return 0, false
		}
		v := int(text[p]-'0')*10 + int(text[p+1]-'0')
		p += 2
		return v, true
	}
	h, ok := two()
	if !ok {
		return false
	}
	colon := p < len(text) && text[p] == ':'
	if colon {
		p++
	}
	m, ok := two()
	if !ok || m > 59 {
		return false
	}
	s := 0
	if p < len(text) && (text[p] == ':') == colon && (colon || (text[p] >= '0' && text[p] <= '9')) {
		save := p
		if colon {
			p++
		}
		if v, ok := two(); ok && v <= 59 {
			s = v
		} else {
			p = save
		}
	}
	*pos = p
	r.hasOffset, r.offset = true, sign*(h*3600+m*60+s)
	return true
}

// strptimeDatetime builds the datetime strptime returns: aware (a fixed
// offset zone) when %z was parsed, naive otherwise.
func strptimeDatetime(r *strptimeResult) (time.Time, object.Object) {
	loc := time.Local
	if r.hasOffset {
		if r.offset <= -86400 || r.offset >= 86400 {
			return time.Time{}, errors.NewValueError("offset must be a timedelta strictly between -timedelta(hours=24) and timedelta(hours=24)")
		}
		k := tzKey{offset: r.offset}
		if r.tzname != "" {
			k.name = r.tzname
		}
		loc = tzLocation(k)
	}
	f := datetimeFields{r.year, r.month, r.day, r.hour, r.minute, r.second, r.micro}
	return f.time(loc)
}

// strptimeDirectives reports which directive letters a format uses.
func strptimeDirectives(format string) map[byte]bool {
	dirs := map[byte]bool{}
	for i := 0; i+1 < len(format); i++ {
		if format[i] == '%' {
			dirs[format[i+1]] = true
			i++
		}
	}
	return dirs
}
