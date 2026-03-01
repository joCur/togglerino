package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/togglerino/togglerino/internal/model"
)

// --- Mocks ---

type mockScheduleStore struct {
	due       []model.ScheduledFlagChange
	listErr   error
	execErr   error
	executed  []string // schedule IDs that were executed
}

func (m *mockScheduleStore) ListDue(_ context.Context, _ time.Time) ([]model.ScheduledFlagChange, error) {
	return m.due, m.listErr
}

func (m *mockScheduleStore) Execute(_ context.Context, _ pgx.Tx, scheduleID, _, _ string, _ model.ConfigSnapshotPayload) error {
	m.executed = append(m.executed, scheduleID)
	return m.execErr
}

type mockLookup struct {
	projectKey string
	projectID  string
	flagKey    string
	envKey     string
	err        error // returned by all methods if set
}

func (m *mockLookup) ProjectKeyByFlagID(_ context.Context, _ string) (string, error) {
	return m.projectKey, m.err
}

func (m *mockLookup) ProjectIDByFlagID(_ context.Context, _ string) (string, error) {
	return m.projectID, m.err
}

func (m *mockLookup) FlagKeyByID(_ context.Context, _ string) (string, error) {
	return m.flagKey, m.err
}

func (m *mockLookup) EnvKeyByID(_ context.Context, _ string) (string, error) {
	return m.envKey, m.err
}

type mockTx struct{}

func (m *mockTx) Begin(_ context.Context) (pgx.Tx, error)                        { return &fakeTx{}, nil }

type mockTxErr struct{ err error }

func (m *mockTxErr) Begin(_ context.Context) (pgx.Tx, error)                      { return nil, m.err }

// fakeTx implements pgx.Tx with no-op methods.
type fakeTx struct{}

