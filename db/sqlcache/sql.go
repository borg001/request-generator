package sqlcache

import "strings"

// Conservative PostgreSQL token scan. Quoted identifiers are also inspected so
// quoting a sequence/lock function cannot turn it into a cacheable read. Unknown
// syntax bypasses caching; this is not a SQL authorization/parser boundary.
func readOnly(query string) bool {
	tokens, ok := sqlTokens(query)
	if !ok || len(tokens) == 0 || (tokens[0] != "SELECT" && tokens[0] != "WITH") {
		return false
	}
	for _, token := range tokens {
		switch token {
		case "INSERT", "UPDATE", "DELETE", "MERGE", "INTO", "FOR", "LOCK", "COPY", "CALL", "DO", "SET", "RESET", "DISCARD", "BEGIN", "COMMIT", "ROLLBACK", "NEXTVAL", "SETVAL", "CURRVAL", "LASTVAL", "SET_CONFIG", "CURRENT_SETTING", "RANDOM", "CLOCK_TIMESTAMP", "TXID_CURRENT", "GEN_RANDOM_UUID", "UUID_GENERATE_V4":
			return false
		}
		if strings.HasPrefix(token, "PG_") {
			return false
		}
	}
	return true
}

func sqlTokens(s string) ([]string, bool) {
	var tokens []string
	ended := false
	for i := 0; i < len(s); {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			i++
			continue
		}
		if i+1 < len(s) && s[i:i+2] == "--" {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(s) && s[i:i+2] == "/*" {
			i += 2
			depth := 1
			for i < len(s) && depth > 0 {
				if i+1 < len(s) && s[i:i+2] == "/*" {
					depth++
					i += 2
				} else if i+1 < len(s) && s[i:i+2] == "*/" {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return nil, false
			}
			continue
		}
		if ended {
			return nil, false
		}
		if c == ';' {
			ended = true
			i++
			continue
		}
		if c == '\'' || c == '"' {
			quote := c
			// PostgreSQL standard_conforming_strings=on: backslashes in
			// ordinary strings (notably regular expressions) are literal.
			// Only E'...' uses backslash escaping for quote delimiters.
			escaped := quote == '\'' && i > 0 && (s[i-1] == 'E' || s[i-1] == 'e')
			if i > 0 && s[i-1] == '&' {
				return nil, false
			}
			i++
			var value strings.Builder
			closed := false
			for i < len(s) {
				if s[i] == '\\' && escaped {
					i += 2
					continue
				}
				if s[i] == quote {
					i++
					if i < len(s) && s[i] == quote {
						value.WriteByte(quote)
						i++
						continue
					}
					closed = true
					break
				}
				value.WriteByte(s[i])
				i++
			}
			if !closed {
				return nil, false
			}
			if quote == '"' {
				tokens = append(tokens, strings.ToUpper(value.String()))
			}
			continue
		}
		if c == '$' {
			i++
			if i == len(s) || s[i] < '0' || s[i] > '9' {
				return nil, false
			}
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			continue
		}
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			start := i
			i++
			for i < len(s) && (s[i] == '_' || s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' || s[i] >= '0' && s[i] <= '9') {
				i++
			}
			tokens = append(tokens, strings.ToUpper(s[start:i]))
			continue
		}
		if c >= 128 {
			return nil, false
		}
		i++
	}
	return tokens, true
}
