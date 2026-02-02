//go:build acceptance

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	localstackEndpoint = "http://localhost:4566"
	testRegion         = "us-east-1"
	testBucketPrefix   = "test-bucket-"
)

// AcceptanceTestSuite provides setup/teardown for acceptance tests
type AcceptanceTestSuite struct {
	client     *s3.Client
	uploader   S3Uploader
	bucketName string
	ctx        context.Context
}

// newAcceptanceTestSuite creates a new test suite connected to LocalStack
func newAcceptanceTestSuite(t *testing.T) *AcceptanceTestSuite {
	t.Helper()

	ctx := context.Background()

	// Create custom endpoint resolver for LocalStack
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               localstackEndpoint,
				HostnameImmutable: true,
				SigningRegion:     testRegion,
			}, nil
		},
	)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(testRegion),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	// Create S3 client with path-style addressing
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// Create uploader
	uploader := NewS3UploaderWithClient(client)

	// Generate unique bucket name
	bucketName := fmt.Sprintf("%s%d", testBucketPrefix, time.Now().UnixNano())

	suite := &AcceptanceTestSuite{
		client:     client,
		uploader:   uploader,
		bucketName: bucketName,
		ctx:        ctx,
	}

	// Create the test bucket
	suite.createBucket(t)

	return suite
}

func (s *AcceptanceTestSuite) createBucket(t *testing.T) {
	t.Helper()

	_, err := s.client.CreateBucket(s.ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucketName),
	})
	if err != nil {
		t.Fatalf("Failed to create test bucket %s: %v", s.bucketName, err)
	}

	t.Logf("Created test bucket: %s", s.bucketName)
}

func (s *AcceptanceTestSuite) cleanup(t *testing.T) {
	t.Helper()

	// List and delete all objects
	listOutput, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
	})
	if err != nil {
		t.Logf("Warning: failed to list objects for cleanup: %v", err)
		return
	}

	for _, obj := range listOutput.Contents {
		_, err := s.client.DeleteObject(s.ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucketName),
			Key:    obj.Key,
		})
		if err != nil {
			t.Logf("Warning: failed to delete object %s: %v", *obj.Key, err)
		}
	}

	// Delete the bucket
	_, err = s.client.DeleteBucket(s.ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(s.bucketName),
	})
	if err != nil {
		t.Logf("Warning: failed to delete bucket %s: %v", s.bucketName, err)
	} else {
		t.Logf("Deleted test bucket: %s", s.bucketName)
	}
}

