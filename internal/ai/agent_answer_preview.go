package ai

import "strings"

func partialJSONStringField(raw, field string) string {
	needle := `"` + field + `"`
	start := strings.Index(raw, needle)
	if start < 0 {
		return ""
	}
	rest := raw[start+len(needle):]

	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = strings.TrimLeft(rest[colon+1:], " \t\r\n")
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:]

	var builder strings.Builder
	escaped := false
	for _, char := range rest {
		if escaped {
			switch char {
			case 'n':
				builder.WriteRune('\n')
			case 't':
				builder.WriteRune('\t')
			case 'r':
				builder.WriteRune('\r')
			case '"', '\\', '/':
				builder.WriteRune(char)
			case 'u':

				escaped = false
				return builder.String()
			default:
				builder.WriteRune(char)
			}
			escaped = false
			continue
		}
		switch char {
		case '\\':
			escaped = true
		case '"':

			return builder.String()
		default:
			builder.WriteRune(char)
		}
	}

	if escaped {
		return builder.String()
	}
	return builder.String()
}

func isFinalAnswerToolName(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasPrefix("final_answer", name) || strings.HasPrefix(name, "final_answer")
}
