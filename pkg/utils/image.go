package utils

import (
	"fmt"
	"io"
	"net/http"
)

// DownloadImage 下载网络图片并返回字节切片
func DownloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %v", err)
	}

	return data, nil
}
