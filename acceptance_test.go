//go:build acceptance
// +build acceptance

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	localstackEndpoint = "http://localhost:4566"
	testBucketName     = "test-bucket"
	testRegion         = "us-east-1"
)

// acceptanceTestSuite holds resources for acceptance tests
type acceptanceTestSuite struct {
	ctx      context.Context
	s3Client *s3.Client
	bucket   string
	tempDir  string
}

// newAcceptanceTestSuite creates and initializes a test suite for acceptance tests
func newAcceptanceTestSuite(t *testing.T) *acceptanceTestSuite {
	t.Helper()

	ctx := context.Background()

	// Load AWS SDK v2 config with LocalStack endpoint
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(testRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test",
			"test",
			"",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load AWS config: %v", err)
	}

	// Create S3 client pointing to LocalStack
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(localstackEndpoint)
		o.UsePathStyle = true // LocalStack requires path-style addressing
	})

	// Create test bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucketName),
	})
	if err != nil {
		// Check if bucket already exists (idempotent)
		var bne *types.BucketAlreadyExists
		var bao *types.BucketAlreadyOwnedByYou
		if !errors.As(err, &bne) && !errors.As(err, &bao) {
			t.Fatalf("failed to create bucket: %v", err)
		}
	}

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "go-s3-uploader-acceptance-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	suite := &acceptanceTestSuite{
		ctx:      ctx,
		s3Client: client,
		bucket:   testBucketName,
		tempDir:  tempDir,
	}

	// Register cleanup
	t.Cleanup(func() {
		suite.cleanup(t)
	})

	return suite
}

// cleanup removes test resources
func (suite *acceptanceTestSuite) cleanup(t *testing.T) {
	t.Helper()

	// Clean up temp directory
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			t.Logf("warning: failed to remove temp directory: %v", err)
		}
	}

	// Clean up S3 bucket contents
	if suite.s3Client != nil && suite.bucket != "" {
		suite.emptyBucket(t)
	}
}

// emptyBucket removes all objects from the test bucket
func (suite *acceptanceTestSuite) emptyBucket(t *testing.T) {
	t.Helper()

	listOutput, err := suite.s3Client.ListObjectsV2(suite.ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(suite.bucket),
	})
	if err != nil {
		t.Logf("warning: failed to list objects for cleanup: %v", err)
		return
	}

	for _, obj := range listOutput.Contents {
		_, err := suite.s3Client.DeleteObject(suite.ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(suite.bucket),
			Key:    obj.Key,
		})
		if err != nil {
			t.Logf("warning: failed to delete object %s: %v", *obj.Key, err)
		}
	}
}

// createTestFile creates a file with given content in the temp directory
func (suite *acceptanceTestSuite) createTestFile(t *testing.T, relativePath, content string) string {
	t.Helper()

	fullPath := filepath.Join(suite.tempDir, relativePath)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file %s: %v", fullPath, err)
	}

	return fullPath
}

// objectExists checks if an object exists in S3
func (suite *acceptanceTestSuite) objectExists(t *testing.T, key string) bool {
	t.Helper()

	_, err := suite.s3Client.HeadObject(suite.ctx, &s3.HeadObjectInput{
		Bucket: aws.String(suite.bucket),
		Key:    aws.String(key),
	})

	return err == nil
}

