package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Adapter implements ObjectStorage against any S3-compatible backend.
type S3Adapter struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

func NewS3Adapter(client *s3.Client, bucket string) *S3Adapter {
	return &S3Adapter{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    bucket,
	}
}

func (a *S3Adapter) PutObject(ctx context.Context, key string, data []byte, contentType string) error {
	reader := bytes.NewReader(data)
	contentLength := int64(len(data))

	input := &s3.PutObjectInput{
		Bucket:        aws.String(a.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(contentLength),
	}

	// Apply Cache-Control based on object type
	if strings.HasSuffix(key, ".m3u8") {
		input.CacheControl = aws.String("no-cache")
	} else if strings.HasSuffix(key, ".ts") || strings.HasSuffix(key, ".mp4") || strings.HasSuffix(key, ".jpg") || strings.HasSuffix(key, ".png") {
		input.CacheControl = aws.String("public, max-age=31536000, immutable")
	}

	// Disable automatic checksum calculation which forces aws-chunked transfer encoding.
	// Backblaze B2 does not support aws-chunked and rejects requests with
	// "x-amz-decoded-content-length" header. Setting WhenRequired disables this.
	_, err := a.client.PutObject(ctx, input, func(o *s3.Options) {
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	return err
}

// PutObjectStream streams data from r directly to S3 using the manager.Uploader
// which handles chunking and buffering internally, avoiding seekability issues.
// contentLength is kept for signature but manager uses Body.
func (a *S3Adapter) PutObjectStream(ctx context.Context, key string, r io.Reader, contentLength int64, contentType string) error {
	uploader := manager.NewUploader(a.client, func(u *manager.Uploader) {
		u.PartSize = 5 * 1024 * 1024 // 5 MB part size
		u.ClientOptions = append(u.ClientOptions, func(o *s3.Options) {
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		})
	})

	input := &s3.PutObjectInput{
		Bucket:      aws.String(a.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	}

	if strings.HasSuffix(key, ".m3u8") {
		input.CacheControl = aws.String("no-cache")
	} else if strings.HasSuffix(key, ".ts") || strings.HasSuffix(key, ".mp4") || strings.HasSuffix(key, ".jpg") || strings.HasSuffix(key, ".png") {
		input.CacheControl = aws.String("public, max-age=31536000, immutable")
	}

	_, err := uploader.Upload(ctx, input)
	return err
}

func (a *S3Adapter) GetObjectStream(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := a.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(key),
	}, func(o *s3.Options) {
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (a *S3Adapter) GetObject(ctx context.Context, key string) ([]byte, error) {
	body, err := a.GetObjectStream(ctx, key)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

func (a *S3Adapter) DeleteObject(ctx context.Context, key string) error {
	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (a *S3Adapter) GetPresignedURL(ctx context.Context, key string) (string, error) {
	req, err := a.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (a *S3Adapter) GetPresignedUploadURL(ctx context.Context, key, contentType string) (string, error) {
	req, err := a.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}


// CheckACL verifies required bucket permissions exist (call at startup).
func (a *S3Adapter) CheckACL(ctx context.Context) error {
	_, err := a.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(a.bucket),
	})
	if err != nil {
		return fmt.Errorf("storage: cannot access bucket %q: %w", a.bucket, err)
	}
	return nil
}

// Ensure S3Adapter implements ObjectStorage at compile time.
var _ interface{ PutObject(context.Context, string, []byte, string) error } = (*S3Adapter)(nil)

// keep awss3types imported (used for compile-time check guard)
var _ = awss3types.Object{}