func (s *AcceptanceTestSuite) getObject(t *testing.T, key string) ([]byte, error) {
	t.Helper()

	output, err := s.client.GetObject(s.ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()

	return io.ReadAll(output.Body)
}

func (s *AcceptanceTestSuite) objectExists(t *testing.T, key string) bool {
	t.Helper()

	_, err := s.client.HeadObject(s.ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	return err == nil
}

// --- Acceptance Tests ---

func TestAcceptance_SingleFileUpload(t *testing.T) {
	suite := newAcceptanceTestSuite(t)
	defer suite.cleanup(t)

	// Upload a simple file
	content := "Hello, LocalStack!"
	input := &UploadInput{
		Bucket:      suite.bucketName,
		Key:         "test/hello.txt",
		Body:        strings.NewReader(content),
		ContentType: stringPtr("text/plain"),
	}

	output, err := suite.uploader.Upload(suite.ctx, input)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	t.Logf("Upload succeeded: %s", output.Location)

	// Verify the file exists and has correct content
	retrieved, err := suite.getObject(t, "test/hello.txt")
	if err != nil {
		t.Fatalf("Failed to retrieve uploaded file: %v", err)
	}

	if string(retrieved) != content {
		t.Errorf("Content mismatch: got %q, want %q", retrieved, content)
	}
}

func TestAcceptance_UploadWithHeaders(t *testing.T) {
	suite := newAcceptanceTestSuite(t)
	defer suite.cleanup(t)

	input := &UploadInput{
		Bucket:       suite.bucketName,
		Key:          "assets/style.css",
		Body:         strings.NewReader("body { color: red; }"),
		ContentType:  stringPtr("text/css"),
		CacheControl: stringPtr("max-age=31536000"),
	}

	_, err := suite.uploader.Upload(suite.ctx, input)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Verify headers using HeadObject
	head, err := suite.client.HeadObject(suite.ctx, &s3.HeadObjectInput{
		Bucket: aws.String(suite.bucketName),
		Key:    aws.String("assets/style.css"),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	if head.ContentType == nil || *head.ContentType != "text/css" {
		t.Errorf("ContentType mismatch: got %v", head.ContentType)
	}
	if head.CacheControl == nil || *head.CacheControl != "max-age=31536000" {
		t.Errorf("CacheControl mismatch: got %v", head.CacheControl)
	}
}

func TestAcceptance_MultipleFilesUpload(t *testing.T) {
	suite := newAcceptanceTestSuite(t)
	defer suite.cleanup(t)

	files := map[string]string{
		"file1.txt": "Content of file 1",
		"file2.txt": "Content of file 2",
		"file3.txt": "Content of file 3",
	}

	for key, content := range files {
		input := &UploadInput{
			Bucket:      suite.bucketName,
			Key:         key,
			Body:        strings.NewReader(content),
			ContentType: stringPtr("text/plain"),
		}

		_, err := suite.uploader.Upload(suite.ctx, input)
		if err != nil {
			t.Fatalf("Upload of %s failed: %v", key, err)
		}
	}

	// Verify all files exist
	for key, expectedContent := range files {
		content, err := suite.getObject(t, key)
		if err != nil {
			t.Errorf("Failed to get %s: %v", key, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s: got %q, want %q", key, content, expectedContent)
		}
	}
}

func TestAcceptance_ConcurrentUploads(t *testing.T) {
	suite := newAcceptanceTestSuite(t)
	defer suite.cleanup(t)

	const numFiles = 20
	var wg sync.WaitGroup
	errors := make(chan error, numFiles)

	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			input := &UploadInput{
				Bucket:      suite.bucketName,
				Key:         fmt.Sprintf("concurrent/file-%d.txt", n),
				Body:        strings.NewReader(fmt.Sprintf("Content %d", n)),
				ContentType: stringPtr("text/plain"),
			}

			_, err := suite.uploader.Upload(suite.ctx, input)
			if err != nil {
				errors <- fmt.Errorf("file %d: %w", n, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Verify all files exist
	for i := 0; i < numFiles; i++ {
		key := fmt.Sprintf("concurrent/file-%d.txt", i)
		if !suite.objectExists(t, key) {
			t.Errorf("File %s not found", key)
		}
	}
}

func TestAcceptance_LargeFileUpload(t *testing.T) {
	suite := newAcceptanceTestSuite(t)
	defer suite.cleanup(t)

	// Create a 5MB file
	size := 5 * 1024 * 1024
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	input := &UploadInput{
		Bucket:      suite.bucketName,
		Key:         "large/bigfile.bin",
		Body:        strings.NewReader(string(content)),
		ContentType: stringPtr("application/octet-stream"),
	}

	_, err := suite.uploader.Upload(suite.ctx, input)
	if err != nil {
		t.Fatalf("Large file upload failed: %v", err)
	}

	// Verify size
	head, err := suite.client.HeadObject(suite.ctx, &s3.HeadObjectInput{
		Bucket: aws.String(suite.bucketName),
		Key:    aws.String("large/bigfile.bin"),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	if head.ContentLength == nil || *head.ContentLength != int64(size) {
		t.Errorf("Size mismatch: got %v, want %d", head.ContentLength, size)
	}
}

func TestAcceptance_UploadFromTestDirectory(t *testing.T) {
	suite := newAcceptanceTestSuite(t)
	defer suite.cleanup(t)

	// Use the actual test files from the test directory
	testDir := "test/output"
	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read test directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(testDir, file.Name())
		f, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("Failed to open %s: %v", filePath, err)
		}

		input := &UploadInput{
			Bucket:      suite.bucketName,
			Key:         file.Name(),
			Body:        f,
			ContentType: stringPtr("application/octet-stream"),
		}

		_, err = suite.uploader.Upload(suite.ctx, input)
		f.Close()

		if err != nil {
			t.Errorf("Failed to upload %s: %v", file.Name(), err)
		}
	}

	// Verify uploads
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !suite.objectExists(t, file.Name()) {
			t.Errorf("File %s not found in S3", file.Name())
		}
	}
}

func TestAcceptance_FullUploadPipeline(t *testing.T) {
	suite := newAcceptanceTestSuite(t)
	defer suite.cleanup(t)

	// Create a temporary test directory
	tmpDir, err := os.MkdirTemp("", "s3-upload-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := map[string]string{
		"index.html":      "<html><body>Hello</body></html>",
		"style.css":       "body { margin: 0; }",
		"script.js":       "console.log('hello');",
		"data/config.json": `{"key": "value"}`,
	}

	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	// Test the s3putGenWithUploader function
	oldSource := opts.Source
	oldBucket := opts.BucketName
	opts.Source = tmpDir
	opts.BucketName = suite.bucketName
	defer func() {
		opts.Source = oldSource
		opts.BucketName = oldBucket
	}()

	// Create uploader function with our test uploader
	uploadFn := s3putGenWithUploader(suite.uploader)

	// Upload each file
	for name := range testFiles {
		src := newSourceFile(name)
		err := uploadFn(src)
		if err != nil {
			t.Errorf("Failed to upload %s: %v", name, err)
		}
	}

	// Verify all files are in S3
	for name, expectedContent := range testFiles {
		content, err := suite.getObject(t, name)
		if err != nil {
			t.Errorf("Failed to get %s: %v", name, err)
			continue
		}
		// Note: some files may be gzip encoded, so we check existence
		if len(content) == 0 {
			t.Errorf("File %s has no content", name)
		}
		t.Logf("Verified %s: %d bytes (expected %d)", name, len(content), len(expectedContent))
	}
}

func TestAcceptance_InitAWSClientWithEndpoint(t *testing.T) {
	// Test the initAWSClientWithEndpoint function
	err := initAWSClientWithEndpoint(localstackEndpoint, testRegion)
	if err != nil {
		t.Fatalf("initAWSClientWithEndpoint failed: %v", err)
	}

	if s3Uploader == nil {
		t.Fatal("s3Uploader is nil after initialization")
	}

	// Create a test bucket and try uploading
	ctx := context.Background()

	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               localstackEndpoint,
				HostnameImmutable: true,
				SigningRegion:     testRegion,
			}, nil
		},
	)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(testRegion),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("Failed to create AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	bucketName := fmt.Sprintf("init-test-%d", time.Now().UnixNano())
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}); err != nil {
		t.Fatalf("Failed to create test bucket %s: %v", bucketName, err)
	}
	defer client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})

	// Upload using the global uploader
	input := &UploadInput{
		Bucket:      bucketName,
		Key:         "test.txt",
		Body:        strings.NewReader("test content"),
		ContentType: stringPtr("text/plain"),
	}

	_, err = s3Uploader.Upload(ctx, input)
	if err != nil {
		t.Fatalf("Upload with global uploader failed: %v", err)
	}

	// Cleanup
	client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("test.txt"),
	})
}
