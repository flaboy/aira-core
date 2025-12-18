package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3TokenData struct {
	Policy     string `json:"policy"`
	AccessID   string `json:"accessid"`
	Host       string `json:"host"`
	Signature  string `json:"signature"`
	Dir        string `json:"dir"`
	Expiration string `json:"expiration"`
	Public     bool   `json:"public"` // 标识存储是否为 public，前端上传时需要设置 ACL
}

type S3Storage struct {
	client    *s3.Client
	bucket    string
	region    string
	endpoint  string
	publicURL string
	accessKey string
	secretKey string
	public    bool
}

func NewS3Storage(accessKey, secretKey, bucket, region, endpoint, publicURL string, public bool) (*S3Storage, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint}, nil
			})),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	if endpoint == "" {
		endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", region)
	}
	if publicURL == "" {
		publicURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucket, region)
	}

	if !strings.HasSuffix(publicURL, "/") {
		publicURL = publicURL + "/"
	}

	client := s3.NewFromConfig(cfg)
	return &S3Storage{
		client:    client,
		bucket:    bucket,
		region:    region,
		endpoint:  endpoint,
		public:    public,
		publicURL: publicURL,
		accessKey: accessKey,
		secretKey: secretKey,
	}, nil
}

func (s *S3Storage) PutObject(file io.Reader, path string, opts ...PutOption) error {
	path = strings.TrimSuffix(path, "/")
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
		Body:   file,
	}

	if s.public {
		input.ACL = types.ObjectCannedACLPublicRead
	}

	// 处理选项
	for _, opt := range opts {
		if opt.Key == OptMimeType {
			if contentType, ok := opt.Value.(string); ok {
				input.ContentType = aws.String(contentType)
			}
		}
	}

	_, err := s.client.PutObject(context.TODO(), input)
	if err != nil {
		return err
	}

	return nil
}

func (s *S3Storage) Open(path string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	resp, err := s.client.GetObject(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("get object failed: %w", err)
	}

	return resp.Body, nil
}

func (s *S3Storage) List(path string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(path),
	}

	resp, err := s.client.ListObjectsV2(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("list objects failed: %w", err)
	}

	var keys []string
	for _, obj := range resp.Contents {
		keys = append(keys, *obj.Key)
	}

	return keys, nil
}

func (s *S3Storage) DeleteObject(path string) error {
	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3Storage) GetURL(path string, opts ...GetOption) string {
	path = strings.TrimSuffix(path, "/")
	if s.public {
		return fmt.Sprintf("%s%s", s.publicURL, path)
	}

	// 处理预签名URL
	var expires time.Duration
	for _, opt := range opts {
		if opt.Key == OptExpires {
			if duration, ok := opt.Value.(time.Duration); ok {
				expires = duration
			}
		}
	}

	presignClient := s3.NewPresignClient(s.client)
	presignedURL, err := presignClient.PresignGetObject(context.TODO(),
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(path),
		},
		s3.WithPresignExpires(expires),
	)
	if err != nil {
		// 如果预签名失败，返回公开URL
		return fmt.Sprintf("%s/%s", s.publicURL, path)
	}

	return presignedURL.URL
}

func (s *S3Storage) GetThumbnailURL(path string, opts ...GetOption) string {
	// 这里可以实现缩略图的生成逻辑
	// 目前直接返回公开URL
	return s.GetURL(path, opts...)
}

func (s *S3Storage) Move(src, dst string) error {
	src = strings.TrimPrefix(src, "/")
	dst = strings.TrimPrefix(dst, "/")

	// 首先尝试作为单个文件移动
	_, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(src),
	})

	if err == nil {
		// 源是单个文件，执行单文件移动
		return s.moveSingleFile(src, dst)
	}

	// 源不是单个文件，尝试作为目录移动
	return s.moveDirectory(src, dst)
}

func (s *S3Storage) moveSingleFile(src, dst string) error {
	// 复制对象
	copySource := fmt.Sprintf("%s/%s", s.bucket, src)

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(dst),
	}
	if s.public {
		input.ACL = types.ObjectCannedACLPublicRead
	}

	_, err := s.client.CopyObject(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("copy object failed: %w", err)
	}

	// 删除源对象
	err = s.DeleteObject(src)
	if err != nil {
		return fmt.Errorf("delete source file failed: %w", err)
	}

	return nil
}

func (s *S3Storage) moveDirectory(src, dst string) error {
	// 确保源路径以/结尾（用于列出目录内容）
	if !strings.HasSuffix(src, "/") {
		src = src + "/"
	}

	// 列出源目录下的所有对象
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(src),
	}

	var objectsToDelete []string

	// 分页处理所有对象
	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("list objects failed: %w", err)
		}

		for _, obj := range page.Contents {
			srcKey := *obj.Key
			// 计算目标键：替换前缀
			dstKey := strings.Replace(srcKey, src, dst+"/", 1)

			// 复制对象
			copySource := fmt.Sprintf("%s/%s", s.bucket, srcKey)
			copyInput := &s3.CopyObjectInput{
				Bucket:     aws.String(s.bucket),
				CopySource: aws.String(copySource),
				Key:        aws.String(dstKey),
			}
			if s.public {
				copyInput.ACL = types.ObjectCannedACLPublicRead
			}

			_, err := s.client.CopyObject(context.TODO(), copyInput)
			if err != nil {
				return fmt.Errorf("copy object %s failed: %w", srcKey, err)
			}

			objectsToDelete = append(objectsToDelete, srcKey)
		}
	}

	if len(objectsToDelete) == 0 {
		return fmt.Errorf("no objects found in source directory: %s", src)
	}

	// 批量删除源对象
	for _, key := range objectsToDelete {
		err := s.DeleteObject(key)
		if err != nil {
			// 继续删除其他对象，不因为单个失败而停止
		}
	}

	return nil
}

func (s *S3Storage) GetUploadContext(ctx context.Context, path string) (*UploadContext, error) {
	// 生成上传策略
	expiration := time.Now().Add(15 * time.Minute)
	conditions := []interface{}{
		map[string]string{"bucket": s.bucket},
		[]interface{}{"starts-with", "$key", path},
		map[string]string{"success_action_status": "200"},
		[]interface{}{"content-length-range", 0, 104857600}, // 100MB
		[]interface{}{"starts-with", "$Content-Type", ""},   // 允许所有Content-Type
	}

	// 如果存储是 public，添加 ACL 条件
	// 注意：S3 POST 策略中，表单字段需要使用 $ 前缀
	if s.public {
		conditions = append(conditions, map[string]string{"$x-amz-acl": "public-read"})
	}

	policy := map[string]interface{}{
		"expiration": expiration.UTC().Format(time.RFC3339),
		"conditions": conditions,
	}

	// 将策略转换为JSON
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("marshal policy failed: %w", err)
	}

	// Base64编码策略
	policyBase64 := base64.StdEncoding.EncodeToString(policyJSON)

	// 计算签名
	h := hmac.New(sha1.New, []byte(s.secretKey))
	h.Write([]byte(policyBase64))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	host := strings.Replace(s.endpoint, "https://", "https://"+s.bucket+".", 1)

	tokenData := S3TokenData{
		Policy:     policyBase64,
		AccessID:   s.accessKey,
		Host:       host,
		Signature:  signature,
		Dir:        path,
		Expiration: expiration.UTC().Format(time.RFC3339),
		Public:     s.public,
	}

	return &UploadContext{
		Mode: "s3",
		Data: tokenData,
	}, nil
}
