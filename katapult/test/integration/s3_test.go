//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/maxitosh/katapult/internal/testutil"
)

// @cpt-dod:cpt-katapult-dod-integration-tests-component-tests:p2
// @cpt-flow:cpt-katapult-flow-integration-tests-run-component-tests:p2

// @cpt-begin:cpt-katapult-dod-integration-tests-component-tests:p2:inst-s3-tests

func TestS3_UploadAndDownload_ViaMinIO(t *testing.T) {
	cfg := getTestMinIO(t)
	ctx := context.Background()

	client := newMinIOClient(t, cfg)

	// Create bucket.
	if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
		// Ignore "bucket already exists" error.
		exists, errCheck := client.BucketExists(ctx, cfg.Bucket)
		if errCheck != nil || !exists {
			t.Fatalf("creating bucket: %v", err)
		}
	}

	// Upload test data.
	testData := []byte("integration test data for S3 upload/download verification")
	objectKey := "test-transfer/data.bin"

	_, err := client.PutObject(ctx, cfg.Bucket, objectKey, bytes.NewReader(testData), int64(len(testData)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("uploading to MinIO: %v", err)
	}

	// Download and verify checksum.
	obj, err := client.GetObject(ctx, cfg.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("downloading from MinIO: %v", err)
	}
	defer obj.Close()

	downloaded, err := io.ReadAll(obj)
	if err != nil {
		t.Fatalf("reading download body: %v", err)
	}

	uploadHash := sha256.Sum256(testData)
	downloadHash := sha256.Sum256(downloaded)

	if uploadHash != downloadHash {
		t.Fatalf("checksum mismatch: upload=%x, download=%x", uploadHash, downloadHash)
	}
}

func newMinIOClient(t *testing.T, cfg testutil.MinIOConfig) *minio.Client {
	t.Helper()
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("creating minio client: %v", err)
	}
	return client
}

// @cpt-end:cpt-katapult-dod-integration-tests-component-tests:p2:inst-s3-tests
