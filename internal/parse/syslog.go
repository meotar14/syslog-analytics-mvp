package parse

import (
	"strconv"
	"strings"
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

	if strings.HasPrefix(trimmed, "<") {
		if end := strings.Index(trimmed, ">"); end > 1 {
			if pri, err := strconv.Atoi(trimmed[1:end]); err == nil {
				msg.Facility = pri / 8
				msg.Severity = pri % 8
				msg.ParsedOK = true
			}
			trimmed = strings.TrimSpace(trimmed[end+1:])
		}
	}

	fields := strings.Fields(trimmed)
	if len(fields) >= 4 {
		hostIdx := 3
		if strings.Contains(fields[0], "T") && strings.Contains(fields[0], ":") {
			hostIdx = 1
		}
		if hostIdx < len(fields) {
			msg.Hostname = sanitizeToken(fields[hostIdx])
		}
		if hostIdx+1 < len(fields) {
			msg.Program = sanitizeProgram(fields[hostIdx+1])
		}
	}

	return msg
}

func sanitizeToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "[]")
	if value == "" {
		return "unknown"
	}
	return value
}

func sanitizeProgram(value string) string {
	value = strings.TrimSpace(value)
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
