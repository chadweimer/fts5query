package fts5query

type config struct {
	columns        []string
	prefixMatch    bool
	minTermLength  int
}

func defaultConfig() config {
	return config{
		columns:       nil,
		prefixMatch:   true,
		minTermLength: 1,
	}
}

// Option configures query parsing behavior.
type Option func(*config)

// WithColumns restricts matching to the specified FTS5 virtual table columns.
// In SQLite FTS5, this produces `{col1 col2} : (...)` syntax with safe expression
// parenthesization so the column filter correctly scopes to the entire query.
// Columns with empty or whitespace-only names are ignored.
func WithColumns(columns ...string) Option {
	return func(c *config) {
		cols := make([]string, 0, len(columns))
		for _, col := range columns {
			if col != "" {
				cols = append(cols, col)
			}
		}
		c.columns = cols
	}
}

// WithPrefixMatch controls whether unquoted bare words have a prefix wildcard `*`
// appended (e.g., "cookie" -> "cookie"*). Quoted phrases always match exact tokens.
// The default is true.
func WithPrefixMatch(enabled bool) Option {
	return func(c *config) {
		c.prefixMatch = enabled
	}
}

// WithMinTermLength sets the minimum character length for a bare search term to
// be considered valid. Terms shorter than this length are discarded.
// The default is 1 (all non-empty terms are kept).
func WithMinTermLength(length int) Option {
	return func(c *config) {
		if length < 1 {
			length = 1
		}
		c.minTermLength = length
	}
}
