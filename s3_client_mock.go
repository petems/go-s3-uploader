package main

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// MockS3Uploader is a test double for S3Uploader that records all upload
// attempts and can be configured to return specific errors.
type MockS3Uploader struct {
	mu sync.Mutex

	// Uploads records all upload attempts in order
	Uploads []*RecordedUpload

	// ErrorFunc allows dynamic error injection based on the upload input.
	// If nil, uploads succeed. Return an error to simulate failures.
	ErrorFunc func(input *UploadInput) error

	// UploadCount tracks the total number of upload attempts
	UploadCount int
}

// RecordedUpload stores the details of an upload attempt for verification.
type RecordedUpload struct {
	Input   *UploadInput
	Content []byte // Body content is read and stored for verification
	Error   error  // The error returned (if any)
}

// NewMockS3Uploader creates a new mock uploader that records uploads.
func NewMockS3Uploader() *MockS3Uploader {
	return &MockS3Uploader{
		Uploads: make([]*RecordedUpload, 0),
	}
}

// Upload implements S3Uploader.Upload by recording the upload and
// optionally returning an error from ErrorFunc.
func (m *MockS3Uploader) Upload(_ context.Context, input *UploadInput) (*UploadOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UploadCount++

	// Read the body content for verification
	var content []byte
	if input.Body != nil {
		var err error
		content, err = io.ReadAll(input.Body)
		if err != nil {
			return nil, fmt.Errorf("mock: failed to read body: %w", err)
		}
	}

	recorded := &RecordedUpload{
		Input:   input,
		Content: content,
	}

	// Check if we should return an error
	var err error
	if m.ErrorFunc != nil {
		err = m.ErrorFunc(input)
		recorded.Error = err
	}

	m.Uploads = append(m.Uploads, recorded)

	if err != nil {
		return nil, err
	}

	return &UploadOutput{
		Location: fmt.Sprintf("https://%s.s3.amazonaws.com/%s", input.Bucket, input.Key),
		ETag:     stringPtr("\"mock-etag\""),
	}, nil
}

// Reset clears all recorded uploads and resets the counter.
func (m *MockS3Uploader) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Uploads = make([]*RecordedUpload, 0)
	m.UploadCount = 0
}

// GetUploadByKey returns the first upload matching the given key, or nil if not found.
// Note: The returned pointer references internal test data and should not be modified by callers.
func (m *MockS3Uploader) GetUploadByKey(key string) *RecordedUpload {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.Uploads {
		if u.Input.Key == key {
			return u
		}
	}
	return nil
}

// GetUploadsByBucket returns all uploads to the specified bucket.
// Note: The returned pointers reference internal test data and should not be modified by callers.
func (m *MockS3Uploader) GetUploadsByBucket(bucket string) []*RecordedUpload {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*RecordedUpload
	for _, u := range m.Uploads {
		if u.Input.Bucket == bucket {
			result = append(result, u)
		}
	}
	return result
}

// stringPtr is a helper to get a pointer to a string.
func stringPtr(s string) *string {
	return &s
}

// --- Error injection helpers ---

// ErrorOnKey returns an ErrorFunc that fails uploads matching the given key.
func ErrorOnKey(key string, err error) func(*UploadInput) error {
	return func(input *UploadInput) error {
		if input.Key == key {
			return err
		}
		return nil
	}
}

// ErrorOnAttempt returns an ErrorFunc that fails the Nth upload attempt (1-indexed).
func ErrorOnAttempt(attempt int, err error) func(*UploadInput) error {
	count := 0
	var mu sync.Mutex
	return func(input *UploadInput) error {
		mu.Lock()
		defer mu.Unlock()
		count++
		if count == attempt {
			return err
		}
		return nil
	}
}

// ErrorAlways returns an ErrorFunc that fails all uploads.
func ErrorAlways(err error) func(*UploadInput) error {
	return func(input *UploadInput) error {
		return err
	}
}

// ErrorNTimes returns an ErrorFunc that fails the first N uploads, then succeeds.
func ErrorNTimes(n int, err error) func(*UploadInput) error {
	count := 0
	var mu sync.Mutex
	return func(input *UploadInput) error {
		mu.Lock()
		defer mu.Unlock()
		count++
		if count <= n {
			return err
		}
		return nil
	}
}

// --- Common test errors ---

// RecoverableS3Error represents a transient S3 error that should trigger retries.
type RecoverableS3Error struct {
	Message string
}

func (e *RecoverableS3Error) Error() string {
	return e.Message
}

// NewRecoverableError creates an error that matches the recoverable error patterns.
func NewRecoverableError() error {
	return &RecoverableS3Error{Message: "RequestTimeout: request timed out. Idle connections will be closed."}
}

// NewNetworkError creates a simulated network error.
func NewNetworkError() error {
	return &RecoverableS3Error{Message: "dial tcp: lookup s3.amazonaws.com: no such host"}
}

// FatalS3Error represents a permanent S3 error that should not be retried.
type FatalS3Error struct {
	Message string
}

func (e *FatalS3Error) Error() string {
	return e.Message
}

// NewAccessDeniedError creates a simulated access denied error.
func NewAccessDeniedError() error {
	return &FatalS3Error{Message: "AccessDenied: Access Denied"}
}

// NewBucketNotFoundError creates a simulated bucket not found error.
func NewBucketNotFoundError() error {
	return &FatalS3Error{Message: "NoSuchBucket: The specified bucket does not exist"}
}
