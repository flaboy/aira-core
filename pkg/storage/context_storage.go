package storage

import (
	"context"
	"io"
)

// ContextStorage 为需要可取消IO的新调用方提供可选契约，不改变既有Storage接口。
type ContextStorage interface {
	PutObjectContext(context.Context, io.Reader, string, ...PutOption) error
	OpenContext(context.Context, string) (io.ReadCloser, error)
	DeleteObjectContext(context.Context, string) error
}

var _ ContextStorage = (*LocalStorage)(nil)
var _ ContextStorage = (*S3Storage)(nil)

type storageContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r storageContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
