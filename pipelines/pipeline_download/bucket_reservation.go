package pipeline_download

import "github.com/t2bot/go-leaky-bucket"

type bucketReservation struct {
	bucket        *leaky.Bucket
	reservedBytes int64
}

func reserveBucket(bucket *leaky.Bucket, sizeBytes int64) (*bucketReservation, error) {
	if err := bucket.Add(sizeBytes); err != nil {
		return nil, err
	}
	return &bucketReservation{
		bucket:        bucket,
		reservedBytes: sizeBytes,
	}, nil
}

func (r *bucketReservation) Commit(sizeBytes int64) error {
	if r == nil || r.reservedBytes == 0 {
		return nil
	}
	if err := r.bucket.Drain(r.reservedBytes - sizeBytes); err != nil {
		return err
	}
	r.reservedBytes = 0
	return nil
}

func (r *bucketReservation) Release() error {
	if r == nil || r.reservedBytes == 0 {
		return nil
	}
	if err := r.bucket.Drain(r.reservedBytes); err != nil {
		return err
	}
	r.reservedBytes = 0
	return nil
}
