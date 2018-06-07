
# tclog
    import "github.com/xkeyideal/gokit/tclog"
	

1. 日志的参数设置全部使用函数调用的方式实现，不采用json方式
2. 将Debug, Info, Notice级别的日志定义为常规日志，存入log_filename.log文件中
3. 将Warn, Error, Fatal级别的日志定义为错误日志，存入log_filename.log.wf文件中
4. 日志的设置更加方便灵活
5. 日志的时间采用的是北京时间

tclog日志库支持5种日志级别：
```
	1）Debug
	2）Info
	3）Notice
	4）Warn
	5）Error
	6）Fatal
```

日志级别优先级：

	Debug < Info < Notice < Warn < Error < Fatal
即如果定义日志级别为Debug：则Debug、Info、Notice、Warn、Error、Fatal等级别的日志都会输出；反之，如果定义日志级别为Info，则Debug不会输出，其它级别的日志都会输出。

### func NewTcLog

``` go
func NewTcLog(filepath, filename, level string) (*TcLog, error)
```

初始化TcLog，需要传入日志异步写入目录、文件名和日志级别，日志的写入方式默认就是异步的
	
## Example

```go
	logger, err := NewTcLog("D://log/tclog", "tclog_test", "debug")
	if err != nil {
		fmt.Println(err)
	}
	
	logger.Debug("this is a debug test, id=%d", 3838)
	logger.Notice("this is a notice test, id=%d", 3839)
	logger.Info("this is a info test, id=%d", 3830)
	
	time.Sleep(time.Second)
	logger.Close()
```

## Log Field Example

```go
	logger, err := NewTcLog("D://log/tclog", "tclog_test", "debug")
	if err != nil {
		fmt.Println(err)
	}
	
	field := NewTcLogField(logger)
	field.Set("logid", "%d", 1234567890)
	field.Set("userid", "%s", "23456")

	field.Debug("test debug field")

	cloneField := field.Clone()
	cloneField.Fatal("test fatal clone field")
	
	time.Sleep(time.Second)
	logger.Close()
```
