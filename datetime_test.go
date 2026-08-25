package scriptling

import (
	"testing"
	"time"

	"github.com/paularlott/scriptling/stdlib"
)

// setLocalZone overrides time.Local for the duration of the test so zone
// dependent behaviour is deterministic regardless of the host timezone.
func setLocalZone(t *testing.T, offsetHours int) {
	t.Helper()
	orig := time.Local
	time.Local = time.FixedZone("TESTZONE", offsetHours*3600)
	t.Cleanup(func() { time.Local = orig })
}

// evalDatetimeScript registers the datetime and time libraries, evaluates the
// script (with the library imports prepended) and fails the test on any
// evaluation error.
func evalDatetimeScript(t *testing.T, script string) *Scriptling {
	t.Helper()
	p := New()
	p.RegisterLibrary(stdlib.DatetimeLibrary)
	p.RegisterLibrary(stdlib.TimeLibrary)
	if _, err := p.Eval("import time\nimport datetime\n" + script); err != nil {
		t.Fatalf("eval error: %v", err)
	}
	return p
}

func assertResult(t *testing.T, p *Scriptling, want interface{}) {
	t.Helper()

	switch want := want.(type) {
	case string:
		got, objErr := p.GetVarAsString("result")
		if objErr != nil {
			t.Fatalf("result variable not found or not a string: %v", objErr)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	case bool:
		got, objErr := p.GetVarAsBool("result")
		if objErr != nil {
			t.Fatalf("result variable not found or not a bool: %v", objErr)
		}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	case float64:
		got, objErr := p.GetVarAsFloat("result")
		if objErr != nil {
			t.Fatalf("result variable not found or not a float: %v", objErr)
		}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	case int64:
		got, objErr := p.GetVarAsInt("result")
		if objErr != nil {
			t.Fatalf("result variable not found or not an int: %v", objErr)
		}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	default:
		t.Fatalf("unsupported expected type %T", want)
	}
}

func runDatetimeCases(t *testing.T, tests []struct {
	name   string
	script string
	want   interface{}
}) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := evalDatetimeScript(t, tt.script)
			assertResult(t, p, tt.want)
		})
	}
}

