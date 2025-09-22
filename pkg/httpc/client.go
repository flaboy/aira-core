package httpc

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	fasthttpClient *fasthttp.Client
	stdClient      *http.Client
	clientOnce     sync.Once
)

// FastHTTPClient 返回一个单例的 fasthttp.Client 实例
// 这个客户端被全局共享，具有默认的优化配置
func FastHTTPClient() *fasthttp.Client {
	clientOnce.Do(func() {
		fasthttpClient = &fasthttp.Client{
			MaxConnsPerHost:     1000,
			MaxIdleConnDuration: 30 * time.Second,
			ReadTimeout:         10 * time.Second,
			WriteTimeout:        10 * time.Second,
		}

		// 初始化标准 http.Client
		stdClient = &http.Client{
			Timeout: 10 * time.Second,
			// 使用自定义 Transport 来桥接 fasthttp
			// 注意：这里只提供基本功能，完全兼容需要更复杂的适配
			Transport: &fasthttpTransport{},
		}
	})
	return fasthttpClient
}

// Client 返回一个标准库兼容的 http.Client
// 内部使用 fasthttp 提供能力
func Client() http.Client {
	// 确保初始化
	FastHTTPClient()
	return *stdClient
}

// fasthttpTransport 是一个桥接 fasthttp 和 标准库 http.Transport 的适配器
type fasthttpTransport struct{}

// RoundTrip 实现 http.RoundTripper 接口
func (t *fasthttpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 创建 fasthttp 请求和响应
	fasthttpReq := fasthttp.AcquireRequest()
	fasthttpResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(fasthttpReq)
	defer fasthttp.ReleaseResponse(fasthttpResp)

	// 转换请求
	fasthttpReq.SetRequestURI(req.URL.String())
	fasthttpReq.Header.SetMethod(req.Method)

	// 复制 headers
	for k, vv := range req.Header {
		for _, v := range vv {
			fasthttpReq.Header.Add(k, v)
		}
	}

	// 处理请求体
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		fasthttpReq.SetBody(body)
	}

	// 执行请求
	if err := FastHTTPClient().Do(fasthttpReq, fasthttpResp); err != nil {
		return nil, err
	}

	// 创建标准库的响应
	resp := &http.Response{
		StatusCode: fasthttpResp.StatusCode(),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(fasthttpResp.Body())),
	}

	// 复制响应头
	fasthttpResp.Header.VisitAll(func(key, value []byte) {
		resp.Header.Add(string(key), string(value))
	})

	return resp, nil
}

// DoGet 执行GET请求并返回响应体
func DoGet(url string) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodGet)

	if err := FastHTTPClient().Do(req, resp); err != nil {
		return nil, err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("HTTP错误状态码: %d", resp.StatusCode())
	}

	return resp.Body(), nil
}

// DoPost 执行POST请求并返回响应体
func DoPost(url string, contentType string, body []byte) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType(contentType)
	req.SetBody(body)

	if err := FastHTTPClient().Do(req, resp); err != nil {
		return nil, err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("HTTP错误状态码: %d", resp.StatusCode())
	}

	return resp.Body(), nil
}

// DoPostForm 执行POST表单请求并返回响应体
func DoPostForm(url string, args *fasthttp.Args) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/x-www-form-urlencoded")

	if args != nil {
		args.WriteTo(req.BodyWriter())
	}

	if err := FastHTTPClient().Do(req, resp); err != nil {
		return nil, err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("HTTP错误状态码: %d", resp.StatusCode())
	}

	return resp.Body(), nil
}
