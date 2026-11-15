package yaml

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
	"gopkg.in/yaml.v3"
)

func randomYAMLString(rng *rand.Rand) string {
	pieces := []string{"a", "Z", "x1", "_", " ", "  ", "\n", "\n\n", "\t", "\r", ":", ": ", " #", "#", "-", "- ", "?", "'", "\"",
		"\\", "{", "}", "[", "]", ",", "&", "*", "!", "|", ">", "%", "@", "`", "é", "中", "😀", " ", "\x01", "\x7f",
		"\xff", "true", "yes", "No", "null", "~", "1", "0x1F", "1.5", "1e3", ".inf", "12:30", "<<", "...", "---", "=", "0o7"}
	var b strings.Builder
	for n := rng.Intn(6); n > 0; n-- {
		b.WriteString(pieces[rng.Intn(len(pieces))])
	}
	if rng.Intn(40) == 0 {
		b.WriteString(strings.Repeat("word ", rng.Intn(40)))
	}
	return b.String()
}

func randomYAMLValue(rng *rand.Rand, depth int) object.Object {
	k := rng.Intn(10)
	if depth > 4 {
		k = rng.Intn(6)
	}
	switch k {
	case 0:
		return object.NewInteger(rng.Int63n(2000) - 1000)
	case 1:
		fs := []float64{0, 1.5, -2.25, 1e20, 1e-7, math.Inf(1), math.Inf(-1), math.NaN(), 3, rng.NormFloat64()}
		return object.NewFloat(fs[rng.Intn(len(fs))])
	case 2, 3:
		return object.NewString(randomYAMLString(rng))
	case 4:
		return object.NewBoolean(rng.Intn(2) == 0)
	case 5:
		if rng.Intn(3) == 0 {
			return object.NewBytes([]byte(randomYAMLString(rng)))
		}
		return &object.Null{}
	case 6, 7:
		n := rng.Intn(4)
		elems := make([]object.Object, n)
		for i := range elems {
			elems[i] = randomYAMLValue(rng, depth+1)
		}
		if k == 6 {
			return &object.List{Elements: elems}
		}
		return &object.Tuple{Elements: elems}
	default:
		d := object.NewStringDict(nil)
		for n := rng.Intn(5); n > 0; n-- {
			key := randomYAMLString(rng)
			if rng.Intn(3) == 0 {
				key = []string{"a", "b", "a10", "a2", "10", "9", "B", "_x", "x_1", "x01", "x1"}[rng.Intn(11)]
			}
			d.SetByString(key, randomYAMLValue(rng, depth+1))
		}
		return d
	}
}

// TestYAMLEmitterMatchesMarshal checks yaml.dump writes exactly what
// yaml.Marshal wrote for the converted value, which it used before.
func TestYAMLEmitterMatchesMarshal(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 20000; i++ {
		v := randomYAMLValue(rng, 0)
		want, err := yaml.Marshal(conversion.ToGo(v))
		if err != nil {
			continue
		}
		got := yamlDumpFunc(context.Background(), object.NewKwargs(nil), v)
		s, ok := got.(*object.String)
		if !ok {
			t.Fatalf("dump(%s): %s", v.Inspect(), got.Inspect())
		}
		if s.StringValue() != string(want) {
			t.Fatalf("dump(%s)\n got: %q\nwant: %q", v.Inspect(), s.StringValue(), want)
		}
	}
}

// TestYAMLPlainSafe checks the shortcut for plain strings agrees with
// yaml.v3.
func TestYAMLPlainSafe(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	alphabet := "abyYnNoOeEfFtTuUlLrsx019_  "
	for i := 0; i < 200000; i++ {
		var b strings.Builder
		for n := 1 + rng.Intn(7); n > 0; n-- {
			b.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		s := b.String()
		if !plainSafe(s) {
			continue
		}
		out, _ := yaml.Marshal(s)
		if string(out) != s+"\n" {
			t.Fatalf("plainSafe(%q) but yaml.v3 writes %q", s, out)
		}
	}
}
