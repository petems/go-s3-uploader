package main

import (
	"fmt"
	"sync"
)

// MockS3Client implements S3Client for testing
type MockS3Client struct {
	mu      sync.RWMutex
	uploads []RecordedUpload
	err     error
}

// RecordedUpload captures details of an upload call for testing
type RecordedUpload struct {
	Bucket               string
	Key                  string
	ContentType          string
	ContentEncoding      string
	CacheControl         string
	ServerSideEncryption string
}

// NewMockS3Client creates a new mock S3 client
func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		uploads: make([]RecordedUpload, 0),
	}
}

// Upload records the upload parameters for testing
func (m *MockS3Client) Upload(input *S3UploadInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	record := RecordedUpload{}
	if input.Bucket != nil {
		record.Bucket = *input.Bucket
	}
	if input.Key != nil {
		record.Key = *input.Key
	}
	if input.ContentType != nil {
		record.ContentType = *input.ContentType
	}
	if input.ContentEncoding != nil {
		record.ContentEncoding = *input.ContentEncoding
	}
	if input.CacheControl != nil {
		record.CacheControl = *input.CacheControl
	}
	if input.ServerSideEncryption != nil {
		record.ServerSideEncryption = *input.ServerSideEncryption
	}

	m.uploads = append(m.uploads, record)
	return nil
}

// SetError configures the mock to return an error on Upload
func (m *MockS3Client) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// GetUploads returns all recorded uploads
func (m *MockS3Client) GetUploads() []RecordedUpload {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent concurrent modification
	result := make([]RecordedUpload, len(m.uploads))
	copy(result, m.uploads)
	return result
}

// GetUploadCount returns the number of uploads recorded
func (m *MockS3Client) GetUploadCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.uploads)
}

// GetUploadByKey returns the upload for a specific key
// Returns internal pointer - do not modify to avoid race conditions
func (m *MockS3Client) GetUploadByKey(key string) (*RecordedUpload, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.uploads {
		if m.uploads[i].Key == key {
			return &m.uploads[i], nil
		}
	}
	return nil, fmt.Errorf("upload not found for key: %s", key)
}

// Reset clears all recorded uploads
func (m *MockS3Client) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploads = make([]RecordedUpload, 0)
	m.err = nil
}
