package tools

import "time"

func CSTNow() time.Time {
	cstLocal, _ := time.LoadLocation("Asia/Shanghai")
	return time.Now().In(cstLocal)
}

func CSTLocal() *time.Location {
	cstLocal, _ := time.LoadLocation("Asia/Shanghai")
	return cstLocal
}

func CSTNowUnix() int64 {
	cstLocal, _ := time.LoadLocation("Asia/Shanghai")
	return time.Now().In(cstLocal).Unix()
}

func Stamp2Time(sec, nsec int64) time.Time {
	cstLocal, _ := time.LoadLocation("Asia/Shanghai")
	return time.Unix(sec, nsec).In(cstLocal)
}

func Exact2Hour(n time.Time) time.Time {
	local, _ := time.LoadLocation("Asia/Shanghai")
	return time.Date(n.Year(), n.Month(), n.Day(), n.Hour(), 0, 0, 0, local)
}

func Exact2Day(n time.Time) time.Time {
	local, _ := time.LoadLocation("Asia/Shanghai")
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, local)
}

func Date2Time(n time.Time, hour int) int64 {
	local, _ := time.LoadLocation("Asia/Shanghai")
	t := time.Date(n.Year(), n.Month(), n.Day(), hour, 0, 0, 0, local)
	return t.Unix()
}

func CSTNowDate() time.Time {
	cstLocal, _ := time.LoadLocation("Asia/Shanghai")
	timeStr := time.Now().In(cstLocal).Format("2006-01-02")
	time, _ := time.Parse("2006-01-02", timeStr)
	return time
}
