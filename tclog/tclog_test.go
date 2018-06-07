package tclog

import (
	"testing"
	"time"
)

func TestTcLogger(t *testing.T) {
	logger, err := NewTcLog("D://log/tclog", "tclog_test", "debug")
	if err != nil {
		t.Fatalf("logger open failed, %v", err)
	}
	logger.EnableFuncCallDepth(true)

	logger.Debug("this is a debug test, id=%d", 3838)
	logger.Notice("this is a notice test, id=%d", 3839)
	logger.Info("this is a info test, id=%d", 3830)

	field := NewTcLogField(logger)
	field.Set("logid", "%d", 1234567890)
	field.Set("userid", "%s", "23456")

	field.Debug("test debug field")

	cloneField := field.Clone()
	cloneField.Fatal("test fatal clone field")

	time.Sleep(time.Second)
	logger.Close()
}