func (f *fakeTx) Begin(_ context.Context) (pgx.Tx, error)                          { return f, nil }
func (f *fakeTx) Commit(_ context.Context) error                                   { return nil }
func (f *fakeTx) Rollback(_ context.Context) error                                 { return nil }
func (f *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (f *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults       { return nil }
func (f *fakeTx) LargeObjects() pgx.LargeObjects                                   { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(_ context.Context, _ string, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error)   { return nil, nil }
func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row          { return nil }
func (f *fakeTx) Conn() *pgx.Conn                                                 { return nil }

type mockCache struct {
	refreshed []string // "projectKey:envKey"
}

func (m *mockCache) Refresh(_ context.Context, projectKey, envKey string) error {
	m.refreshed = append(m.refreshed, projectKey+":"+envKey)
	return nil
}

type mockBroadcaster struct {
	broadcasts []broadcast
}

type broadcast struct {
	projectKey, envKey, flagKey string
	enabled                    bool
	defaultVariant             string
}

func (m *mockBroadcaster) Broadcast(projectKey, envKey, flagKey string, enabled bool, defaultVariant string) {
	m.broadcasts = append(m.broadcasts, broadcast{projectKey, envKey, flagKey, enabled, defaultVariant})
}

type mockAudit struct {
	entries []model.AuditEntry
}

func (m *mockAudit) Record(_ context.Context, entry model.AuditEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

// --- Helpers ---

func validSnapshot() json.RawMessage {
	return json.RawMessage(`{"enabled":true,"default_variant":"on","variants":[],"targeting_rules":[]}`)
}

func makeSchedule(id string, snapshot json.RawMessage) model.ScheduledFlagChange {
	return model.ScheduledFlagChange{
		ID:             id,
		FlagID:         "flag-1",
		EnvironmentID:  "env-1",
		ScheduledAt:    time.Now().Add(-1 * time.Minute),
		Status:         model.ScheduleStatusPending,
		ConfigSnapshot: snapshot,
	}
}

// --- Tests ---

func TestTick_NoDueSchedules(t *testing.T) {
	store := &mockScheduleStore{}
	c := &Checker{
		schedules:   store,
		lookup:      &mockLookup{},
		db:          &mockTx{},
		cache:       &mockCache{},
		broadcaster: &mockBroadcaster{},
		audit:       &mockAudit{},
		now:         time.Now,
	}

	c.tick(context.Background())

	if len(store.executed) != 0 {
		t.Errorf("expected no executions, got %d", len(store.executed))
	}
}

func TestTick_ListDueError(t *testing.T) {
	store := &mockScheduleStore{listErr: errors.New("db down")}
	c := &Checker{
		schedules:   store,
		lookup:      &mockLookup{},
		db:          &mockTx{},
		cache:       &mockCache{},
		broadcaster: &mockBroadcaster{},
		audit:       &mockAudit{},
		now:         time.Now,
	}

	// Should not panic
	c.tick(context.Background())
}

func TestTick_ExecutesDueSchedule(t *testing.T) {
	store := &mockScheduleStore{
		due: []model.ScheduledFlagChange{makeSchedule("sched-1", validSnapshot())},
	}
	lookup := &mockLookup{projectKey: "proj", projectID: "proj-id", flagKey: "my-flag", envKey: "dev"}
	cache := &mockCache{}
	bc := &mockBroadcaster{}
	audit := &mockAudit{}

	c := &Checker{
		schedules:   store,
		lookup:      lookup,
		db:          &mockTx{},
		cache:       cache,
		broadcaster: bc,
		audit:       audit,
		now:         time.Now,
	}

	c.tick(context.Background())

	if len(store.executed) != 1 || store.executed[0] != "sched-1" {
		t.Fatalf("expected sched-1 executed, got %v", store.executed)
	}
	if len(cache.refreshed) != 1 || cache.refreshed[0] != "proj:dev" {
		t.Errorf("expected cache refresh for proj:dev, got %v", cache.refreshed)
	}
	if len(bc.broadcasts) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.broadcasts))
	}
	if bc.broadcasts[0].flagKey != "my-flag" || bc.broadcasts[0].enabled != true {
		t.Errorf("unexpected broadcast: %+v", bc.broadcasts[0])
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "schedule_executed" {
		t.Errorf("expected schedule_executed audit entry, got %v", audit.entries)
	}
}

func TestTick_MultipleDueSchedules(t *testing.T) {
	store := &mockScheduleStore{
		due: []model.ScheduledFlagChange{
			makeSchedule("s1", validSnapshot()),
			makeSchedule("s2", validSnapshot()),
		},
	}
	c := &Checker{
		schedules:   store,
		lookup:      &mockLookup{projectKey: "p", projectID: "pid", flagKey: "f", envKey: "e"},
		db:          &mockTx{},
		cache:       &mockCache{},
		broadcaster: &mockBroadcaster{},
		audit:       &mockAudit{},
		now:         time.Now,
	}

	c.tick(context.Background())

	if len(store.executed) != 2 {
		t.Errorf("expected 2 executions, got %d", len(store.executed))
	}
}

func TestExecute_InvalidSnapshot(t *testing.T) {
	store := &mockScheduleStore{}
	c := &Checker{
		schedules:   store,
		lookup:      &mockLookup{},
		db:          &mockTx{},
		cache:       &mockCache{},
		broadcaster: &mockBroadcaster{},
		audit:       &mockAudit{},
		now:         time.Now,
	}

	// Invalid JSON should not panic and should not call Execute
	c.execute(context.Background(), makeSchedule("bad", json.RawMessage(`{invalid`)))

	if len(store.executed) != 0 {
		t.Errorf("expected no executions for invalid snapshot, got %d", len(store.executed))
	}
}

func TestExecute_BeginTxError(t *testing.T) {
	store := &mockScheduleStore{}
	c := &Checker{
		schedules:   store,
		lookup:      &mockLookup{},
		db:          &mockTxErr{err: errors.New("pool exhausted")},
		cache:       &mockCache{},
		broadcaster: &mockBroadcaster{},
		audit:       &mockAudit{},
		now:         time.Now,
	}

	c.execute(context.Background(), makeSchedule("s1", validSnapshot()))

	if len(store.executed) != 0 {
		t.Errorf("expected no executions when Begin fails, got %d", len(store.executed))
	}
}

func TestExecute_StoreExecuteError(t *testing.T) {
	store := &mockScheduleStore{execErr: errors.New("row not found")}
	cache := &mockCache{}
	c := &Checker{
		schedules:   store,
		lookup:      &mockLookup{},
		db:          &mockTx{},
		cache:       cache,
		broadcaster: &mockBroadcaster{},
		audit:       &mockAudit{},
		now:         time.Now,
	}

	c.execute(context.Background(), makeSchedule("s1", validSnapshot()))

	// Execute was called but failed — cache should not be refreshed
	if len(cache.refreshed) != 0 {
		t.Errorf("expected no cache refresh on Execute error, got %v", cache.refreshed)
	}
}

func TestExecute_NilVariantsDefaulted(t *testing.T) {
	// Snapshot without variants/targeting_rules — should get defaulted to []
	snapshot := json.RawMessage(`{"enabled":true,"default_variant":"on"}`)
	store := &mockScheduleStore{}
	c := &Checker{
		schedules:   store,
		lookup:      &mockLookup{projectKey: "p", projectID: "pid", flagKey: "f", envKey: "e"},
		db:          &mockTx{},
		cache:       &mockCache{},
		broadcaster: &mockBroadcaster{},
		audit:       &mockAudit{},
		now:         time.Now,
	}

	c.execute(context.Background(), makeSchedule("s1", snapshot))

	if len(store.executed) != 1 {
		t.Errorf("expected execution to succeed with nil variants, got %d", len(store.executed))
	}
}

func TestPostExecute_LookupFailure_StillAudits(t *testing.T) {
	// projectKey lookup fails, but projectID succeeds — audit should still happen
	lookup := &mockLookup{projectID: "pid", flagKey: "f", envKey: "e"}
	cache := &mockCache{}
	audit := &mockAudit{}

	c := &Checker{
		schedules:   &mockScheduleStore{},
		lookup:      lookup,
		db:          &mockTx{},
		cache:       cache,
		broadcaster: &mockBroadcaster{},
		audit:       audit,
		now:         time.Now,
	}

	// Override projectKey to fail
	failLookup := &selectiveLookup{
		projectKeyErr: errors.New("not found"),
		projectID:     "pid",
		flagKey:       "f",
		envKey:        "e",
	}
	c.lookup = failLookup

	sc := makeSchedule("s1", validSnapshot())
	var snap model.ConfigSnapshotPayload
	json.Unmarshal(sc.ConfigSnapshot, &snap)

	c.postExecute(context.Background(), sc, snap)

	// Cache should NOT be refreshed (projectKey failed)
	if len(cache.refreshed) != 0 {
		t.Errorf("expected no cache refresh when projectKey fails, got %v", cache.refreshed)
	}
	// Audit should still succeed (projectID succeeded)
	if len(audit.entries) != 1 {
		t.Errorf("expected audit to proceed despite projectKey failure, got %d entries", len(audit.entries))
	}
}

// selectiveLookup lets individual methods fail independently.
type selectiveLookup struct {
	projectKey    string
	projectKeyErr error
	projectID     string
	projectIDErr  error
	flagKey       string
	flagKeyErr    error
	envKey        string
	envKeyErr     error
}

func (l *selectiveLookup) ProjectKeyByFlagID(_ context.Context, _ string) (string, error) {
	return l.projectKey, l.projectKeyErr
}
func (l *selectiveLookup) ProjectIDByFlagID(_ context.Context, _ string) (string, error) {
	return l.projectID, l.projectIDErr
}
func (l *selectiveLookup) FlagKeyByID(_ context.Context, _ string) (string, error) {
	return l.flagKey, l.flagKeyErr
}
func (l *selectiveLookup) EnvKeyByID(_ context.Context, _ string) (string, error) {
	return l.envKey, l.envKeyErr
}