// TestDatetimeUTCPlus8 covers timezone handling with the local zone set to
// UTC+8: UTC-facing functions must report UTC, local-facing functions must
// report local. Fixed timestamps keep every assertion deterministic.
func TestDatetimeUTCPlus8(t *testing.T) {
	setLocalZone(t, 8)

	runDatetimeCases(t, []struct {
		name   string
		script string
		want   interface{}
	}{
		// time.gmtime reports UTC components of the epoch
		{"gmtime epoch", `
			g = time.gmtime(0)
			result = g[0]==1970 and g[1]==1 and g[2]==1 and g[3]==0 and g[4]==0 and g[5]==0 and g[6]==4 and g[7]==1
		`, true},
		// 1705314645 = 2024-01-15T10:30:45Z, a Monday (Go weekday 1)
		{"gmtime known timestamp", `
			g = time.gmtime(1705314645)
			result = g[0]==2024 and g[1]==1 and g[2]==15 and g[3]==10 and g[4]==30 and g[5]==45 and g[6]==1 and g[7]==15
		`, true},
		// time.localtime reports local components (UTC+8)
		{"localtime epoch", `
			l = time.localtime(0)
			result = l[0]==1970 and l[1]==1 and l[2]==1 and l[3]==8 and l[4]==0 and l[5]==0
		`, true},
		{"localtime known timestamp", `
			l = time.localtime(1705314645)
			result = l[2]==15 and l[3]==18 and l[4]==30 and l[5]==45
		`, true},
		{"strftime with gmtime tuple keeps tuple fields", `
			result = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(0))
		`, "1970-01-01 00:00:00"},
		{"time.strptime tuple keeps parsed fields", `
			g = time.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
			result = g[0]==2024 and g[1]==1 and g[2]==15 and g[3]==10 and g[4]==30 and g[5]==45
		`, true},
		{"mktime interprets tuple as local", `
			result = time.mktime([1970, 1, 1, 8, 0, 0, 4, 1, 0])
		`, 0.0},

		// fromtimestamp renders in local time
		{"fromtimestamp renders local", `
			result = datetime.datetime.fromtimestamp(0).strftime("%Y-%m-%dT%H:%M:%S")
		`, "1970-01-01T08:00:00"},
		{"fromtimestamp hour is local", `
			result = datetime.datetime.fromtimestamp(0).hour()
		`, int64(8)},

		// naive constructor interprets fields as local
		{"constructor timestamp is local", `
			result = datetime.datetime(1970, 1, 1, 12, 0, 0).timestamp()
		`, 14400.0},
		{"constructor isoformat keeps fields", `
			result = datetime.datetime(2024, 1, 15, 10, 30).isoformat()
		`, "2024-01-15T10:30:00"},
		{"constructor kwargs", `
			result = datetime.datetime(2024, 1, 15, hour=10, minute=30, second=45).isoformat()
		`, "2024-01-15T10:30:45"},

		// utcnow must render UTC components: compare against the UTC tuple of
		// the instance's own timestamp (same instant, so exact match)
		{"utcnow renders UTC components", `
			u = datetime.datetime.utcnow()
			g = time.gmtime(u.timestamp())
			result = u.year()==g[0] and u.month()==g[1] and u.day()==g[2] and u.hour()==g[3] and u.minute()==g[4] and u.second()==g[5]
		`, true},
		// now must render local components
		{"now renders local components", `
			n = datetime.datetime.now()
			l = time.localtime(n.timestamp())
			result = n.year()==l[0] and n.month()==l[1] and n.day()==l[2] and n.hour()==l[3] and n.minute()==l[4] and n.second()==l[5]
		`, true},
		// now and utcnow describe the same instant
		{"now and utcnow same instant", `
			d = datetime.datetime.now().timestamp() - datetime.datetime.utcnow().timestamp()
			result = d < 5 and d > -5
		`, true},

		// strptime is naive: fields round-trip exactly regardless of local zone
		{"strptime round trip", `
			result = datetime.datetime.strptime("2024-01-15T23:51:33", "%Y-%m-%dT%H:%M:%S").strftime("%Y-%m-%dT%H:%M:%S")
		`, "2024-01-15T23:51:33"},
		{"strptime isoformat keeps fields", `
			result = datetime.datetime.strptime("1970-01-01T00:30:00", "%Y-%m-%dT%H:%M:%S").isoformat()
		`, "1970-01-01T00:30:00"},
		{"strptime timestamp is UTC fields", `
			result = datetime.datetime.strptime("1970-01-01T00:30:00", "%Y-%m-%dT%H:%M:%S").timestamp()
		`, 1800.0},
		{"strptime field accessors", `
			s = datetime.datetime.strptime("1970-01-01T00:30:00", "%Y-%m-%dT%H:%M:%S")
			result = s.year() == 1970 and s.hour() == 0 and s.minute() == 30
		`, true},
		{"strptime weekday", `
			s = datetime.datetime.strptime("2024-01-15T00:00:00", "%Y-%m-%dT%H:%M:%S")
			result = s.weekday() == 0 and s.isoweekday() == 1
		`, true},

		// arithmetic and replace must preserve the zone of their instance
		{"utc strptime plus timedelta", `
			s = datetime.datetime.strptime("2024-01-15T00:00:05", "%Y-%m-%dT%H:%M:%S")
			result = (s + datetime.timedelta(days=-1)).isoformat()
		`, "2024-01-14T00:00:05"},
		{"utc strptime minus timedelta", `
			s = datetime.datetime.strptime("2024-01-15T00:00:05", "%Y-%m-%dT%H:%M:%S")
			result = (s - datetime.timedelta(days=1)).isoformat()
		`, "2024-01-14T00:00:05"},
		{"replace on utc instance stays utc", `
			s = datetime.datetime.strptime("1970-01-01T00:30:00", "%Y-%m-%dT%H:%M:%S")
			result = s.replace(hour=9).isoformat()
		`, "1970-01-01T09:30:00"},
		{"replace on utc instance keeps utc timestamp", `
			s = datetime.datetime.strptime("1970-01-01T00:30:00", "%Y-%m-%dT%H:%M:%S")
			result = s.replace(hour=9).timestamp()
		`, 34200.0},
		{"replace on local instance stays local", `
			f = datetime.datetime.fromtimestamp(0)
			result = f.replace(hour=9).strftime("%Y-%m-%dT%H:%M:%S")
		`, "1970-01-01T09:00:00"},
		{"utcnow replace renders utc", `
			u = datetime.datetime.utcnow().replace(hour=7, minute=0, second=0, microsecond=0)
			g = time.gmtime(u.timestamp())
			result = u.hour() == 7 and g[3] == u.hour() and g[2] == u.day()
		`, true},

		// comparisons and subtraction are instant based
		{"datetime subtraction seconds", `
			a = datetime.datetime.strptime("2024-01-15T10:00:00", "%Y-%m-%dT%H:%M:%S")
			b = datetime.datetime.strptime("2024-01-15T09:00:00", "%Y-%m-%dT%H:%M:%S")
			result = a - b
		`, 3600.0},
		{"datetime minus seconds int", `
			s = datetime.datetime.strptime("2024-01-15T10:00:00", "%Y-%m-%dT%H:%M:%S")
			result = (s - 3600).isoformat()
		`, "2024-01-15T09:00:00"},
		{"datetime minus seconds float", `
			s = datetime.datetime.strptime("2024-01-15T10:00:00", "%Y-%m-%dT%H:%M:%S")
			result = (s - 3600.0).isoformat()
		`, "2024-01-15T09:00:00"},
		{"datetime minus timedelta", `
			s = datetime.datetime.strptime("2024-01-15T10:00:00", "%Y-%m-%dT%H:%M:%S")
			result = (s - datetime.timedelta(hours=1)).isoformat()
		`, "2024-01-15T09:00:00"},
		{"datetime augmented add seconds", `
			s = datetime.datetime.strptime("2024-01-15T10:00:00", "%Y-%m-%dT%H:%M:%S")
			s += 3600
			result = s.isoformat()
		`, "2024-01-15T11:00:00"},
		{"datetime augmented sub seconds", `
			s = datetime.datetime.strptime("2024-01-15T10:00:00", "%Y-%m-%dT%H:%M:%S")
			s -= 3600
			result = s.isoformat()
		`, "2024-01-15T09:00:00"},
		{"datetime comparison across zones", `
			a = datetime.datetime.strptime("1970-01-01T00:00:00", "%Y-%m-%dT%H:%M:%S")
			b = datetime.datetime.fromtimestamp(0)
			result = a == b
		`, true},
		{"datetime ordering", `
			a = datetime.datetime.strptime("2024-01-15T10:00:00", "%Y-%m-%dT%H:%M:%S")
			b = datetime.datetime.strptime("2024-01-15T09:00:00", "%Y-%m-%dT%H:%M:%S")
			result = a > b and b < a and a >= a and b <= b and a != b
		`, true},

		// module level strftime formats timestamps in local time
		{"module strftime local timestamp", `
			result = datetime.datetime.strftime("%H", 0)
		`, "08"},

		// date class
		{"date isoformat", `
			result = datetime.date(2024, 1, 15).isoformat()
		`, "2024-01-15"},
		{"date subtraction days", `
			result = datetime.date(2024, 1, 22) - datetime.date(2024, 1, 15)
		`, int64(7)},
		{"date addition", `
			result = datetime.date(2024, 1, 15) + 7 == datetime.date(2024, 1, 22)
		`, true},
		{"date addition via timedelta", `
			result = datetime.date(2024, 1, 15) + datetime.timedelta(weeks=1) == datetime.date(2024, 1, 22)
		`, true},
		{"date subtraction via days", `
			result = datetime.date(2024, 1, 22) - 7 == datetime.date(2024, 1, 15)
		`, true},
		{"date subtraction via timedelta", `
			result = datetime.date(2024, 1, 22) - datetime.timedelta(weeks=1) == datetime.date(2024, 1, 15)
		`, true},
		{"date augmented sub days", `
			d = datetime.date(2024, 1, 22)
			d -= 7
			result = d == datetime.date(2024, 1, 15)
		`, true},
		{"date replace", `
			result = datetime.date(2024, 1, 15).replace(year=2025, month=6).isoformat()
		`, "2025-06-15"},
		{"date weekday", `
			d = datetime.date(2024, 1, 15)
			result = d.weekday() == 0 and d.isoweekday() == 1
		`, true},
		{"date today isoformat shape", `
			d = datetime.date.today()
			s = d.isoformat()
			result = len(s) == 10 and s[4] == "-" and s[7] == "-"
		`, true},

		// timedelta totals
		{"timedelta composite", `
			result = datetime.timedelta(days=1, hours=2, minutes=30)
		`, 95400.0},
		{"timedelta weeks", `
			result = datetime.timedelta(weeks=1)
		`, 604800.0},
		{"timedelta negative", `
			result = datetime.timedelta(days=-1)
		`, -86400.0},
		{"timedelta microseconds", `
			result = datetime.timedelta(microseconds=500000)
		`, 0.5},
	})
}

