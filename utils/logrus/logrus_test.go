/**
 * @author: Jay
 * @date: 2024/11/19
 * @file: logrus_test.go
 * @description:
 */

package logrus

import "testing"

// TestLogFileRotation 测试日志文件轮转
func TestLogFileRotation(t *testing.T) {
	// 实例化日志
	Initialize()
	Info("Testing log file rotation")
	Warn("This should trigger rotation if file size exceeds limit")
	Error("This is an error message for rotation testing")
}