// getObjectContent retrieves the content of an object from S3
func (suite *acceptanceTestSuite) getObjectContent(t *testing.T, key string) string {
	t.Helper()

	output, err := suite.s3Client.GetObject(suite.ctx, &s3.GetObjectInput{
		Bucket: aws.String(suite.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to get object %s: %v", key, err)
	}
	defer output.Body.Close()

	content, err := io.ReadAll(output.Body)
	if err != nil {
		t.Fatalf("failed to read object body: %v", err)
	}

	return string(content)
}

// TestAcceptanceBasicUpload tests basic file upload to LocalStack
func TestAcceptanceBasicUpload(t *testing.T) {
	suite := newAcceptanceTestSuite(t)

	// Create test file
	testContent := "Hello, LocalStack!"
	testFile := "test.txt"
	suite.createTestFile(t, testFile, testContent)

	// Configure uploader options
	originalOpts := opts
	defer func() { opts = originalOpts }()

	opts = &options{
		BucketName:   suite.bucket,
		Source:       suite.tempDir,
		CacheFile:    filepath.Join(suite.tempDir, ".go-s3-uploader.txt"),
		WorkersCount: 1,
		Region:       testRegion,
		doUpload:     true,
		doCache:      true,
		dryRun:       false,
	}

	// Override AWS endpoint for LocalStack
	os.Setenv("AWS_ENDPOINT_URL", localstackEndpoint)
	defer os.Unsetenv("AWS_ENDPOINT_URL")

	// Initialize AWS client pointing to LocalStack
	oldSess := sess
	oldS3svc := s3svc
	defer func() {
		sess = oldSess
		s3svc = oldS3svc
	}()

	// Reinitialize with LocalStack endpoint
	initAWSClient()

	// Run upload
	current, diff := filesLists()
	if len(diff) == 0 {
		t.Fatal("expected files to upload, got none")
	}

	s3put := s3putGen()
	src := newSourceFile(testFile)
	if err := s3put(src); err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// Verify file was uploaded
	if !suite.objectExists(t, testFile) {
		t.Errorf("expected object %s to exist in S3", testFile)
	}

	// Verify content matches
	uploadedContent := suite.getObjectContent(t, testFile)
	if uploadedContent != testContent {
		t.Errorf("content mismatch: got %q, want %q", uploadedContent, testContent)
	}

	// Save cache
	if err := current.Dump(opts.CacheFile); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	// Second run should skip upload (cached)
	_, diff2 := filesLists()
	if len(diff2) != 0 {
		t.Errorf("expected no files to upload on second run, got %d", len(diff2))
	}
}

// TestAcceptanceMultipleFiles tests uploading multiple files
func TestAcceptanceMultipleFiles(t *testing.T) {
	suite := newAcceptanceTestSuite(t)

	// Create multiple test files
	files := map[string]string{
		"file1.txt":           "content 1",
		"file2.txt":           "content 2",
		"subdir/file3.txt":    "content 3",
		"subdir/nested/f4.txt": "content 4",
	}

	for path, content := range files {
		suite.createTestFile(t, path, content)
	}

	// Configure uploader
	originalOpts := opts
	defer func() { opts = originalOpts }()

	opts = &options{
		BucketName:   suite.bucket,
		Source:       suite.tempDir,
		CacheFile:    filepath.Join(suite.tempDir, ".go-s3-uploader.txt"),
		WorkersCount: 2,
		Region:       testRegion,
		doUpload:     true,
		doCache:      true,
		dryRun:       false,
	}

	os.Setenv("AWS_ENDPOINT_URL", localstackEndpoint)
	defer os.Unsetenv("AWS_ENDPOINT_URL")

	oldSess := sess
	oldS3svc := s3svc
	defer func() {
		sess = oldSess
		s3svc = oldS3svc
	}()

	initAWSClient()

	// Upload all files
	current, diff := filesLists()
	if len(diff) != len(files) {
		t.Fatalf("expected %d files to upload, got %d", len(files), len(diff))
	}

	s3put := s3putGen()
	for _, fname := range diff {
		src := newSourceFile(fname)
		if err := s3put(src); err != nil {
			t.Errorf("upload failed for %s: %v", fname, err)
		}
	}

	// Verify all files exist
	for path := range files {
		if !suite.objectExists(t, path) {
			t.Errorf("expected object %s to exist in S3", path)
		}
	}

	// Save cache
	if err := current.Dump(opts.CacheFile); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}
}

// TestAcceptanceDryRun tests dry run mode
func TestAcceptanceDryRun(t *testing.T) {
	suite := newAcceptanceTestSuite(t)

	testFile := "dryrun-test.txt"
	suite.createTestFile(t, testFile, "dry run content")

	originalOpts := opts
	defer func() { opts = originalOpts }()

	opts = &options{
		BucketName:   suite.bucket,
		Source:       suite.tempDir,
		CacheFile:    filepath.Join(suite.tempDir, ".go-s3-uploader.txt"),
		WorkersCount: 1,
		Region:       testRegion,
		doUpload:     true,
		doCache:      true,
		dryRun:       true, // Dry run enabled
	}

	os.Setenv("AWS_ENDPOINT_URL", localstackEndpoint)
	defer os.Unsetenv("AWS_ENDPOINT_URL")

	oldSess := sess
	oldS3svc := s3svc
	oldAppEnv := appEnv
	defer func() {
		sess = oldSess
		s3svc = oldS3svc
		appEnv = oldAppEnv
	}()

	initAWSClient()

	// Run dry run upload
	_, diff := filesLists()
	if len(diff) == 0 {
		t.Fatal("expected files to upload, got none")
	}

	s3put := s3putGen()
	src := newSourceFile(testFile)
	if err := s3put(src); err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	// Verify file was NOT uploaded (dry run)
	// Wait a bit to ensure upload would have completed
	time.Sleep(100 * time.Millisecond)
	if suite.objectExists(t, testFile) {
		t.Errorf("object %s should not exist in dry run mode", testFile)
	}
}
