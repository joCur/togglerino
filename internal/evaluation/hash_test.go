package evaluation

import (
	"fmt"
	"testing"
)

func TestConsistentHash_Deterministic(t *testing.T) {
	// Same input always gives same output.
	flagKey := "feature-flag-1"
	userID := "user-123"

	result1 := ConsistentHash(flagKey, userID)
	result2 := ConsistentHash(flagKey, userID)
	result3 := ConsistentHash(flagKey, userID)

	if result1 != result2 || result2 != result3 {
		t.Errorf("ConsistentHash is not deterministic: %d, %d, %d", result1, result2, result3)
	}
}

func TestConsistentHash_Range(t *testing.T) {
	// Result should always be in [0, 99].
	for i := 0; i < 1000; i++ {
		bucket := ConsistentHash("flag", fmt.Sprintf("user-%d", i))
		if bucket < 0 || bucket > 99 {
			t.Errorf("ConsistentHash returned %d, expected 0-99", bucket)
		}
	}
}

func TestConsistentHash_DifferentFlagKeys(t *testing.T) {
	// Different flag keys should generally give different buckets for the same user.
	userID := "user-123"
	bucket1 := ConsistentHash("flag-a", userID)
	bucket2 := ConsistentHash("flag-b", userID)
	bucket3 := ConsistentHash("flag-c", userID)

	// While it's theoretically possible for all to be the same, it's very unlikely
	// with SHA-256. We check that at least one differs.
	if bucket1 == bucket2 && bucket2 == bucket3 {
		t.Errorf("all three different flag keys produced the same bucket %d for user %s", bucket1, userID)
	}
}

func TestConsistentHash_DifferentUsers(t *testing.T) {
	// Different users should generally give different buckets for the same flag.
	flagKey := "my-flag"
	bucket1 := ConsistentHash(flagKey, "user-1")
	bucket2 := ConsistentHash(flagKey, "user-2")
	bucket3 := ConsistentHash(flagKey, "user-3")

	if bucket1 == bucket2 && bucket2 == bucket3 {
		t.Errorf("all three different users produced the same bucket %d for flag %s", bucket1, flagKey)
	}
}

func TestConsistentHash_Distribution(t *testing.T) {
	// Hash 10000 users, check no bucket has more than 2x the expected count.
	// Expected count per bucket = 10000 / 100 = 100.
	// 2x expected = 200.
	counts := make([]int, 100)
	numUsers := 10000

	for i := 0; i < numUsers; i++ {
		bucket := ConsistentHash("distribution-test-flag", fmt.Sprintf("user-%d", i))
		counts[bucket]++
	}

	expected := numUsers / 100
	maxAllowed := expected * 2

	for bucket, count := range counts {
		if count > maxAllowed {
			t.Errorf("bucket %d has %d entries, max allowed is %d (2x expected %d)",
				bucket, count, maxAllowed, expected)
		}
	}

	// Also check that no bucket is completely empty (very unlikely with 10000 users / 100 buckets).
	for bucket, count := range counts {
		if count == 0 {
			t.Errorf("bucket %d has 0 entries, expected some distribution", bucket)
		}
	}
}
