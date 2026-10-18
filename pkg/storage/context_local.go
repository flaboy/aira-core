package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type storageContextReadCloser struct {
	storageContextReader
	close func() error
}

func (r *storageContextReadCloser) Close() error { return r.close() }

func (s *LocalStorage) OpenContext(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(s.basePath, path))
	if err != nil {
		return nil, fmt.Errorf("open local object: %w", err)
	}
	stop := context.AfterFunc(ctx, func() { _ = file.Close() })
	return &storageContextReadCloser{storageContextReader: storageContextReader{ctx: ctx, reader: file}, close: func() error { stop(); return file.Close() }}, nil
}

func (s *LocalStorage) PutObjectContext(ctx context.Context, reader io.Reader, path string, _ ...PutOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fullPath := filepath.Join(s.basePath, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create local object parent: %w", err)
	}
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create local object: %w", err)
	}
	defer file.Close()
	stop := context.AfterFunc(ctx, func() { _ = file.Close() })
	defer stop()
	_, err = io.Copy(file, storageContextReader{ctx: ctx, reader: reader})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("write local object: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close local object: %w", err)
	}
	return nil
}

func (s *LocalStorage) DeleteObjectContext(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.basePath, path)); err != nil {
		return fmt.Errorf("delete local object: %w", err)
	}
	return nil
}