// TestDatetimeUTCMinus5 repeats the zone sensitive assertions with the local
// zone west of UTC to catch sign errors.
func TestDatetimeUTCMinus5(t *testing.T) {
	setLocalZone(t, -5)

	runDatetimeCases(t, []struct {
		name   string
		script string
		want   interface{}
	}{
		{"gmtime epoch hour", `
			result = time.gmtime(0)[3]
		`, int64(0)},
		{"localtime epoch", `
			l = time.localtime(0)
			result = l[1] == 12 and l[2] == 31 and l[3] == 19
		`, true},
		{"fromtimestamp renders local", `
			result = datetime.datetime.fromtimestamp(0).strftime("%Y-%m-%dT%H:%M")
		`, "1969-12-31T19:00"},
		{"constructor timestamp is local", `
			result = datetime.datetime(1970, 1, 1, 0, 0, 0).timestamp()
		`, 18000.0},
		{"strptime round trip", `
			result = datetime.datetime.strptime("2024-01-15T23:51:33", "%Y-%m-%dT%H:%M:%S").strftime("%Y-%m-%dT%H:%M:%S")
		`, "2024-01-15T23:51:33"},
		{"utcnow renders UTC components", `
			u = datetime.datetime.utcnow()
			g = time.gmtime(u.timestamp())
			result = u.year()==g[0] and u.month()==g[1] and u.day()==g[2] and u.hour()==g[3] and u.minute()==g[4] and u.second()==g[5]
		`, true},
		{"now renders local components", `
			n = datetime.datetime.now()
			l = time.localtime(n.timestamp())
			result = n.year()==l[0] and n.month()==l[1] and n.day()==l[2] and n.hour()==l[3] and n.minute()==l[4] and n.second()==l[5]
		`, true},
	})
}

