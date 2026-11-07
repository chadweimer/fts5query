package fts5query_test

import (
	"testing"

	"github.com/chadweimer/fts5query"
)

func BenchmarkParse(b *testing.B) {
	queries := []struct {
		name string
		q    string
	}{
		{"SingleWord", "chocolate"},
		{"MultipleWords", "chocolate chip cookies"},
		{"QuotedPhrase", `"chocolate chip cookies"`},
		{"ComplexQuery", `recipe: 3/4 cup! "all-purpose flour" OR oatmeal -walnuts`},
	}

	for _, tc := range queries {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = fts5query.Parse(tc.q, fts5query.WithColumns("name", "ingredients"))
			}
		})
	}
}
