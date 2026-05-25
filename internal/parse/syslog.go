package parse

import (
	"strconv"
	"strings"
	"unicode"
)

type Message struct {
	Hostname  string
	Program   string
	Severity  int
	Facility  int
	Vendor    string
	FortiGate FortiGateFields
	RawMessage string
	RawBytes   int
	ParsedOK   bool
}

type FortiGateFields struct {
	Type      string
	Subtype   string
	Level     string
	Action    string
	VD        string
	SrcIP     string
	SrcPort   int
	DstIP     string
	DstPort   int
	SrcIntf   string
	DstIntf   string
	PolicyID  int64
	SessionID int64
	Proto     int
	Service   string
	UserName  string
	App       string
	AppCat    string
}

func Parse(raw string) Message {
	msg := Message{
		Hostname:   "unknown",
		Program:    "unknown",
		Severity:   -1,
		Facility:   -1,
		Vendor:     "generic",
		RawMessage: strings.TrimSpace(raw),
		RawBytes:   len(raw),
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

	parseFortiGate(trimmed, &msg)
	return msg
}

func parseFortiGate(payload string, msg *Message) {
	values := parseKeyValues(payload)
	if len(values) == 0 {
		return
	}
	if _, ok := values["devid"]; !ok {
		if _, ok := values["devname"]; !ok {
			if _, ok := values["subtype"]; !ok {
				return
			}
		}
	}

	msg.Vendor = "fortigate"
	msg.ParsedOK = true
	if msg.Hostname == "unknown" {
		if devname := values["devname"]; devname != "" {
			msg.Hostname = sanitizeHost(devname)
		}
	}
	if msg.Severity < 0 {
		msg.Severity = fortiGateSeverity(values["level"])
	}
	msg.FortiGate = FortiGateFields{
		Type:      values["type"],
		Subtype:   values["subtype"],
		Level:     values["level"],
		Action:    values["action"],
		VD:        values["vd"],
		SrcIP:     values["srcip"],
		SrcPort:   atoiDefault(values["srcport"], 0),
		DstIP:     values["dstip"],
		DstPort:   atoiDefault(values["dstport"], 0),
		SrcIntf:   values["srcintf"],
		DstIntf:   values["dstintf"],
		PolicyID:  atoi64Default(values["policyid"], 0),
		SessionID: atoi64Default(values["sessionid"], 0),
		Proto:     atoiDefault(values["proto"], 0),
		Service:   values["service"],
		UserName:  firstNonEmpty(values["user"], values["unauthuser"]),
		App:       values["app"],
		AppCat:    values["appcat"],
	}
}

func parseKeyValues(payload string) map[string]string {
	values := map[string]string{}
	for i := 0; i < len(payload); {
		for i < len(payload) && unicode.IsSpace(rune(payload[i])) {
			i++
		}
		start := i
		for i < len(payload) && payload[i] != '=' && !unicode.IsSpace(rune(payload[i])) {
			i++
		}
		if i >= len(payload) || payload[i] != '=' || i == start {
			for i < len(payload) && !unicode.IsSpace(rune(payload[i])) {
				i++
			}
			continue
		}
		key := strings.ToLower(payload[start:i])
		i++
		valueStart := i
		var value string
		if i < len(payload) && payload[i] == '"' {
			i++
			valueStart = i
			for i < len(payload) && payload[i] != '"' {
				i++
			}
			value = payload[valueStart:i]
			if i < len(payload) && payload[i] == '"' {
				i++
			}
		} else {
			for i < len(payload) && !unicode.IsSpace(rune(payload[i])) {
				i++
			}
			value = payload[valueStart:i]
		}
		if key != "" {
			values[key] = value
		}
	}
	return values
}

func fortiGateSeverity(level string) int {
	switch strings.ToLower(level) {
	case "emergency":
		return 0
	case "alert":
		return 1
	case "critical", "crit":
		return 2
	case "error", "err":
		return 3
	case "warning", "warn":
		return 4
	case "notice":
		return 5
	case "information", "informational", "info":
		return 6
	case "debug":
		return 7
	default:
		return -1
	}
}

func atoiDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func atoi64Default(value string, fallback int64) int64 {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
