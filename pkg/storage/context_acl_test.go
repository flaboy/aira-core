package storage

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type aclTestHTTPClient func(*http.Request) (*http.Response, error)

func (f aclTestHTTPClient) Do(r *http.Request) (*http.Response, error) { return f(r) }
func TestContextObjectACL(t *testing.T) {
	var calls atomic.Int32
	client := s3.New(s3.Options{Region: "unit-test-1", Credentials: credentials.NewStaticCredentialsProvider("unit", "unit", ""), HTTPClient: aclTestHTTPClient(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		if req.Header.Get("X-Amz-Acl") != "public-read" {
			t.Error("ACL contract changed")
		}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})})
	store := &S3Storage{client: client, bucket: "unit-bucket"}
	if err := store.SetObjectACLContext(context.Background(), "owned/file.png", "invalid"); err == nil {
		t.Fatal("invalid type accepted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.SetObjectACLContext(cancelled, "owned/file.png", types.ObjectCannedACLPublicRead); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancelled context", err)
	}
	if calls.Load() != 0 {
		t.Fatal("cancelled/invalid request used transport")
	}
	bounded, stop := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer stop()
	if err := store.SetObjectACLContext(bounded, "owned/file.png/", types.ObjectCannedACLPublicRead); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("deadline not propagated", err)
	}
	if calls.Load() != 1 {
		t.Fatal("unexpected retry", calls.Load())
	}
	local := NewLocalStorage(t.TempDir(), "")
	if err := local.SetObjectACLContext(context.Background(), "owned/file.png", nil); err != nil {
		t.Fatal(err)
	}
	if err := local.SetObjectACLContext(cancelled, "owned/file.png", nil); !errors.Is(err, context.Canceled) {
		t.Fatal("local ignored cancel")
	}
	var old Storage = store
	_ = old
	var _ aws.HTTPClient = aclTestHTTPClient(nil)
}
