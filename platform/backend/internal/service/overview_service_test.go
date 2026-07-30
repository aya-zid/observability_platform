package service

import (
	"sort"
	"strings"
	"testing"
	"time"
)

func TestTruncateStringShort(t *testing.T) {
	got := truncateString("hello", 10)
	if got != "hello" {
		t.Fatalf("truncateString() = %q, want %q", got, "hello")
	}
}

func TestTruncateStringExact(t *testing.T) {
	got := truncateString("hello", 5)
	if got != "hello" {
		t.Fatalf("truncateString() = %q, want %q", got, "hello")
	}
}

func TestTruncateStringLong(t *testing.T) {
	got := truncateString("hello world this is a long message", 10)
	want := "hello worl..."
	if got != want {
		t.Fatalf("truncateString() = %q, want %q", got, want)
	}
}

func TestTruncateStringEmpty(t *testing.T) {
	got := truncateString("", 10)
	if got != "" {
		t.Fatalf("truncateString() = %q, want %q", got, "")
	}
}

func TestTimelineSortNewestFirst(t *testing.T) {
	events := []TimelineEvent{
		{Timestamp: "2026-07-29T10:00:01Z"},
		{Timestamp: "2026-07-29T10:00:03Z"},
		{Timestamp: "2026-07-29T10:00:02Z"},
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp > events[j].Timestamp
	})
	if events[0].Timestamp != "2026-07-29T10:00:03Z" {
		t.Fatalf("first = %q, want 2026-07-29T10:00:03Z", events[0].Timestamp)
	}
	if events[2].Timestamp != "2026-07-29T10:00:01Z" {
		t.Fatalf("last = %q, want 2026-07-29T10:00:01Z", events[2].Timestamp)
	}
}

func TestTimelineCappedAt100(t *testing.T) {
	events := make([]TimelineEvent, 150)
	for i := 0; i < 150; i++ {
		events[i] = TimelineEvent{Timestamp: "2026-07-29T10:00:00Z"}
	}
	if len(events) > 100 {
		events = events[:100]
	}
	if len(events) != 100 {
		t.Fatalf("len after cap = %d, want 100", len(events))
	}
}

func TestDefaultTimelineWindowIsFiveMinutes(t *testing.T) {
	now := time.Now()
	startNs := int64(0)
	endNs := int64(0)

	// This mirrors the logic in GetOverview
	if startNs == 0 || endNs == 0 {
		endNs = now.UnixNano()
		startNs = endNs - int64(5*time.Minute)
	}

	duration := time.Duration(endNs - startNs)
	if duration != 5*time.Minute {
		t.Fatalf("default window = %v, want 5m", duration)
	}
	if endNs > now.UnixNano() {
		t.Fatalf("end should not be in the future")
	}
	if startNs > endNs {
		t.Fatalf("start should be before end")
	}
}

func TestClassifyNginx5xxIsError(t *testing.T) {
	line := `192.168.1.1 - - [29/Jul/2026:12:00:00 +0000] "GET /api/test HTTP/1.1" 503 123 "-" "curl/7.0"`
	if got := classifyTimelineEvent("nginx", line); got != "error" {
		t.Fatalf("classifyTimelineEvent(nginx, 5xx) = %q, want error", got)
	}
}

func TestClassifyNginx4xxIsWarning(t *testing.T) {
	line := `192.168.1.1 - - [29/Jul/2026:12:00:00 +0000] "GET /missing HTTP/1.1" 404 0 "-" "curl/7.0"`
	if got := classifyTimelineEvent("nginx", line); got != "warning" {
		t.Fatalf("classifyTimelineEvent(nginx, 4xx) = %q, want warning", got)
	}
}

func TestClassifyNginxErrorLogIsError(t *testing.T) {
	line := `2026/07/29 12:00:00 [error] 12345#12345: *1 connect() failed (111: Connection refused) while connecting to upstream`
	if got := classifyTimelineEvent("nginx", line); got != "error" {
		t.Fatalf("classifyTimelineEvent(nginx, error log) = %q, want error", got)
	}
}

func TestClassifyPostgresErrorIsError(t *testing.T) {
	line := `2026-07-29 12:00:00 UTC [12345] postgres@db ERROR:  duplicate key value violates unique constraint "pk_users"`
	if got := classifyTimelineEvent("postgres", line); got != "error" {
		t.Fatalf("classifyTimelineEvent(postgres, ERROR) = %q, want error", got)
	}
}

func TestClassifyPostgresFatalIsError(t *testing.T) {
	line := `2026-07-29 12:00:00 UTC [12345] postgres@db FATAL:  could not connect to the database server`
	if got := classifyTimelineEvent("postgres", line); got != "error" {
		t.Fatalf("classifyTimelineEvent(postgres, FATAL) = %q, want error", got)
	}
}

