package conversion

import (
	"fmt"
	"strings"
	"testing"
)

func benchJSON(records int) string {
	var b strings.Builder
	b.WriteString(`{"items": [`)
	for i := 0; i < records; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id": %d, "name": "item-%d", "price": %d.5, "tags": ["a", "b", "c"], "meta": {"created": "2026-01-01", "owner": {"id": %d, "active": true}, "score": null}}`, i, i, i, i%97)
	}
	b.WriteString(`], "total": 1}`)
	return b.String()
}

func BenchmarkParseJSON(b *testing.B) {
	for _, n := range []int{10, 1000, 20000} {
		doc := benchJSON(n)
		b.Run(fmt.Sprintf("records=%d", n), func(b *testing.B) {
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ParseJSON(doc); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
