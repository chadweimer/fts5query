package fts5query

import (
	"strings"
	"unicode"
)

type rawToken struct {
	text      string
	isQuoted  bool
	isNegated bool
}

func parseTokens(input string, cfg config) (positive, negative []string) {
	tokens := tokenize(input)
	pendingOR := false

	for _, tok := range tokens {
		if !tok.isQuoted && !tok.isNegated && tok.text == "OR" {
			if len(positive) > 0 {
				pendingOR = true
			}
			continue
		}

		formatted := formatToken(tok, cfg)
		if formatted == "" {
			if tok.isNegated {
				pendingOR = false
			}
			continue
		}

		if tok.isNegated {
			pendingOR = false
			negative = append(negative, formatted)
			continue
		}

		if pendingOR {
			positive = append(positive, "OR")
			pendingOR = false
		}
		positive = append(positive, formatted)
	}

	return positive, negative
}

func tokenize(input string) []rawToken {
	runes := []rune(input)
	var tokens []rawToken

	for i := skipSpaces(runes, 0); i < len(runes); i = skipSpaces(runes, i) {
		var tok rawToken
		tok, i = readNextToken(runes, i)
		tokens = append(tokens, tok)
	}

	return tokens
}

func skipSpaces(runes []rune, i int) int {
	for i < len(runes) && unicode.IsSpace(runes[i]) {
		i++
	}
	return i
}

func readNextToken(runes []rune, i int) (rawToken, int) {
	isNegated := false
	if runes[i] == '-' && i+1 < len(runes) && !unicode.IsSpace(runes[i+1]) {
		isNegated = true
		i++
	}

	if i < len(runes) && runes[i] == '"' {
		text, next := readQuotedText(runes, i+1)
		return rawToken{text: text, isQuoted: true, isNegated: isNegated}, next
	}

	text, next := readBareText(runes, i)
	return rawToken{text: text, isQuoted: false, isNegated: isNegated}, next
}

func readQuotedText(runes []rune, start int) (string, int) {
	i := start
	for i < len(runes) && runes[i] != '"' {
		i++
	}
	text := string(runes[start:i])
	if i < len(runes) && runes[i] == '"' {
		i++
	}
	return text, i
}

func readBareText(runes []rune, start int) (string, int) {
	i := start
	for i < len(runes) && !unicode.IsSpace(runes[i]) && runes[i] != '"' {
		i++
	}
	return string(runes[start:i]), i
}

func formatToken(tok rawToken, cfg config) string {
	if tok.isQuoted {
		phrase := cleanPhrase(tok.text)
		if phrase == "" {
			return ""
		}
		return `"` + phrase + `"`
	}

	term := cleanTerm(tok.text)
	if term == "" || len([]rune(term)) < cfg.minTermLength {
		return ""
	}

	if cfg.prefixMatch {
		return `"` + term + `"*`
	}
	return `"` + term + `"`
}

func cleanPhrase(phrase string) string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case 0, '"', ':', '*', '^', '{', '}', '(', ')':
			return ' '
		default:
			return r
		}
	}, phrase)
	clean = strings.TrimSpace(clean)
	if !strings.ContainsFunc(clean, isAlphanumeric) {
		return ""
	}
	return clean
}

func cleanTerm(term string) string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case 0, '"', ':', '*', '^', '{', '}', '(', ')', '~', '+', '&', '|':
			return -1
		default:
			return r
		}
	}, term)
	return strings.TrimFunc(clean, func(r rune) bool {
		return !isAlphanumeric(r)
	})
}

func isAlphanumeric(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}
