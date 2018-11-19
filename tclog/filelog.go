package tclog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type FileLog struct {
	sync.Mutex

	maxLines               int
	normalMaxLinesCurLines int
	errMaxLinesCurLines    int

	// Rotate at size
	maxSize              int
	normalMaxSizeCurSize int
	errMaxSizeCurSize    int

	// Rotate daily
	daily         bool
	maxDays       int64
	dailyOpenDate int

	rotate bool

	file     *os.File
	errFile  *os.File
	filepath string
	filename string
}

func IsDir(filename string) (bool, error) {

	if len(filename) <= 0 {
		return false, NewError("invalid dir")
	}

	stat, err := os.Stat(filename)
	if err != nil {
		return false, NewError("invalid path:" + filename)
	}

	if !stat.IsDir() {
		return false, nil
	}

	return true, nil
}

func NewError(format string, a ...interface{}) error {

	err := fmt.Sprintf(format, a...)
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		return errors.New(err)
	}

	function := runtime.FuncForPC(pc).Name()
	msg := fmt.Sprintf("%s func:%s file:%s line:%d",
		err, function, file, line)
	return errors.New(msg)
}

func fileExist(filepath string) bool {
	_, err := os.Stat(filepath)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}

	return false
}

func (fileLog *FileLog) reopenCheck() {
	ticker := time.NewTicker(120 * time.Second)

	for range ticker.C {
		normalLog := fileLog.filepath + "/" + fileLog.filename + ".log"
		if !fileExist(normalLog) {
			file, err := fileLog.openFile(normalLog)
			if err == nil {
				fileLog.Lock()
				if fileLog.file != nil {
					fileLog.file.Close()
				}
				fileLog.file = file
				fileLog.normalMaxSizeCurSize = 0
				fileLog.normalMaxLinesCurLines = 0
				fileLog.Unlock()
			}
		}

		warnLog := normalLog + ".wf"
		if !fileExist(warnLog) {
			errFile, err := fileLog.openFile(warnLog)
			if err == nil {
				fileLog.Lock()
				if fileLog.errFile != nil {
					fileLog.errFile.Close()
				}
				fileLog.errFile = errFile
				fileLog.errMaxSizeCurSize = 0
				fileLog.errMaxLinesCurLines = 0
				fileLog.Unlock()
			}
		}
	}

}

func newFileLog() *FileLog {
	return &FileLog{
		filename: "log_filename",
		maxLines: FileDefMaxLines,
		maxSize:  FileDefMaxSize,
		daily:    true,
		maxDays:  FileDefMaxDays,
		rotate:   true,
	}
}

func (fileLog *FileLog) openFile(filename string) (*os.File, error) {

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)

	if err != nil {
		return nil, NewError("open %s failed, err:%v", filename, err)
	}

	return file, err
}

func (fileLog *FileLog) startLogger() error {

	normalLog := fileLog.filepath + "/" + fileLog.filename + ".log"
	file, err := fileLog.openFile(normalLog)
	if err != nil {
		return err
	}
	if fileLog.file != nil {
		fileLog.file.Close()
	}
	fileLog.file = file

	warnLog := normalLog + ".wf"
	errFile, err := fileLog.openFile(warnLog)
	if err != nil {
		fileLog.file.Close()
		fileLog.file = nil
		return err
	}
	if fileLog.errFile != nil {
		fileLog.errFile.Close()
	}
	fileLog.errFile = errFile

	return fileLog.initFd()
}

func (fileLog *FileLog) initFd() error {
	fileLog.dailyOpenDate = time.Now().Day()

	normalFd := fileLog.file
	fInfo, err := normalFd.Stat()
	if err != nil {
		return NewError("get normalfile stat err:%v", err)
	}
	fileLog.normalMaxSizeCurSize = int(fInfo.Size())
	fileLog.normalMaxLinesCurLines = 0
	if fInfo.Size() > 0 {
		normalLog := fileLog.filepath + "/" + fileLog.filename + ".log"
		count, err := fileLog.lines(normalLog)
		if err != nil {
			return err
		}
		fileLog.normalMaxLinesCurLines = count
	}

	errFd := fileLog.errFile
	fInfo, err = errFd.Stat()
	if err != nil {
		return NewError("get errfile stat err:%v", err)
	}
	fileLog.errMaxSizeCurSize = int(fInfo.Size())
	fileLog.errMaxLinesCurLines = 0
	if fInfo.Size() > 0 {
		errLog := fileLog.filepath + "/" + fileLog.filename + ".log.wf"
		count, err := fileLog.lines(errLog)
		if err != nil {
			return err
		}
		fileLog.errMaxLinesCurLines = count
	}
	return nil
}

