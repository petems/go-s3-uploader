package main

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Uploader abstracts S3 upload operations for testability.
// Implementations include the real AWS SDK v2 uploader and mock versions for testing.
type S3Uploader interface {
	// Upload uploads content to S3 with the specified parameters.
	Upload(ctx context.Context, input *UploadInput) (*UploadOutput, error)
}

// UploadInput contains the parameters for an S3 upload operation.
// This abstraction allows tests to verify upload parameters without
// depending directly on AWS SDK types.
type UploadInput struct {
	Bucket               string
	Key                  string
	Body                 io.Reader
	ContentType          *string
	ContentEncoding      *string
	CacheControl         *string
	ServerSideEncryption *string
}

// UploadOutput contains the result of an S3 upload operation.
type UploadOutput struct {
	Location  string
	VersionID *string
	ETag      *string
}

// S3UploaderSDK implements S3Uploader using the AWS SDK v2.
type S3UploaderSDK struct {
	uploader *manager.Uploader
}

// NewS3Uploader creates a new S3Uploader backed by the AWS SDK v2.
func NewS3Uploader(cfg aws.Config, optFns ...func(*manager.Uploader)) *S3UploaderSDK {
	client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(client, optFns...)
	return &S3UploaderSDK{uploader: uploader}
}

// NewS3UploaderWithClient creates a new S3Uploader with a custom S3 client.
// Useful for testing with custom endpoints (e.g., LocalStack).
func NewS3UploaderWithClient(client *s3.Client, optFns ...func(*manager.Uploader)) *S3UploaderSDK {
	uploader := manager.NewUploader(client, optFns...)
	return &S3UploaderSDK{uploader: uploader}
}

// Upload implements S3Uploader.Upload using the AWS SDK v2 manager.
func (u *S3UploaderSDK) Upload(ctx context.Context, input *UploadInput) (*UploadOutput, error) {
	sdkInput := &s3.PutObjectInput{
		Bucket: aws.String(input.Bucket),
		Key:    aws.String(input.Key),
		Body:   input.Body,
	}

	// Only set optional fields if they are provided
	if input.ContentType != nil {
		sdkInput.ContentType = input.ContentType
	}
	if input.ContentEncoding != nil {
		sdkInput.ContentEncoding = input.ContentEncoding
	}
	if input.CacheControl != nil {
		sdkInput.CacheControl = input.CacheControl
	}
	if input.ServerSideEncryption != nil {
		sdkInput.ServerSideEncryption = types.ServerSideEncryption(*input.ServerSideEncryption)
	}

	result, err := u.uploader.Upload(ctx, sdkInput)
	if err != nil {
		return nil, err
	}

	return &UploadOutput{
		Location:  result.Location,
		VersionID: result.VersionID,
		ETag:      result.ETag,
	}, nil
}
