// main_test.go 验证监听失败会结束等待，不留下无法提供服务的空进程。
package main

import (
	"errors"
	"testing"
)

// TestListenFailure 验证启动错误原样交给关闭流程和退出日志。
func TestListenFailure(t *testing.T) {
	want := errors.New("listen failed")
	done := make(chan error, 1)
	done <- want
	if err := waitExit(done); !errors.Is(err, want) {
		t.Fatal("listener failure lost")
	}
}
