package service

import (
	"strings"
	"testing"
)

func TestClassifyPostgresLogAvoidsDeadlockTableNameFalsePositive(t *testing.T) {
	message := "LOG:  statement: DROP TABLE IF EXISTS deadlock_test;"

	if got := classifyPostgresLog(message); got != "schema change" {
		t.Fatalf("classifyPostgresLog() = %q, want schema change", got)
	}
}

func TestClassifyPostgresLogRecognizesRealLockWait(t *testing.T) {
	message := "LOG:  process 127207 still waiting for AccessShareLock on relation 1259 of database 5 after 1000.077 ms"

	if got := classifyPostgresLog(message); got != "lock wait" {
		t.Fatalf("classifyPostgresLog() = %q, want lock wait", got)
	}
}

func TestClassifyPostgresLogHandlesMultilineDDLStatement(t *testing.T) {
	message := "LOG:  statement:\nCREATE TABLE test_errors (id INT PRIMARY KEY);"

	if got := classifyPostgresLog(message); got != "schema change" {
		t.Fatalf("classifyPostgresLog() = %q, want schema change", got)
	}
}

func TestMergePGMultilineEntriesMergesDetailButKeepsStatementPrimary(t *testing.T) {
	entries := []LokiLogEntry{
		{
			TimestampNs: "100",
			Line:        `2026-05-12 17:38:46.750 CET [130546] postgres@postgres [local] psql ERROR:  duplicate key value violates unique constraint "test_errors_pkey"`,
		},
		{
			TimestampNs: "101",
			Line:        `2026-05-12 17:38:46.750 CET [130546] postgres@postgres [local] psql DETAIL:  Key (id)=(1) already exists.`,
		},
		{
			TimestampNs: "102",
			Line:        `2026-05-12 17:38:46.692 CET [130546] postgres@postgres [local] psql STATEMENT:  SELECT * FROM nonexistent_table_12345;`,
		},
	}

	merged := mergePGMultilineEntries(entries)
	if len(merged) != 2 {
		t.Fatalf("len(mergePGMultilineEntries()) = %d, want 2", len(merged))
	}
	if !strings.Contains(merged[0].Line, "ERROR:") || !strings.Contains(merged[0].Line, "DETAIL:") {
		t.Fatalf("first merged entry should contain ERROR and DETAIL, got %q", merged[0].Line)
	}
	if !strings.Contains(merged[1].Line, "STATEMENT:") {
		t.Fatalf("second merged entry should remain STATEMENT primary, got %q", merged[1].Line)
	}
}

func TestMergePGMultilineEntriesMergesHeaderContinuationBeforePrimary(t *testing.T) {
	entries := []LokiLogEntry{
		{
			TimestampNs: "100",
			Line:        `2026-05-12 17:38:46.750 CET [130546] postgres@postgres [local] psql DETAIL:  Key (id)=(1) already exists.`,
		},
		{
			TimestampNs: "100",
			Line:        `2026-05-12 17:38:46.750 CET [130546] postgres@postgres [local] psql ERROR:  duplicate key value violates unique constraint "test_errors_pkey"`,
		},
	}

	merged := mergePGMultilineEntries(entries)
	if len(merged) != 1 {
		t.Fatalf("len(mergePGMultilineEntries()) = %d, want 1", len(merged))
	}
	if !strings.Contains(merged[0].Line, "ERROR:") || !strings.Contains(merged[0].Line, "DETAIL:") {
		t.Fatalf("merged entry should contain ERROR and DETAIL, got %q", merged[0].Line)
	}
}

func TestMergePGMultilineEntriesKeepsCheckpointServerLinesSeparate(t *testing.T) {
	entries := []LokiLogEntry{
		{
			TimestampNs: "100",
			Line:        `2026-05-12 17:05:27.597 CET [1140] @ LOG:  checkpoint starting: time`,
		},
		{
			TimestampNs: "101",
			Line:        `2026-05-12 17:05:29.683 CET [1140] @ LOG:  checkpoint complete: wrote 0 buffers (0.0%)`,
		},
	}

	merged := mergePGMultilineEntries(entries)
	if len(merged) != 2 {
		t.Fatalf("len(mergePGMultilineEntries()) = %d, want 2", len(merged))
	}

	event := parsePostgresLogEvent(merged[0])
	if event.Category != "checkpoint" || event.Message != "LOG:  checkpoint starting: time" {
		t.Fatalf("unexpected parsed checkpoint event: category=%q message=%q", event.Category, event.Message)
	}
}