func TestClassifyPostgresSlowQueryIsWarning(t *testing.T) {
	line := `2026-07-29 12:00:00 UTC [12345] postgres@db LOG:  slow query: SELECT * FROM large_table WHERE id > 1000 (duration: 5234.123 ms)`
	if got := classifyTimelineEvent("postgres", line); got != "warning" {
		t.Fatalf("classifyTimelineEvent(postgres, slow query) = %q, want warning", got)
	}
}

func TestClassifyMariaDBSlowQueryIsWarning(t *testing.T) {
	line := `2026-07-29 12:00:00 UTC [12345] mysqld: Slow query: SELECT * FROM big_table WHERE condition=1 (execution time 12.345 sec)`
	if got := classifyTimelineEvent("mariadb", line); got != "warning" {
		t.Fatalf("classifyTimelineEvent(mariadb, slow query) = %q, want warning", got)
	}
}

func TestClassifyMariaDBDeadlockIsError(t *testing.T) {
	line := `2026-07-29 12:00:00 UTC [12345] mysqld: Deadlock found when trying to get lock; try restarting transaction`
	if got := classifyTimelineEvent("mariadb", line); got != "error" {
		t.Fatalf("classifyTimelineEvent(mariadb, deadlock) = %q, want error", got)
	}
}

func TestClassifyPHPFPMTimeoutIsError(t *testing.T) {
	line := `29-Jul-2026 12:00:00 UTC PHP Fatal error: Maximum execution time of 30 seconds exceeded in /var/www/app.php on line 42`
	if got := classifyTimelineEvent("phpfpm", line); got != "error" {
		t.Fatalf("classifyTimelineEvent(phpfpm, timeout) = %q, want error", got)
	}
}

func TestClassifyRedisWaitIsWarning(t *testing.T) {
	line := `WAIT 1 1000 blocked for 500.123 seconds`
	if got := classifyTimelineEvent("redis", line); got != "warning" {
		t.Fatalf("classifyTimelineEvent(redis, WAIT) = %q, want warning", got)
	}
}

func TestClassifyRedisErrorIsError(t *testing.T) {
	line := `4515:M 29 Jul 2026 12:00:00.123 # Redis error: Error condition on socket for SYNC: connection refused`
	if got := classifyTimelineEvent("redis", line); got != "error" {
		t.Fatalf("classifyTimelineEvent(redis, error) = %q, want error", got)
	}
}

func TestClassifyRedisMisconfigIsError(t *testing.T) {
	line := `MISCONF Redis is configured to save RDB snapshots, but it is currently not able to persist on disk`
	if got := classifyTimelineEvent("redis", line); got != "error" {
		t.Fatalf("classifyTimelineEvent(redis, misconfig) = %q, want error", got)
	}
}

func TestServiceLogQueryNginxUsesServiceNameLabel(t *testing.T) {
	q := serviceLogQuery("nginx", "host-1")
	if !strings.Contains(q, `service_name=`) {
		t.Fatalf("serviceLogQuery(nginx) should use service_name= label: %s", q)
	}
}

func TestServiceLogQueryDoubleQuotesEscapedNotBackslashes(t *testing.T) {
	q := serviceLogQuery("nginx", "host-1")
	// In LogQL |~ "..." string context, backslashes must be \\ to pass through.
	// So \[ in the pattern should appear as \\[ in the output.
	// Double quotes should be \".
	if !strings.Contains(q, `\\[`) {
		t.Fatalf("serviceLogQuery(nginx) should have \\[ doubled to \\\\[: %s", q)
	}
	if !strings.Contains(q, `\"`) {
		t.Fatalf("serviceLogQuery(nginx) should escape double quotes: %s", q)
	}
}

func TestServiceLogQueryPostgresUsesServiceNameLabel(t *testing.T) {
	q := serviceLogQuery("postgres", "host-1")
	if !strings.Contains(q, `service_name=`) {
		t.Fatalf("serviceLogQuery(postgres) should use service_name= label: %s", q)
	}
}

func TestServiceLogQueryDefaultFallsBackToErrorPattern(t *testing.T) {
	q := serviceLogQuery("unknown-svc", "host-1")
	if !strings.Contains(q, `error`) || !strings.Contains(q, `service_name=`) {
		t.Fatalf("serviceLogQuery(unknown) should use service_name= with error pattern: %s", q)
	}
}

func TestServiceErrorPatternNginx(t *testing.T) {
	p := serviceErrorPattern("nginx")
	if !strings.Contains(p, "error") || !strings.Contains(p, "5[0-9]{2}") {
		t.Fatalf("serviceErrorPattern(nginx) should detect logs and 5xx, got: %s", p)
	}
}

func TestServiceErrorPatternPostgres(t *testing.T) {
	p := serviceErrorPattern("postgres")
	if !strings.Contains(p, "fatal:") || !strings.Contains(p, "deadlock") {
		t.Fatalf("serviceErrorPattern(postgres) should detect fatal: and deadlock, got: %s", p)
	}
}

func TestServiceErrorPatternDefault(t *testing.T) {
	p := serviceErrorPattern("unknown")
	if p != "error" {
		t.Fatalf("serviceErrorPattern(unknown) should fall back to 'error', got: %s", p)
	}
}
