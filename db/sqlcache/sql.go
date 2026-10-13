package sqlcache

import "strings"

const (
	readHead = 1 << iota
	unsafeRead
	sessionHead
	sessionToken
)

// Classify one identifier using stack space. Most SQL contains thousands of
// ordinary ASCII identifiers; neither those nor string literals need copies.
func tokenFlags(token string) int {
	var upper [32]byte
	if len(token) > len(upper) {
		if strings.EqualFold(token[:3], "PG_") {
			return unsafeRead
		}
		return 0
	}
	for i := range len(token) {
		c := token[i]
		if c >= 128 {
			return upperTokenFlags(strings.ToUpper(token))
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		upper[i] = c
	}
	return upperTokenFlags(string(upper[:len(token)]))
}

func upperTokenFlags(token string) int {
	switch token {
	case "SELECT", "WITH":
		return readHead
	case "SET", "RESET", "DISCARD", "BEGIN", "COMMIT", "ROLLBACK", "DO":
		return unsafeRead | sessionHead
	case "START":
		return sessionHead
	case "SET_CONFIG":
		return unsafeRead | sessionToken
	case "TEMP", "TEMPORARY":
		return sessionToken
	case "INSERT", "UPDATE", "DELETE", "MERGE", "INTO", "FOR", "LOCK", "COPY", "CALL", "NEXTVAL", "SETVAL", "CURRVAL", "LASTVAL", "CURRENT_SETTING", "RANDOM", "CLOCK_TIMESTAMP", "TXID_CURRENT", "GEN_RANDOM_UUID", "UUID_GENERATE_V4":
		return unsafeRead
	}
	if strings.HasPrefix(token, "PG_") {
		return unsafeRead
	}
	return 0
}

// Conservative PostgreSQL scan with the same exclusions for both direct and
// prepared queries. No token slice or copy of literal contents is retained.
func readOnly(query string) bool {
	first, readable := true, false
	valid := scanSQLTokens(query, func(token string) {
		flags := tokenFlags(token)
		if first {
			readable = flags&readHead != 0
			first = false
		}
		if flags&unsafeRead != 0 {
			readable = false
		}
	})
	return valid && readable
}

func scanSQLTokens(s string, emit func(string)) bool {
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
				return false
			}
			continue
		}
		if ended {
			return false
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
				return false
			}
			i++
			start := i
			doubled := false
			closed := false
			for i < len(s) {
				if s[i] == '\\' && escaped {
					i += 2
					continue
				}
				if s[i] == quote {
					i++
					if i < len(s) && s[i] == quote {
						doubled = true
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return false
			}
			if quote == '"' {
				value := s[start : i-1]
				if doubled {
					value = strings.ReplaceAll(value, `""`, `"`)
				}
				emit(value)
			}
			continue
		}
		if c == '$' {
			i++
			if i == len(s) || s[i] < '0' || s[i] > '9' {
				return false
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
			emit(s[start:i])
			continue
		}
		if c >= 128 {
			return false
		}
		i++
	}
	return true
}
