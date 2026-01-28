package main

import (
	"io"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3Client defines the interface for S3 operations
type S3Client interface {
	Upload(input *S3UploadInput) error
}

// S3UploadInput contains the parameters for uploading an object to S3
type S3UploadInput struct {
	Bucket               *string
	Key                  *string
	Body                 io.Reader
	ContentType          *string
	ContentEncoding      *string
	CacheControl         *string
	ServerSideEncryption *string
}

// RealS3Client wraps the AWS S3 uploader
type RealS3Client struct {
	uploader *s3manager.Uploader
}

// NewRealS3Client creates a new real S3 client
func NewRealS3Client(uploader *s3manager.Uploader) *RealS3Client {
	return &RealS3Client{uploader: uploader}
}

// Upload uploads an object to S3
func (c *RealS3Client) Upload(input *S3UploadInput) error {
	uploadInput := &s3manager.UploadInput{
		Bucket: input.Bucket,
		Key:    input.Key,
		Body:   input.Body,
	}

	// Set optional fields only if provided (consistent nil checking)
	if input.ContentType != nil {
		uploadInput.ContentType = input.ContentType
	}

	if input.ContentEncoding != nil {
		uploadInput.ContentEncoding = input.ContentEncoding
	}

	if input.CacheControl != nil {
		uploadInput.CacheControl = input.CacheControl
	}

	if input.ServerSideEncryption != nil {
		uploadInput.ServerSideEncryption = input.ServerSideEncryption
	}

	_, err := c.uploader.Upload(uploadInput)
	return err
}
