package parse

import "testing"

func TestParseFortiGateKeyValuePayload(t *testing.T) {
	raw := `date=2026-05-25 time=12:01:02 devid="FGT123" devname="fw-edge" type="traffic" subtype="forward" level="notice" vd="root" srcip=10.0.0.10 srcport=52344 dstip=8.8.8.8 dstport=53 srcintf="lan" dstintf="wan" action="accept" policyid=42 sessionid=123456 proto=17 service="DNS" user="alice" app="DNS" appcat="Network.Service"`

	msg := Parse(raw)

	if msg.Vendor != "fortigate" {
		t.Fatalf("Vendor = %q, want fortigate", msg.Vendor)
	}
	if !msg.ParsedOK {
		t.Fatal("ParsedOK = false, want true")
	}
	if msg.Hostname != "fw-edge" {
		t.Fatalf("Hostname = %q, want fw-edge", msg.Hostname)
	}
	if msg.Severity != 5 {
		t.Fatalf("Severity = %d, want 5", msg.Severity)
	}
	if msg.FortiGate.Type != "traffic" || msg.FortiGate.Subtype != "forward" {
		t.Fatalf("FortiGate type/subtype = %q/%q, want traffic/forward", msg.FortiGate.Type, msg.FortiGate.Subtype)
	}
	if msg.FortiGate.SrcIP != "10.0.0.10" || msg.FortiGate.DstIP != "8.8.8.8" {
		t.Fatalf("FortiGate src/dst = %q/%q, want 10.0.0.10/8.8.8.8", msg.FortiGate.SrcIP, msg.FortiGate.DstIP)
	}
	if msg.FortiGate.PolicyID != 42 || msg.FortiGate.SessionID != 123456 {
		t.Fatalf("FortiGate policy/session = %d/%d, want 42/123456", msg.FortiGate.PolicyID, msg.FortiGate.SessionID)
	}
}

func TestParseFortiGateKeepsPRISeverity(t *testing.T) {
	raw := `<134>date=2026-05-25 time=12:01:02 devid="FGT123" devname="fw-edge" type="event" subtype="system" level="error" action="login"`

	msg := Parse(raw)

	if msg.Facility != 16 {
		t.Fatalf("Facility = %d, want 16", msg.Facility)
	}
	if msg.Severity != 6 {
		t.Fatalf("Severity = %d, want PRI severity 6", msg.Severity)
	}
	if msg.FortiGate.Level != "error" {
		t.Fatalf("FortiGate level = %q, want error", msg.FortiGate.Level)
	}
}