// TestDatetimeLocalUTC covers the degenerate case where the local zone is UTC.
func TestDatetimeLocalUTC(t *testing.T) {
	setLocalZone(t, 0)

	runDatetimeCases(t, []struct {
		name   string
		script string
		want   interface{}
	}{
		{"localtime equals gmtime at epoch", `
			result = time.localtime(0)[3] == time.gmtime(0)[3]
		`, true},
		{"fromtimestamp renders utc", `
			result = datetime.datetime.fromtimestamp(0).strftime("%H")
		`, "00"},
		{"strptime round trip", `
			result = datetime.datetime.strptime("2024-01-15T23:51:33", "%Y-%m-%dT%H:%M:%S").strftime("%Y-%m-%dT%H:%M:%S")
		`, "2024-01-15T23:51:33"},
		{"utcnow renders UTC components", `
			u = datetime.datetime.utcnow()
			g = time.gmtime(u.timestamp())
			result = u.hour() == g[3] and u.minute() == g[4]
		`, true},
	})
}

// TestDatetimeHostTimezone runs the critical invariants against whatever zone
// the host is configured with, since CI machines vary.
func TestDatetimeHostTimezone(t *testing.T) {
	runDatetimeCases(t, []struct {
		name   string
		script string
		want   interface{}
	}{
		{"gmtime epoch is midnight UTC", `
			g = time.gmtime(0)
			result = g[0]==1970 and g[1]==1 and g[2]==1 and g[3]==0 and g[4]==0 and g[5]==0
		`, true},
		{"utcnow matches gmtime of same instant", `
			u = datetime.datetime.utcnow()
			g = time.gmtime(u.timestamp())
			result = u.year()==g[0] and u.month()==g[1] and u.day()==g[2] and u.hour()==g[3] and u.minute()==g[4] and u.second()==g[5]
		`, true},
		{"now matches localtime of same instant", `
			n = datetime.datetime.now()
			l = time.localtime(n.timestamp())
			result = n.year()==l[0] and n.month()==l[1] and n.day()==l[2] and n.hour()==l[3] and n.minute()==l[4] and n.second()==l[5]
		`, true},
		{"strptime round trip", `
			result = datetime.datetime.strptime("2024-01-15T23:51:33", "%Y-%m-%dT%H:%M:%S").strftime("%Y-%m-%dT%H:%M:%S")
		`, "2024-01-15T23:51:33"},
		{"utcnow replace renders utc", `
			u = datetime.datetime.utcnow().replace(hour=7, minute=0, second=0, microsecond=0)
			g = time.gmtime(u.timestamp())
			result = u.hour() == 7 and g[3] == u.hour() and g[2] == u.day()
		`, true},
	})
}
