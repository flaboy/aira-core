package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func (s *S3Storage) PutObjectContext(ctx context.Context, reader io.Reader, path string, opts ...PutOption) error {
	input := &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(strings.TrimSuffix(path, "/")), Body: reader}
	if s.public {
		input.ACL = types.ObjectCannedACLPublicRead
	}
	for _, option := range opts {
		if option.Key == OptMimeType {
			if value, ok := option.Value.(string); ok {
				input.ContentType = aws.String(value)
			}
		}
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put S3 object: %w", err)
	}
	return nil
}

func (s *S3Storage) OpenContext(ctx context.Context, path string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(path)})
	if err != nil {
		return nil, fmt.Errorf("get S3 object: %w", err)
	}
	return result.Body, nil
}

func (s *S3Storage) DeleteObjectContext(ctx context.Context, path string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(path)}); err != nil {
		return fmt.Errorf("delete S3 object: %w", err)
	}
	return nil
}
