package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMockS3Uploader_RecordsUploads(t *testing.T) {
	mock := NewMockS3Uploader()
	ctx := context.Background()

	input := &UploadInput{
		Bucket:      "test-bucket",
		Key:         "test/file.txt",
		Body:        strings.NewReader("hello world"),
		ContentType: stringPtr("text/plain"),
	}

	output, err := mock.Upload(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Location == "" {
		t.Error("expected Location to be set")
	}

	if mock.UploadCount != 1 {
		t.Errorf("expected UploadCount=1, got %d", mock.UploadCount)
	}

	if len(mock.Uploads) != 1 {
		t.Fatalf("expected 1 recorded upload, got %d", len(mock.Uploads))
	}

	recorded := mock.Uploads[0]
	if recorded.Input.Key != "test/file.txt" {
		t.Errorf("expected key 'test/file.txt', got '%s'", recorded.Input.Key)
	}
	if string(recorded.Content) != "hello world" {
		t.Errorf("expected content 'hello world', got '%s'", recorded.Content)
	}
}

func TestMockS3Uploader_ErrorFunc(t *testing.T) {
	mock := NewMockS3Uploader()
	mock.ErrorFunc = ErrorAlways(errors.New("simulated failure"))
	ctx := context.Background()

	input := &UploadInput{
		Bucket: "test-bucket",
		Key:    "test/file.txt",
		Body:   strings.NewReader("content"),
	}

	_, err := mock.Upload(ctx, input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "simulated failure" {
		t.Errorf("expected 'simulated failure', got '%s'", err.Error())
	}

	// Verify the error was recorded
	if mock.Uploads[0].Error == nil {
		t.Error("expected error to be recorded")
	}
}

func TestMockS3Uploader_ErrorOnKey(t *testing.T) {
	mock := NewMockS3Uploader()
	mock.ErrorFunc = ErrorOnKey("fail-this.txt", errors.New("key-specific error"))
	ctx := context.Background()

	// This should succeed
	_, err := mock.Upload(ctx, &UploadInput{
		Bucket: "test-bucket",
		Key:    "success.txt",
		Body:   strings.NewReader("content"),
	})
	if err != nil {
		t.Fatalf("unexpected error for success.txt: %v", err)
	}

	// This should fail
	_, err = mock.Upload(ctx, &UploadInput{
		Bucket: "test-bucket",
		Key:    "fail-this.txt",
		Body:   strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("expected error for fail-this.txt")
	}
}

func TestMockS3Uploader_ErrorNTimes(t *testing.T) {
	mock := NewMockS3Uploader()
	mock.ErrorFunc = ErrorNTimes(2, errors.New("temporary failure"))
	ctx := context.Background()

	input := &UploadInput{
		Bucket: "test-bucket",
		Key:    "retry-test.txt",
		Body:   strings.NewReader("content"),
	}

	// First two attempts should fail
	for i := 0; i < 2; i++ {
		input.Body = strings.NewReader("content") // Reset reader
		_, err := mock.Upload(ctx, input)
		if err == nil {
			t.Fatalf("attempt %d: expected error", i+1)
		}
	}

	// Third attempt should succeed
	input.Body = strings.NewReader("content")
	_, err := mock.Upload(ctx, input)
	if err != nil {
		t.Fatalf("attempt 3: unexpected error: %v", err)
	}
}

func TestMockS3Uploader_GetUploadByKey(t *testing.T) {
	mock := NewMockS3Uploader()
	ctx := context.Background()

	mock.Upload(ctx, &UploadInput{Bucket: "bucket", Key: "file1.txt", Body: strings.NewReader("one")})
	mock.Upload(ctx, &UploadInput{Bucket: "bucket", Key: "file2.txt", Body: strings.NewReader("two")})
	mock.Upload(ctx, &UploadInput{Bucket: "bucket", Key: "file3.txt", Body: strings.NewReader("three")})

	upload := mock.GetUploadByKey("file2.txt")
	if upload == nil {
		t.Fatal("expected to find file2.txt")
	}
	if string(upload.Content) != "two" {
		t.Errorf("expected content 'two', got '%s'", upload.Content)
	}

	missing := mock.GetUploadByKey("nonexistent.txt")
	if missing != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestMockS3Uploader_GetUploadsByBucket(t *testing.T) {
	mock := NewMockS3Uploader()
	ctx := context.Background()

	mock.Upload(ctx, &UploadInput{Bucket: "bucket-a", Key: "file1.txt", Body: strings.NewReader("a1")})
	mock.Upload(ctx, &UploadInput{Bucket: "bucket-b", Key: "file2.txt", Body: strings.NewReader("b1")})
	mock.Upload(ctx, &UploadInput{Bucket: "bucket-a", Key: "file3.txt", Body: strings.NewReader("a2")})

	bucketA := mock.GetUploadsByBucket("bucket-a")
	if len(bucketA) != 2 {
		t.Errorf("expected 2 uploads to bucket-a, got %d", len(bucketA))
	}

	bucketB := mock.GetUploadsByBucket("bucket-b")
	if len(bucketB) != 1 {
		t.Errorf("expected 1 upload to bucket-b, got %d", len(bucketB))
	}
}

func TestMockS3Uploader_Reset(t *testing.T) {
	mock := NewMockS3Uploader()
	ctx := context.Background()

	mock.Upload(ctx, &UploadInput{Bucket: "bucket", Key: "file.txt", Body: strings.NewReader("content")})

	if mock.UploadCount != 1 {
		t.Fatalf("expected UploadCount=1 before reset, got %d", mock.UploadCount)
	}

	mock.Reset()

	if mock.UploadCount != 0 {
		t.Errorf("expected UploadCount=0 after reset, got %d", mock.UploadCount)
	}
	if len(mock.Uploads) != 0 {
		t.Errorf("expected 0 uploads after reset, got %d", len(mock.Uploads))
	}
}

func TestMockS3Uploader_VerifyHeaders(t *testing.T) {
	mock := NewMockS3Uploader()
	ctx := context.Background()

	input := &UploadInput{
		Bucket:               "test-bucket",
		Key:                  "compressed.js",
		Body:                 bytes.NewReader([]byte("compressed content")),
		ContentType:          stringPtr("application/javascript"),
		ContentEncoding:      stringPtr("gzip"),
		CacheControl:         stringPtr("max-age=31536000"),
		ServerSideEncryption: stringPtr("AES256"),
	}

	_, err := mock.Upload(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	recorded := mock.Uploads[0]

	if *recorded.Input.ContentType != "application/javascript" {
		t.Errorf("ContentType mismatch: got %s", *recorded.Input.ContentType)
	}
	if *recorded.Input.ContentEncoding != "gzip" {
		t.Errorf("ContentEncoding mismatch: got %s", *recorded.Input.ContentEncoding)
	}
	if *recorded.Input.CacheControl != "max-age=31536000" {
		t.Errorf("CacheControl mismatch: got %s", *recorded.Input.CacheControl)
	}
	if *recorded.Input.ServerSideEncryption != "AES256" {
		t.Errorf("ServerSideEncryption mismatch: got %s", *recorded.Input.ServerSideEncryption)
	}
}

func TestRecoverableErrors(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		isRec bool
	}{
		{"network error", NewNetworkError(), true},
		{"timeout error", NewRecoverableError(), true},
		{"access denied", NewAccessDeniedError(), false},
		{"bucket not found", NewBucketNotFoundError(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRecoverable(tt.err)
			if got != tt.isRec {
				t.Errorf("isRecoverable(%q) = %v, want %v", tt.err.Error(), got, tt.isRec)
			}
		})
	}
}

func TestMockS3Uploader_ConcurrentUploads(t *testing.T) {
	mock := NewMockS3Uploader()
	ctx := context.Background()

	// Simulate concurrent uploads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			input := &UploadInput{
				Bucket: "test-bucket",
				Key:    strings.Replace("file-N.txt", "N", string(rune('0'+n)), 1),
				Body:   strings.NewReader("content"),
			}
			mock.Upload(ctx, input)
			done <- true
		}(i)
	}

	// Wait for all uploads
	for i := 0; i < 10; i++ {
		<-done
	}

	if mock.UploadCount != 10 {
		t.Errorf("expected 10 uploads, got %d", mock.UploadCount)
	}
}
