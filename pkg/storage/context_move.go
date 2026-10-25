package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ContextFileMover 只移动一个已知文件，不改变Storage及既有目录Move契约。
type ContextFileMover interface {
	MoveFileContext(context.Context, string, string) error
}

var _ ContextFileMover = (*LocalStorage)(nil)
var _ ContextFileMover = (*S3Storage)(nil)

func (s *LocalStorage) MoveFileContext(ctx context.Context, src, dst string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Move(src, dst)
}

func (s *S3Storage) MoveFileContext(ctx context.Context, src, dst string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	src = strings.TrimPrefix(src, "/")
	dst = strings.TrimPrefix(dst, "/")
	input := &s3.CopyObjectInput{
		Bucket: aws.String(s.bucket), CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, src)), Key: aws.String(dst),
	}
	if s.public {
		input.ACL = types.ObjectCannedACLPublicRead
	}
	if _, err := s.client.CopyObject(ctx, input); err != nil {
		return fmt.Errorf("copy S3 file: %w", err)
	}
	if err := s.DeleteObjectContext(ctx, src); err != nil {
		return fmt.Errorf("delete source S3 file: %w", err)
	}
	return nil
}
