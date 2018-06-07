package tclog

import (
	"fmt"
)

type TcLogField struct {
	fields           map[string]string
	logger           *TcLog
	logFuncCallDepth int
	ignoreKey        bool
}

func NewTcLogField(logger *TcLog) *TcLogField {
	return &TcLogField{
		fields:           make(map[string]string),
		logger:           logger,
		logFuncCallDepth: TcLogDefCallDepth + 1,
		ignoreKey:        true,
	}
}

func (logField *TcLogField) Clone() *TcLogField {
	cloneField := &TcLogField{
		fields:           make(map[string]string),
		logger:           logField.logger,
		logFuncCallDepth: logField.logFuncCallDepth,
		ignoreKey:        logField.ignoreKey,
	}

	for key, value := range logField.fields {
		cloneField.fields[key] = value
	}

	return cloneField
}

func (logField *TcLogField) SetIgnoreKey(enable bool) {
	logField.ignoreKey = enable
}

func (logField *TcLogField) Set(key string, format string, values ...interface{}) {
	logField.fields[key] = fmt.Sprintf(format, values...)
}

func (logField *TcLogField) Del(key string) {
	delete(logField.fields, key)
}

func (logField *TcLogField) ClearFields() {
	for key, _ := range logField.fields {
		delete(logField.fields, key)
	}
}

func (logField *TcLogField) IncrDepth(depth int) {
	logField.logFuncCallDepth += depth
}

func (logField *TcLogField) Debug(format string, v ...interface{}) {
	if logField.logger != nil {
		msg := ""
		if logField.logger.enableFuncCallDepth {
			logField.logger.logFuncCallDepth = logField.logFuncCallDepth
		}
		for key, value := range logField.fields {
			if logField.ignoreKey {
				msg = fmt.Sprintf("%s [%s]", msg, value)
			} else {
				msg = fmt.Sprintf("%s [%s: %s]", msg, key, value)
			}
		}
		msg = fmt.Sprintf("%s %s", msg, format)
		logField.logger.Debug(msg, v...)
	}
}

func (logField *TcLogField) Info(format string, v ...interface{}) {
	if logField.logger != nil {
		msg := ""
		if logField.logger.enableFuncCallDepth {
			logField.logger.logFuncCallDepth = logField.logFuncCallDepth
		}
		for key, value := range logField.fields {
			if logField.ignoreKey {
				msg = fmt.Sprintf("%s [%s]", msg, value)
			} else {
				msg = fmt.Sprintf("%s [%s: %s]", msg, key, value)
			}
		}
		msg = fmt.Sprintf("%s %s", msg, format)
		logField.logger.Info(msg, v...)
	}
}

func (logField *TcLogField) Notice(format string, v ...interface{}) {
	if logField.logger != nil {
		msg := ""
		if logField.logger.enableFuncCallDepth {
			logField.logger.logFuncCallDepth = logField.logFuncCallDepth
		}
		for key, value := range logField.fields {
			if logField.ignoreKey {
				msg = fmt.Sprintf("%s [%s]", msg, value)
			} else {
				msg = fmt.Sprintf("%s [%s: %s]", msg, key, value)
			}
		}
		msg = fmt.Sprintf("%s %s", msg, format)
		logField.logger.Notice(msg, v...)
	}
}

func (logField *TcLogField) Warn(format string, v ...interface{}) {
	if logField.logger != nil {
		msg := ""
		if logField.logger.enableFuncCallDepth {
			logField.logger.logFuncCallDepth = logField.logFuncCallDepth
		}
		for key, value := range logField.fields {
			if logField.ignoreKey {
				msg = fmt.Sprintf("%s [%s]", msg, value)
			} else {
				msg = fmt.Sprintf("%s [%s: %s]", msg, key, value)
			}
		}
		msg = fmt.Sprintf("%s %s", msg, format)
		logField.logger.Warn(msg, v...)
	}
}

func (logField *TcLogField) Error(format string, v ...interface{}) {
	if logField.logger != nil {
		msg := ""
		if logField.logger.enableFuncCallDepth {
			logField.logger.logFuncCallDepth = logField.logFuncCallDepth
		}
		for key, value := range logField.fields {
			if logField.ignoreKey {
				msg = fmt.Sprintf("%s [%s]", msg, value)
			} else {
				msg = fmt.Sprintf("%s [%s: %s]", msg, key, value)
			}
		}
		msg = fmt.Sprintf("%s %s", msg, format)
		logField.logger.Error(msg, v...)
	}
}

func (logField *TcLogField) Fatal(format string, v ...interface{}) {
	if logField.logger != nil {
		msg := ""
		if logField.logger.enableFuncCallDepth {
			logField.logger.logFuncCallDepth = logField.logFuncCallDepth
		}
		for key, value := range logField.fields {
			if logField.ignoreKey {
				msg = fmt.Sprintf("%s [%s]", msg, value)
			} else {
				msg = fmt.Sprintf("%s [%s: %s]", msg, key, value)
			}
		}
		msg = fmt.Sprintf("%s %s", msg, format)
		logField.logger.Fatal(msg, v...)
	}
}
