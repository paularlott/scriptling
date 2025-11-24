package stdlib

import (
	"github.com/paularlott/scriptling/object"
	"time"
)

var startTime = time.Now()

func GetTimeLibrary() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"time": {
			Fn: func(args ...object.Object) object.Object {
				return &object.Float{Value: float64(time.Now().UnixNano()) / 1e9}
			},
		},
		"perf_counter": {
			Fn: func(args ...object.Object) object.Object {
				return &object.Float{Value: time.Since(startTime).Seconds()}
			},
		},
		"sleep": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "sleep() takes 1 argument"}
				}
				var seconds float64
				switch arg := args[0].(type) {
				case *object.Integer:
					seconds = float64(arg.Value)
				case *object.Float:
					seconds = arg.Value
				default:
					return &object.Error{Message: "sleep() argument must be number"}
				}
				time.Sleep(time.Duration(seconds * float64(time.Second)))
				return &object.Null{}
			},
		},
		"strftime": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: "strftime() takes 2 arguments"}
				}
				format, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "strftime() format must be string"}
				}
				var timestamp float64
				switch t := args[1].(type) {
				case *object.Integer:
					timestamp = float64(t.Value)
				case *object.Float:
					timestamp = t.Value
				default:
					return &object.Error{Message: "strftime() time must be number"}
				}
				t := time.Unix(int64(timestamp), 0)
				return &object.String{Value: t.Format(pythonToGoFormat(format.Value))}
			},
		},
		"strptime": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: "strptime() takes 2 arguments"}
				}
				str, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "strptime() string must be string"}
				}
				format, ok := args[1].(*object.String)
				if !ok {
					return &object.Error{Message: "strptime() format must be string"}
				}
				t, err := time.Parse(pythonToGoFormat(format.Value), str.Value)
				if err != nil {
					return &object.Error{Message: "strptime() parse error: " + err.Error()}
				}
				return &object.Float{Value: float64(t.Unix())}
			},
		},
	}
}

func pythonToGoFormat(pyFormat string) string {
	goFormat := pyFormat
	goFormat = replaceAll(goFormat, "%Y", "2006")
	goFormat = replaceAll(goFormat, "%m", "01")
	goFormat = replaceAll(goFormat, "%d", "02")
	goFormat = replaceAll(goFormat, "%H", "15")
	goFormat = replaceAll(goFormat, "%M", "04")
	goFormat = replaceAll(goFormat, "%S", "05")
	return goFormat
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}
