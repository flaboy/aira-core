package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/flaboy/aira-core/pkg/redis"

	"github.com/google/uuid"
)

type LocalStorage struct {
	basePath string
	baseURL  string
}

func NewLocalStorage(basePath, baseURL string) *LocalStorage {
	return &LocalStorage{
		basePath: basePath,
		baseURL:  strings.TrimRight(baseURL, "/"),
	}
}

func (s *LocalStorage) Open(path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open file failedb: %w", err)
	}
	return file, nil
}

func (s *LocalStorage) List(path string) ([]string, error) {
	fullPath := filepath.Join(s.basePath, path)
	var files []string

	err := filepath.Walk(fullPath, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录本身
		if walkPath == fullPath {
			return nil
		}

		// 将完整路径转换为相对于存储根目录的路径
		relPath, err := filepath.Rel(s.basePath, walkPath)
		if err != nil {
			return err
		}

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			// 如果目录不存在，返回空列表
			return []string{}, nil
		}
		return nil, fmt.Errorf("walk directory failed: %w", err)
	}

	return files, nil
}

func (s *LocalStorage) PutObject(file io.Reader, path string, opts ...PutOption) error {
	fullPath := filepath.Join(s.basePath, path)
	fmt.Println("fullPath", fullPath)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	// 创建目标文件
	dst, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		return err
	}

	return nil
}

func (s *LocalStorage) DeleteObject(path string) error {
	fullPath := filepath.Join(s.basePath, path)
	if err := os.Remove(fullPath); err != nil {
		return err
	}
	return nil
}

func (s *LocalStorage) GetURL(path string, opts ...GetOption) string {
	// 本地存储实现忽略所有选项，直接返回URL
	return fmt.Sprintf("%s/%s", s.baseURL, path)
}

func (s *LocalStorage) GetThumbnailURL(path string, opts ...GetOption) string {
	// 本地存储实现忽略所有选项，直接返回URL
	return fmt.Sprintf("%s/%s", s.baseURL, path)
}

func (s *LocalStorage) Move(src, dst string) error {
	srcPath := filepath.Join(s.basePath, src)
	dstPath := filepath.Join(s.basePath, dst)

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	if err := os.Rename(srcPath, dstPath); err != nil {
		return err
	}
	return nil
}

func (s *LocalStorage) GetUploadContext(ctx context.Context, path string) (*UploadContext, error) {
	id := uuid.New().String()
	smtp := redis.RedisClient.Set(ctx, id, path, 0)
	if smtp.Err() != nil {
		return nil, smtp.Err()
	}
	return &UploadContext{
		Mode: "local",
		Data: map[string]interface{}{
			"token": id,
		},
	}, nil
}

// SetObjectACL is a no-op for local storage (ACL not applicable)
func (s *LocalStorage) SetObjectACL(path string, acl interface{}) error {
	// Local storage doesn't support ACL, so this is a no-op
	return nil
}

func (s *LocalStorage) Output(path string, req *http.Request, w http.ResponseWriter) error {
	fullPath := filepath.Join(s.basePath, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	http.ServeContent(w, req, filepath.Base(fullPath), fileInfo.ModTime(), file)
	return nil
}

func (s *LocalStorage) UploadHttpRequest(req *http.Request) (map[string]interface{}, error) {
	log.Println("=== Local Storage Upload Debug ===")

	// 检查路径参数
	token := req.FormValue("token")
	log.Printf("1. Token: %s", token)
	if token == "" {
		return nil, errors.New("path is required")
	}

	id := redis.RedisClient.Get(req.Context(), token)
	if id.Err() != nil {
		log.Printf("2. Redis error: %v", id.Err())
		return nil, id.Err()
	}
	path := id.Val()
	log.Printf("3. Base path from token: %s", path)
	if path == "" {
		return nil, errors.New("path is required")
	}

	// 获取上传文件
	file, header, err := req.FormFile("file")
	if err != nil {
		log.Printf("4. FormFile error: %v", err)
		return nil, err
	}
	defer file.Close()
	log.Printf("5. File received: %s (size: %d bytes)", header.Filename, header.Size)

	// 获取文件类型
	ext := filepath.Ext(header.Filename)
	contentType := header.Header.Get("Content-Type")
	log.Printf("6. Content-Type: %s, Extension: %s", contentType, ext)

	// 检查是否保留原始文件名
	keepFilename := req.FormValue("keep_filename")
	filename := req.FormValue("filename")
	log.Printf("7. keep_filename: %s, filename: %s", keepFilename, filename)

	if keepFilename == "true" && filename != "" {
		path = filepath.Join(path, filename)
		log.Printf("8. Using original filename, final path: %s", path)
	} else {
		path = filepath.Join(path, uuid.New().String()+ext)
		log.Printf("8. Using UUID filename, final path: %s", path)
	}

	// 上传文件到存储
	log.Printf("9. Uploading to: %s", path)
	err = s.PutObject(file, path, WithMimeType(contentType))
	if err != nil {
		log.Printf("10. PutObject error: %v", err)
		return nil, err
	}

	url := s.GetURL(path)
	log.Printf("11. Upload successful! Path: %s, URL: %s", path, url)

	// 返回上传结果
	return map[string]interface{}{
		"path": path,
		"url":  url,
	}, nil
}
