package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestContextStorageLocal(t *testing.T) {
	store := NewLocalStorage(t.TempDir(), "")
	ctx, cancel := context.WithCancel(t.Context())
	if err := store.PutObjectContext(ctx, bytes.NewReader([]byte("original")), "task/original.bin"); err != nil {
		t.Fatal(err)
	}
	reader, err := store.OpenContext(ctx, "task/original.bin")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	defer reader.Close()
	if _, err = reader.Read(make([]byte, 8)); !errors.Is(err, context.Canceled) {
		t.Fatalf("read ignores cancel %v", err)
	}
	if err = store.DeleteObjectContext(ctx, "task/original.bin"); !errors.Is(err, context.Canceled) {
		t.Fatalf("delete ignores cancel %v", err)
	}
	if err = store.PutObjectContext(ctx, bytes.NewReader([]byte("new")), "forbidden.bin"); !errors.Is(err, context.Canceled) {
		t.Fatalf("put ignores cancel %v", err)
	}
	if r, err := store.Open("forbidden.bin"); err == nil {
		r.Close()
		t.Fatal("canceled put wrote object")
	}
	if err = store.DeleteObjectContext(t.Context(), "task/original.bin"); err != nil {
		t.Fatal(err)
	}
	if err = store.DeleteObjectContext(t.Context(), "task/original.bin"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing delete error lost %v", err)
	}
}

type storageTestTransport struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (t *storageTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.once.Do(func() { close(t.started) })
	select {
	case <-request.Context().Done():
		return nil, request.Context().Err()
	case <-t.release:
		return nil, errors.New("test transport released")
	}
}

// 验证实际SDK把调用方ctx传到HTTP传输边界；不是网络/S3服务验收。
func TestContextStorageS3ContextPropagation(t *testing.T) {
	for _, method := range []string{"put", "open", "delete"} {
		t.Run(method, func(t *testing.T) {
			transport := &storageTestTransport{started: make(chan struct{}), release: make(chan struct{})}
			defer close(transport.release)
			client := s3.New(s3.Options{Region: "test", BaseEndpoint: aws.String("http://storage.invalid"), UsePathStyle: true, Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), HTTPClient: &http.Client{Transport: transport}})
			store := &S3Storage{client: client, bucket: "test"}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				switch method {
				case "put":
					err = store.PutObjectContext(ctx, bytes.NewReader([]byte("x")), "file")
				case "open":
					var reader io.ReadCloser
					reader, err = store.OpenContext(ctx, "file")
					if reader != nil {
						reader.Close()
					}
				case "delete":
					err = store.DeleteObjectContext(ctx, "file")
				}
				done <- err
			}()
			select {
			case <-transport.started:
			case err := <-done:
				t.Fatalf("request did not start: %v", err)
			case <-time.After(time.Second):
				t.Fatal("request start timeout")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("SDK context lost %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("SDK transport ignored cancellation")
			}
		})
	}
}
