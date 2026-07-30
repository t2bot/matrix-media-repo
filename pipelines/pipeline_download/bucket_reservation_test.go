package pipeline_download

import (
	"errors"
	"testing"
	"time"

	"github.com/t2bot/go-leaky-bucket"
)

func newTestBucket(t *testing.T, capacity int64) *leaky.Bucket {
	t.Helper()
	bucket, err := leaky.NewBucket(1, time.Hour, capacity)
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}
	return bucket
}

func TestBucketReservationRelease(t *testing.T) {
	bucket := newTestBucket(t, 100)
	if err := bucket.Add(30); err != nil {
		t.Fatalf("failed to seed bucket: %v", err)
	}

	reservation, err := reserveBucket(bucket, 50)
	if err != nil {
		t.Fatalf("failed to reserve bucket: %v", err)
	}
	if value := bucket.Value(); value != 80 {
		t.Fatalf("expected bucket value 80 after reservation, got %d", value)
	}

	if err = reservation.Release(); err != nil {
		t.Fatalf("failed to release reservation: %v", err)
	}
	if value := bucket.Value(); value != 30 {
		t.Fatalf("expected bucket value 30 after release, got %d", value)
	}

	if err = reservation.Release(); err != nil {
		t.Fatalf("second release should be a no-op: %v", err)
	}
	if value := bucket.Value(); value != 30 {
		t.Fatalf("expected second release to leave bucket value at 30, got %d", value)
	}
}

func TestBucketReservationCommit(t *testing.T) {
	bucket := newTestBucket(t, 100)
	if err := bucket.Add(10); err != nil {
		t.Fatalf("failed to seed bucket: %v", err)
	}

	reservation, err := reserveBucket(bucket, 80)
	if err != nil {
		t.Fatalf("failed to reserve bucket: %v", err)
	}
	if err = reservation.Commit(20); err != nil {
		t.Fatalf("failed to commit reservation: %v", err)
	}
	if value := bucket.Value(); value != 30 {
		t.Fatalf("expected bucket value 30 after commit, got %d", value)
	}

	if err = reservation.Release(); err != nil {
		t.Fatalf("release after commit should be a no-op: %v", err)
	}
	if value := bucket.Value(); value != 30 {
		t.Fatalf("expected release after commit to leave bucket value at 30, got %d", value)
	}
}

func TestBucketReservationCanBeReleasedAfterCommitFailure(t *testing.T) {
	bucket := newTestBucket(t, 100)
	if err := bucket.Add(70); err != nil {
		t.Fatalf("failed to seed bucket: %v", err)
	}

	reservation, err := reserveBucket(bucket, 20)
	if err != nil {
		t.Fatalf("failed to reserve bucket: %v", err)
	}
	if err = reservation.Commit(40); !errors.Is(err, leaky.ErrBucketFull) {
		t.Fatalf("expected bucket full error, got %v", err)
	}

	if err = reservation.Release(); err != nil {
		t.Fatalf("failed to release reservation after commit failure: %v", err)
	}
	if value := bucket.Value(); value != 70 {
		t.Fatalf("expected bucket value 70 after release, got %d", value)
	}
}

func TestReserveBucketFailureDoesNotChangeBucket(t *testing.T) {
	bucket := newTestBucket(t, 100)
	if err := bucket.Add(90); err != nil {
		t.Fatalf("failed to seed bucket: %v", err)
	}

	reservation, err := reserveBucket(bucket, 20)
	if !errors.Is(err, leaky.ErrBucketFull) {
		t.Fatalf("expected bucket full error, got %v", err)
	}
	if reservation != nil {
		t.Fatal("expected no reservation after failed reserve")
	}
	if value := bucket.Value(); value != 90 {
		t.Fatalf("expected failed reservation to leave bucket value at 90, got %d", value)
	}
}
