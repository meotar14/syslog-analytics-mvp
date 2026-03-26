package stats

import (
	"sync"
	"time"

	"syslog-analytics-mvp/internal/parse"
)

type Counter struct {
	MsgCount        int64 `json:"msg_count"`
	ByteCount       int64 `json:"byte_count"`
	ParsedOKCount   int64 `json:"parsed_ok_count"`
	ParsedFailCount int64 `json:"parsed_fail_count"`
}

type SourceKey struct {
	Minute   int64
	SourceIP string
	Hostname string
}

type DimKey struct {
	Minute int64
	Value  int
}

type SourceSummary struct {
	SourceIP  string `json:"source_ip"`
	Hostname  string `json:"hostname"`
	FirstSeen int64  `json:"first_seen_at"`
	LastSeen  int64  `json:"last_seen_at"`
	TotalMsgs int64  `json:"total_msgs"`
	TotalByte int64  `json:"total_bytes"`
}

type Snapshot struct {
	PerSecond         map[int64]Counter
	PerMinute         map[int64]Counter
	PerHour           map[int64]Counter
	PerDay            map[int64]Counter
	PerSourceMinute   map[SourceKey]Counter
	PerSeverityMinute map[DimKey]Counter
	PerFacilityMinute map[DimKey]Counter
	SourceRegistry    map[string]SourceSummary
}

type Collector struct {
	mu              sync.Mutex
	startedAt       time.Time
	perSecond       map[int64]Counter
	perMinute       map[int64]Counter
	perHour         map[int64]Counter
	perDay          map[int64]Counter
	perSourceMinute map[SourceKey]Counter
	perSeverity     map[DimKey]Counter
	perFacility     map[DimKey]Counter
	sourceRegistry  map[string]SourceSummary
}

func NewCollector() *Collector {
	return &Collector{
		startedAt:       time.Now().UTC(),
		perSecond:       map[int64]Counter{},
		perMinute:       map[int64]Counter{},
		perHour:         map[int64]Counter{},
		perDay:          map[int64]Counter{},
		perSourceMinute: map[SourceKey]Counter{},
		perSeverity:     map[DimKey]Counter{},
		perFacility:     map[DimKey]Counter{},
		sourceRegistry:  map[string]SourceSummary{},
	}
}

func (c *Collector) StartedAt() time.Time {
	return c.startedAt
}

func (c *Collector) Record(sourceIP string, parsed parse.Message) {
	now := time.Now().UTC()
	second := now.Unix()
	minute := now.Truncate(time.Minute).Unix()
	hour := now.Truncate(time.Hour).Unix()
	day := now.Truncate(24 * time.Hour).Unix()

	c.mu.Lock()
	defer c.mu.Unlock()

	inc := Counter{MsgCount: 1, ByteCount: int64(parsed.RawBytes)}
	if parsed.ParsedOK {
		inc.ParsedOKCount = 1
	} else {
		inc.ParsedFailCount = 1
	}

	c.perSecond[second] = addCounter(c.perSecond[second], inc)
	c.perMinute[minute] = addCounter(c.perMinute[minute], inc)
	c.perHour[hour] = addCounter(c.perHour[hour], inc)
	c.perDay[day] = addCounter(c.perDay[day], inc)

	sourceKey := SourceKey{
		Minute:   minute,
		SourceIP: sourceIP,
		Hostname: parsed.Hostname,
	}
	c.perSourceMinute[sourceKey] = addCounter(c.perSourceMinute[sourceKey], inc)

	if parsed.Severity >= 0 {
		key := DimKey{Minute: minute, Value: parsed.Severity}
		c.perSeverity[key] = addCounter(c.perSeverity[key], inc)
	}
	if parsed.Facility >= 0 {
		key := DimKey{Minute: minute, Value: parsed.Facility}
		c.perFacility[key] = addCounter(c.perFacility[key], inc)
	}

	reg := c.sourceRegistry[sourceIP]
	if reg.SourceIP == "" {
		reg = SourceSummary{
			SourceIP:  sourceIP,
			Hostname:  parsed.Hostname,
			FirstSeen: second,
		}
	}
	reg.Hostname = parsed.Hostname
	reg.LastSeen = second
	reg.TotalMsgs++
	reg.TotalByte += int64(parsed.RawBytes)
	c.sourceRegistry[sourceIP] = reg
}

func (c *Collector) Drain() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := Snapshot{
		PerSecond:         c.perSecond,
		PerMinute:         c.perMinute,
		PerHour:           c.perHour,
		PerDay:            c.perDay,
		PerSourceMinute:   c.perSourceMinute,
		PerSeverityMinute: c.perSeverity,
		PerFacilityMinute: c.perFacility,
		SourceRegistry:    c.sourceRegistry,
	}

	c.perSecond = map[int64]Counter{}
	c.perMinute = map[int64]Counter{}
	c.perHour = map[int64]Counter{}
	c.perDay = map[int64]Counter{}
	c.perSourceMinute = map[SourceKey]Counter{}
	c.perSeverity = map[DimKey]Counter{}
	c.perFacility = map[DimKey]Counter{}
	c.sourceRegistry = map[string]SourceSummary{}

	return snapshot
}

func (c *Collector) RestoreSource(reg SourceSummary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sourceRegistry[reg.SourceIP] = reg
}

func addCounter(base, inc Counter) Counter {
	base.MsgCount += inc.MsgCount
	base.ByteCount += inc.ByteCount
	base.ParsedOKCount += inc.ParsedOKCount
	base.ParsedFailCount += inc.ParsedFailCount
	return base
}
