package parse

import (
	"strconv"
	"strings"
	"unicode"
)

type Message struct {
	Hostname string
	Program  string
	Severity int
	Facility int
	RawBytes int
	ParsedOK bool
}

func Parse(raw string) Message {
	msg := Message{
		Hostname: "unknown",
		Program:  "unknown",
		Severity: -1,
		Facility: -1,
		RawBytes: len(raw),
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return msg
	}

	payload, priParsed := parsePRI(trimmed, &msg)
	if priParsed {
		trimmed = payload
	}

	fields := strings.Fields(trimmed)
	switch {
	case looksLikeRFC5424(fields):
		msg.Hostname = sanitizeHost(fields[2])
		msg.Program = sanitizeProgram(fields[3])
		msg.ParsedOK = priParsed
	case looksLikeRFC3164(fields):
		msg.Hostname = sanitizeHost(fields[3])
		if len(fields) > 4 {
			msg.Program = sanitizeProgram(fields[4])
		}
		msg.ParsedOK = priParsed
	case len(fields) >= 2 && looksLikeTimestamp(fields[0]):
		msg.Hostname = sanitizeHost(fields[1])
		if len(fields) > 2 {
			msg.Program = sanitizeProgram(fields[2])
		}
		msg.ParsedOK = priParsed
	}

	return msg
}

func parsePRI(raw string, msg *Message) (string, bool) {
	if !strings.HasPrefix(raw, "<") {
		return raw, false
	}
	end := strings.Index(raw, ">")
	if end <= 1 {
		return raw, false
	}
	pri, err := strconv.Atoi(raw[1:end])
	if err != nil {
		return raw, false
	}
	msg.Facility = pri / 8
	msg.Severity = pri % 8
	return strings.TrimSpace(raw[end+1:]), true
}

func sanitizeToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "[]")
	value = strings.Trim(value, "-")
	if value == "" {
		return "unknown"
	}
	return value
}

func sanitizeHost(value string) string {
	value = sanitizeToken(value)
	if value == "" || value == "-" {
		return "unknown"
	}
	return value
}

func sanitizeProgram(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return "unknown"
	}
	if idx := strings.Index(value, ":"); idx > 0 {
		value = value[:idx]
	}
	if idx := strings.Index(value, "["); idx > 0 {
		value = value[:idx]
	}
	if value == "" {
		return "unknown"
	}
	return value
}

func looksLikeRFC5424(fields []string) bool {
	if len(fields) < 6 {
		return false
	}
	if !allDigits(fields[0]) {
		return false
	}
	return looksLikeTimestamp(fields[1])
}

func looksLikeRFC3164(fields []string) bool {
	if len(fields) < 4 {
		return false
	}
	return looksLikeMonth(fields[0]) && looksLikeClock(fields[2])
}

func looksLikeTimestamp(value string) bool {
	return strings.Contains(value, "T") && (strings.HasSuffix(value, "Z") || strings.Contains(value, "+") || strings.Count(value, ":") >= 2)
}

func looksLikeClock(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !allDigits(part) {
			return false
		}
	}
	return true
}

func looksLikeMonth(value string) bool {
	months := map[string]struct{}{
		"Jan": {}, "Feb": {}, "Mar": {}, "Apr": {}, "May": {}, "Jun": {},
		"Jul": {}, "Aug": {}, "Sep": {}, "Oct": {}, "Nov": {}, "Dec": {},
	}
	_, ok := months[value]
	return ok
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