func (fileLog *FileLog) lines(filepath string) (int, error) {
	fd, err := os.Open(filepath)
	if err != nil {
		return 0, err
	}
	defer fd.Close()

	buf := make([]byte, 32768) // 32k
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}

		count += bytes.Count(buf[:c], lineSep)

		if err == io.EOF {
			break
		}
	}

	return count, nil
}

func (fileLog *FileLog) needRotate(size int, day int, normal bool) bool {

	curLines := fileLog.normalMaxLinesCurLines
	curSize := fileLog.normalMaxSizeCurSize
	if normal == false {
		curLines = fileLog.errMaxLinesCurLines
		curSize = fileLog.errMaxSizeCurSize
	}

	return (fileLog.maxLines > 0 && curLines >= fileLog.maxLines) ||
		(fileLog.maxSize > 0 && curSize >= fileLog.maxSize) ||
		(fileLog.daily && day != fileLog.dailyOpenDate)
}

func (fileLog *FileLog) WriteMsg(hostname string, when time.Time, msg string, level int) error {
	h, d := formatTimeHeader(when)

	msg = fmt.Sprintf("%s %s %s %s\n", strings.TrimSpace(string(h)), hostname, levelTextArray[level], msg)
	var msgLength int
	msgLength = len(msg)

	var err error
	if fileLog.rotate {
		fileLog.Lock()
		if level >= LevelWarn {
			if fileLog.needRotate(msgLength, d, false) {
				if err = fileLog.doRotate(when, false); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogErrWriter(%q): %s\n", fileLog.filename, err)
				}
			}
		} else {
			if fileLog.needRotate(msgLength, d, true) {
				if err = fileLog.doRotate(when, true); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", fileLog.filename, err)
				}
			}
		}
		fileLog.Unlock()
	}

	fileLog.Lock()
	if level >= LevelWarn {
		_, err = fileLog.errFile.Write([]byte(msg))
		if err == nil {
			fileLog.errMaxLinesCurLines++
			fileLog.errMaxSizeCurSize += msgLength
		}
	} else {
		_, err = fileLog.file.Write([]byte(msg))
		if err == nil {
			fileLog.normalMaxLinesCurLines++
			fileLog.normalMaxSizeCurSize += msgLength
		}
	}
	fileLog.Unlock()
	return err
}

func (fileLog *FileLog) doRotate(logTime time.Time, normal bool) error {
	filepath := ""
	if normal == true {
		filepath = fileLog.filepath + "/" + fileLog.filename + ".log"
	} else {
		filepath = fileLog.filepath + "/" + fileLog.filename + ".log.wf"
	}
	_, err := os.Lstat(filepath)
	if err != nil {
		return err
	}
	// file exists
	// Find the next available number
	num := 1
	fName := ""
	if fileLog.maxLines > 0 || fileLog.maxSize > 0 {
		for ; err == nil && num < MaxFileNum; num++ {
			if normal == true {
				fName = fileLog.filepath + "/" + fileLog.filename + fmt.Sprintf(".%s.%03d.log", logTime.Format("2006-01-02"), num)
			} else {
				fName = fileLog.filepath + "/" + fileLog.filename + fmt.Sprintf(".%s.%03d.log.wf", logTime.Format("2006-01-02"), num)
			}
			_, err = os.Lstat(fName)
		}
	} else {
		if normal == true {
			fName = fmt.Sprintf("%s/%s.%s.log", fileLog.filepath, fileLog.filename, logTime.Format("2006-01-02"))
		} else {
			fName = fmt.Sprintf("%s/%s.%s.log.wf", fileLog.filepath, fileLog.filename, logTime.Format("2006-01-02"))
		}
		_, err = os.Lstat(fName)
	}

	// return error if the last file checked still existed
	if err == nil {
		return NewError("Rotate: Cannot find free log number to rename %s", fileLog.filename)
	}

	// close fileWriter before rename
	if normal == true {
		fileLog.file.Close()
	} else {
		fileLog.errFile.Close()
	}

	// Rename the file to its new found name
	// even if occurs error,we MUST guarantee to  restart new logger
	renameErr := os.Rename(filepath, fName)
	// re-start logger
	startLoggerErr := fileLog.startLogger()

	go fileLog.deleteOldLog()

	if renameErr != nil {
		return NewError("Rotate: %s", renameErr.Error())
	}
	if startLoggerErr != nil {
		return NewError("Rotate StartLogger: %s", startLoggerErr.Error())
	}

	return nil
}

