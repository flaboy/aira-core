package storage

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"strings"
)

// ContextObjectACL是当前Mockup发布所需的可选能力，不改变既有Storage接口。
type ContextObjectACL interface {
	SetObjectACLContext(context.Context, string, interface{}) error
}

var _ ContextObjectACL = (*LocalStorage)(nil)
var _ ContextObjectACL = (*S3Storage)(nil)

func (s *LocalStorage) SetObjectACLContext(ctx context.Context, path string, acl interface{}) error {
	return ctx.Err()
}
func (s *S3Storage) SetObjectACLContext(ctx context.Context, path string, acl interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value, ok := acl.(types.ObjectCannedACL)
	if !ok {
		return fmt.Errorf("invalid ACL type")
	}
	_, err := s.client.PutObjectAcl(ctx, &s3.PutObjectAclInput{Bucket: aws.String(s.bucket), Key: aws.String(strings.TrimSuffix(path, "/")), ACL: value})
	if err != nil {
		return fmt.Errorf("set object ACL failed: %w", err)
	}
	return nil
}
