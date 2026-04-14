package utils

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Post 发送POST请求
// url: 请求地址
// params: 请求参数，会被序列化为JSON格式
// 返回响应体字节数组和错误信息
// 超时时间为20秒
func Post(url string, params any) ([]byte, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelFunc()
	marshal, _ := json.Marshal(params)
	s := string(marshal)
	reqBody := strings.NewReader(s)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Add("Content-Type", "application/json")
	httpRsp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpRsp.Body.Close()
	rspBody, err := io.ReadAll(httpRsp.Body)
	if err != nil {
		return nil, err
	}
	return rspBody, nil
}

// 你的程序 -> 代理 -> 目标服务 -> 代理 -> 你的程序
// GetWithHeader 发送带自定义请求头的GET请求
// path: 请求地址
// mh: 自定义请求头键值对，可为nil
// proxy: 代理地址，为空字符串时不使用代理（代理就是让你的 HTTP 请求"绕个路"，通过中间服务器转发）
// 返回响应体字节数组和错误信息
// 超时时间为20秒，Content-Type为application/json
func GetWithHeader(path string, mh map[string]string, proxy string) ([]byte, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelFunc()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range mh {
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Add("Content-Type", "application/json")
	client := http.DefaultClient
	if proxy != "" {
		proxyAddress, _ := url.Parse(proxy)
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyAddress),
			},
		}
	}
	httpRsp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpRsp.Body.Close()
	rspBody, err := io.ReadAll(httpRsp.Body)
	if err != nil {
		return nil, err
	}
	return rspBody, nil
}

// PostWithHeader 发送带自定义请求头的POST请求
// path: 请求地址
// params: 请求参数，会被序列化为JSON格式
// mh: 自定义请求头键值对，可为nil
// proxy: 代理地址，为空字符串时不使用代理（代理就是让你的 HTTP 请求"绕个路"，通过中间服务器转发）
// 返回响应体字节数组和错误信息
// 超时时间为20秒
func PostWithHeader(path string, params any, mh map[string]string, proxy string) ([]byte, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelFunc()
	marshal, _ := json.Marshal(params)
	s := string(marshal)
	reqBody := strings.NewReader(s)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}
	for k, v := range mh {
		httpReq.Header.Set(k, v)
	}
	client := http.DefaultClient
	if proxy != "" {
		proxyAddress, _ := url.Parse(proxy)
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyAddress),
			},
		}
	}
	httpRsp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpRsp.Body.Close()
	rspBody, err := io.ReadAll(httpRsp.Body)
	if err != nil {
		return nil, err
	}
	return rspBody, nil
}