func (fileLog *FileLog) deleteOldLog() {
	dir := filepath.Dir(fileLog.filepath)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Unable to delete old log '%s', error: %v\n", path, r)
			}
		}()

		if !info.IsDir() && info.ModTime().Unix() < (time.Now().Unix()-86400*fileLog.maxDays) {
			if strings.HasPrefix(filepath.Base(path), fileLog.filename) &&
				(strings.HasSuffix(filepath.Base(path), ".log") || strings.HasSuffix(filepath.Base(path), ".log.wf")) {
				os.Remove(path)
			}
		}
		return
	})
}

func (fileLog *FileLog) Destroy() {
	if fileLog.errFile != nil {
		fileLog.errFile.Close()
		fileLog.errFile = nil
	}
	if fileLog.file != nil {
		fileLog.file.Close()
		fileLog.file = nil
	}
}

// Flush flush file logger.
// there are no buffering messages in file logger in memory.
// flush file means sync file from disk.
func (fileLog *FileLog) Flush() {
	if fileLog.errFile != nil {
		fileLog.errFile.Sync()
	}
	if fileLog.file != nil {
		fileLog.file.Sync()
	}
}

const (
	y1  = `0123456789`
	y2  = `0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789`
	y3  = `0000000000111111111122222222223333333333444444444455555555556666666666777777777788888888889999999999`
	y4  = `0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789`
	mo1 = `000000000111`
	mo2 = `123456789012`
	d1  = `0000000001111111111222222222233`
	d2  = `1234567890123456789012345678901`
	h1  = `000000000011111111112222`
	h2  = `012345678901234567890123`
	mi1 = `000000000011111111112222222222333333333344444444445555555555`
	mi2 = `012345678901234567890123456789012345678901234567890123456789`
	s1  = `000000000011111111112222222222333333333344444444445555555555`
	s2  = `012345678901234567890123456789012345678901234567890123456789`
)

func formatTimeHeader(when time.Time) ([]byte, int) {
	y, mo, d := when.Date()
	h, mi, s := when.Clock()
	//len("2006/01/02 15:04:05.067")==24
	millsSec := when.Nanosecond() / 1e6
	var buf [23]byte

	buf[0] = y1[y/1000%10]
	buf[1] = y2[y/100]
	buf[2] = y3[y-y/100*100]
	buf[3] = y4[y-y/100*100]
	buf[4] = '/'
	buf[5] = mo1[mo-1]
	buf[6] = mo2[mo-1]
	buf[7] = '/'
	buf[8] = d1[d-1]
	buf[9] = d2[d-1]
	buf[10] = ' '
	buf[11] = h1[h]
	buf[12] = h2[h]
	buf[13] = ':'
	buf[14] = mi1[mi]
	buf[15] = mi2[mi]
	buf[16] = ':'
	buf[17] = s1[s]
	buf[18] = s2[s]
	buf[19] = '.'
	buf[20] = y1[millsSec/100]
	buf[21] = y1[millsSec/10%10]
	buf[22] = y1[millsSec%10]

	return buf[0:], d
}

//func formatTimeHeader(when time.Time) ([]byte, int) {
//	y, mo, d := when.Date()
//	h, mi, s := when.Clock()
//	//len("2006/01/02 15:04:05 ")==20
//	var buf [20]byte

//	buf[0] = y1[y/1000%10]
//	buf[1] = y2[y/100]
//	buf[2] = y3[y-y/100*100]
//	buf[3] = y4[y-y/100*100]
//	buf[4] = '/'
//	buf[5] = mo1[mo-1]
//	buf[6] = mo2[mo-1]
//	buf[7] = '/'
//	buf[8] = d1[d-1]
//	buf[9] = d2[d-1]
//	buf[10] = ' '
//	buf[11] = h1[h]
//	buf[12] = h2[h]
//	buf[13] = ':'
//	buf[14] = mi1[mi]
//	buf[15] = mi2[mi]
//	buf[16] = ':'
//	buf[17] = s1[s]
//	buf[18] = s2[s]
//	buf[19] = ' '

//	return buf[0:], d
//}
