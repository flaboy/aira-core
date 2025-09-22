package storage

import (
	"context"
	"io"
	"time"
)

type PutOption struct {
	Key   string
	Value interface{}
}

type GetOption struct {
	Key   string
	Value interface{}
}

var (
	OptMimeType = "ContentType"
	OptExpires  = "Expires"
)

// WithMimeType 设置文件的MIME类型
func WithMimeType(mimeType string) PutOption {
	return PutOption{
		Key:   OptMimeType,
		Value: mimeType,
	}
}

func WithThumbnailSize(size uint) GetOption {
	return GetOption{
		Key:   "ThumbnailSize",
		Value: size,
	}
}

// WithExpires 设置URL的过期时间
func WithExpires(duration time.Duration) GetOption {
	return GetOption{
		Key:   OptExpires,
		Value: duration,
	}
}

// UploadContext 定义上传上下文
type UploadContext struct {
	Mode string      `json:"mode"`
	Data interface{} `json:"data"`
}

type UploadResult struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

// Storage 定义存储接口
type Storage interface {
	// PutObject 上传对象
	PutObject(file io.Reader, path string, opts ...PutOption) error

	// DeleteObject 删除对象
	DeleteObject(path string) error

	// GetUrl 获取对象的访问URL
	GetURL(path string, opts ...GetOption) string

	GetThumbnailURL(path string, opts ...GetOption) string

	// Open 打开文件进行读取
	Open(path string) (io.ReadCloser, error)

	// List 列出指定路径下的所有文件
	List(path string) ([]string, error)

	// Move 移动对象
	Move(src, dst string) error

	// GetUploadContext 获取上传上下文
	GetUploadContext(ctx context.Context, path string) (*UploadContext, error)
}
