package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type contextMoveTransport struct {
	mu        sync.Mutex
	methods   []string
	block     string
	copyFails bool
	started   chan struct{}
	once      sync.Once
}

func (transport *contextMoveTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.mu.Lock()
	transport.methods = append(transport.methods, request.Method)
	transport.mu.Unlock()
	if request.Method == transport.block {
		transport.once.Do(func() { close(transport.started) })
		<-request.Context().Done()
		return nil, request.Context().Err()
	}
	status, body := http.StatusOK, `<CopyObjectResult><ETag>"test"</ETag><LastModified>2026-10-25T00:00:00Z</LastModified></CopyObjectResult>`
	if request.Method == http.MethodPut {
		if request.Header.Get("X-Amz-Copy-Source") != "test/source.txt" || request.Header.Get("X-Amz-Acl") != "public-read" {
			return nil, errors.New("copy source or ACL missing")
		}
		if transport.copyFails {
			status, body = http.StatusNotFound, `<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`
		}
	} else if request.Method == http.MethodDelete {
		status, body = http.StatusNoContent, ""
	} else {
		return nil, errors.New("single file move attempted non-copy/delete operation")
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}
func contextMoveTestS3(transport *contextMoveTransport) *S3Storage {
	return &S3Storage{client: s3.New(s3.Options{Region: "test", BaseEndpoint: aws.String("http://storage.invalid"), UsePathStyle: true, Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), HTTPClient: &http.Client{Transport: transport}, RetryMaxAttempts: 1}), bucket: "test", public: true}
}
func TestMoveFileContextS3OnlyCopiesThenDeletes(t *testing.T) {
	for _, copyFails := range []bool{false, true} {
		transport := &contextMoveTransport{copyFails: copyFails}
		err := contextMoveTestS3(transport).MoveFileContext(t.Context(), "/source.txt", "/destination.txt")
		want := []string{http.MethodPut, http.MethodDelete}
		if copyFails {
			want = []string{http.MethodPut}
		}
		if (err != nil) != copyFails || !reflect.DeepEqual(transport.methods, want) {
			t.Fatalf("copyFails=%v methods=%v err=%v", copyFails, transport.methods, err)
		}
	}
}
func TestMoveFileContextS3CancellationReachesCopyAndDelete(t *testing.T) {
	for _, stage := range []string{http.MethodPut, http.MethodDelete} {
		t.Run(stage, func(t *testing.T) {
			transport := &contextMoveTransport{block: stage, started: make(chan struct{})}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- contextMoveTestS3(transport).MoveFileContext(ctx, "source.txt", "destination.txt") }()
			select {
			case <-transport.started:
			case err := <-done:
				t.Fatalf("SDK never reached stage: %v", err)
			case <-time.After(time.Second):
				t.Fatal("start timeout")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("SDK ignored cancellation")
			}
			want := []string{http.MethodPut}
			if stage == http.MethodDelete {
				want = append(want, http.MethodDelete)
			}
			transport.mu.Lock()
			actual := append([]string(nil), transport.methods...)
			transport.mu.Unlock()
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("unexpected request after canceled stage: %v", actual)
			}
		})
	}
}
func TestMoveFileContextS3DeadlineBoundsSlowCopy(t *testing.T) {
	transport := &contextMoveTransport{block: http.MethodPut, started: make(chan struct{})}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	err := contextMoveTestS3(transport).MoveFileContext(ctx, "source.txt", "destination.txt")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline lost: %v", err)
	}
	if !reflect.DeepEqual(transport.methods, []string{http.MethodPut}) {
		t.Fatalf("request after timed out copy: %v", transport.methods)
	}
}
func TestMoveFileContextPreCanceledSkipsRemoteIO(t *testing.T) {
	transport := &contextMoveTransport{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := contextMoveTestS3(transport).MoveFileContext(ctx, "source.txt", "destination.txt"); !errors.Is(err, context.Canceled) || len(transport.methods) != 0 {
		t.Fatalf("canceled move issued requests: %v %v", transport.methods, err)
	}
}
func TestMoveFileContextLocalKeepsCanceledSourceAndMovesSingleFile(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalStorage(dir, "")
	if err := os.WriteFile(filepath.Join(dir, "source.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := store.MoveFileContext(ctx, "source.txt", "target/file.txt"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "source.txt")); err != nil {
		t.Fatal("canceled move lost source", err)
	}
	if err := store.MoveFileContext(t.Context(), "source.txt", "target/file.txt"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "target/file.txt")); err != nil || string(data) != "content" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}
