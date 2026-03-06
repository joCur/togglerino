package auth_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var testCounter atomic.Int64
var testRunID = time.Now().UnixNano()

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func uniqueEmail(prefix string) string {
	n := testCounter.Add(1)
	return fmt.Sprintf("%s-%d-%d@test.com", prefix, testRunID, n)
}

func uniqueProjectKey(prefix string) string {
	n := testCounter.Add(1)
	return fmt.Sprintf("%s-%d-%d", prefix, testRunID, n)
}
