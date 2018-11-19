/*
日志级别优先级：

 Debug < Info < Notice < Warn < Error < Fatal
*/

package tclog

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	LevelDebug = iota
	LevelInfo
	LevelNotice
	LevelWarn
	LevelError
	LevelFatal
)

const (
	TcLogDefCallDepth = 3
	FileDefMaxLines   = 1000000
	FileDefMaxSize    = 1 << 28 //256 MB
	FileDefMaxDays    = 7
	MaxFileNum        = 1000
)

var (
	levelTextArray = []string{
		LevelDebug:  "DEBUG",
		LevelInfo:   "INFO",
		LevelNotice: "NOTICE",
		LevelWarn:   "WARN",
		LevelError:  "ERROR",
		LevelFatal:  "FATAL",
	}
)

type TcLog struct {
	level               int
	enableFuncCallDepth bool
	logFuncCallDepth    int
	msgChan             chan *logMsg
	signalChan          chan string
	hostname            string
	fileLog             *FileLog
}

type logMsg struct {
	level int
	msg   string
	when  time.Time
}

var logMsgPool *sync.Pool
var cstLocal *time.Location

func NewTcLog(filepath, filename, level string) (*TcLog, error) {

	log := &TcLog{
		level:               LevelDebug,
		enableFuncCallDepth: false,
		logFuncCallDepth:    TcLogDefCallDepth,
		msgChan:             make(chan *logMsg, 100),
		signalChan:          make(chan string, 1),
		fileLog:             newFileLog(),
	}

	logMsgPool = &sync.Pool{
		New: func() interface{} {
			return &logMsg{}
		},
	}

	cstLocal, _ = time.LoadLocation("Asia/Shanghai")

	isDir, err := IsDir(filepath)
	if err != nil || !isDir {
		err = os.MkdirAll(filepath, 0755)
		if err != nil {
			return nil, NewError("Mkdir failed, err:%v", err)
		}
	}

	log.level = log.levelFromStr(level)
	log.fileLog.filename = filename
	log.fileLog.filepath = filepath
	hostname, _ := os.Hostname()
	log.hostname = hostname

	err = log.fileLog.startLogger()
	if err != nil {
		return nil, err
	}

	go log.fileLog.reopenCheck()

	go log.startLogger()

	return log, nil
}

func (log *TcLog) levelFromStr(level string) int {
	resultLevel := LevelDebug
	lower := strings.ToLower(level)
	switch lower {
	case "debug":
		resultLevel = LevelDebug
	case "info":
		resultLevel = LevelInfo
	case "notice":
		resultLevel = LevelNotice
	case "warn":
		resultLevel = LevelWarn
	case "fatal":
		resultLevel = LevelFatal
	default:
		resultLevel = LevelInfo
	}
	return resultLevel
}

func (log *TcLog) SetLevel(level string) {
	log.level = log.levelFromStr(level)
}

func (log *TcLog) GetLevel() string {
	return levelTextArray[log.level]
}

func (log *TcLog) SetFuncCallDepth(depth int) {
	log.logFuncCallDepth = depth
}

func (log *TcLog) EnableFuncCallDepth(flag bool) {
	log.enableFuncCallDepth = flag
}

func (log *TcLog) SetMaxDays(day int64) {
	log.fileLog.maxDays = day
}

func (log *TcLog) GetMaxDays() int64 {
	return log.fileLog.maxDays
}

func (log *TcLog) SetMaxLines(line int) {
	log.fileLog.maxLines = line
}

func (log *TcLog) GetMaxLines() int {
	return log.fileLog.maxLines
}

func (log *TcLog) SetMaxSize(size int) {
	log.fileLog.maxSize = size
}

func (log *TcLog) GetMaxSize() int {
	return log.fileLog.maxSize
}

func (log *TcLog) EnableRotate(flag bool) {
	log.fileLog.rotate = flag
}

func (log *TcLog) EnableDaily(flag bool) {
	log.fileLog.daily = flag
}

func (log *TcLog) GetHost() string {
	return log.hostname
}

func (log *TcLog) startLogger() {
	gameOver := false
	for {
		select {
		case lm := <-log.msgChan:
			log.writeToFile(lm.when, lm.msg, lm.level)
			logMsgPool.Put(lm)
		case sg := <-log.signalChan:
			// Now should only send "flush" or "close" to log.signalChan
			log.flush()
			if sg == "close" {
				log.fileLog.Destroy()
				gameOver = true
			}
		}
		if gameOver {
			break
		}
	}
}

func (log *TcLog) Flush() {
	log.signalChan <- "flush"
}

// Close close logger, flush all chan data and destroy all adapters in BeeLogger.
func (log *TcLog) Close() {
	log.signalChan <- "close"
	close(log.msgChan)
	close(log.signalChan)
}

func (log *TcLog) flush() {
	for {
		if len(log.msgChan) > 0 {
			lm := <-log.msgChan
			log.writeToFile(lm.when, lm.msg, lm.level)
			logMsgPool.Put(lm)
			continue
		}
		break
	}
	log.fileLog.Flush()
}

func (log *TcLog) getRuntimeInfo(skip int) map[string]interface{} {

	function := ""
	pc, file, line, ok := runtime.Caller(skip)
	if ok {
		function = runtime.FuncForPC(pc).Name()
	}

	runInfo := make(map[string]interface{})
	runInfo["xx_func"] = function
	runInfo["xx_file"] = file
	runInfo["xx_line"] = line

	log.logFuncCallDepth = TcLogDefCallDepth

	return runInfo
}

func (log *TcLog) writeMsg(logLevel int, msg string) {
	when := time.Now().In(cstLocal)
	if log.enableFuncCallDepth {
		runInfo := log.getRuntimeInfo(log.logFuncCallDepth)
		msg = fmt.Sprintf("func:%s file:%s line:%d %s", runInfo["xx_func"], runInfo["xx_file"], runInfo["xx_line"], msg)
	}

	lm := logMsgPool.Get().(*logMsg)
	lm.level = logLevel
	lm.msg = msg
	lm.when = when

	log.msgChan <- lm
}

func (log *TcLog) writeToFile(when time.Time, msg string, level int) {
	log.fileLog.WriteMsg(log.hostname, when, msg, level)
}

// DEBUG Log DEBUG level message.
func (log *TcLog) Debug(format string, v ...interface{}) {
	if LevelDebug < log.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	log.writeMsg(LevelDebug, msg)
}

// INFO Log INFO level message.
func (log *TcLog) Info(format string, v ...interface{}) {
	if LevelInfo < log.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	log.writeMsg(LevelInfo, msg)
}

// Notice Log Notice level message.
func (log *TcLog) Notice(format string, v ...interface{}) {
	if LevelNotice < log.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	log.writeMsg(LevelNotice, msg)
}

// Warn Log Warn level message.
func (log *TcLog) Warn(format string, v ...interface{}) {
	if LevelWarn < log.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	log.writeMsg(LevelWarn, msg)
}

// Error Log Error level message.
func (log *TcLog) Error(format string, v ...interface{}) {
	if LevelError < log.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	log.writeMsg(LevelError, msg)
}

// Fatal Log Fatal level message.
func (log *TcLog) Fatal(format string, v ...interface{}) {
	if LevelFatal < log.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	log.writeMsg(LevelFatal, msg)
}

func (log *TcLog) Output(calldepth int, s string) error {
	log.SetFuncCallDepth(calldepth)
	log.writeMsg(LevelWarn, s)
	return nil
}
