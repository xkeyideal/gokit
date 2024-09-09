package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var server *http.Server
var router *gin.Engine

type TestData struct {
	Name string `json:"name"`
}

func main() {
	router = gin.Default()
	router.POST("/test", func(c *gin.Context) {
		bytes, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			panic("sdfsdf" + err.Error())
			SetStrResp(http.StatusBadRequest, HTTP_BODY_ERR, err.Error(), "", c)
			return
		}

		log.Println("121221", string(bytes))

		SetStrResp(400, 0, "OK", "123", c)
	})

	server = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:12745"),
		Handler: router,
	}

	if err := server.ListenAndServe(); err != nil {
		fmt.Println("listenAndServe error ", err.Error())
		os.Exit(-1)
	}
}

const (
	JSON_UNMARSHAL = 1000
	HTTP_BODY_ERR  = 1001

	SERVER_ERR = 2000
	RPC_ERR    = 2001
)

var errCodeMsg = map[int]string{
	JSON_UNMARSHAL: "[JSON 反序列化异常]: ",
	HTTP_BODY_ERR:  "[HTTP BODY读取异常]: ",

	SERVER_ERR: "[选择后端节点异常]: ",
	RPC_ERR:    "[远端服务器调用出错]: ",
}

func SetStrResp(httpCode, code int, msg string, result interface{}, c *gin.Context) {
	m := msg

	if v, ok := errCodeMsg[code]; ok {
		m = fmt.Sprintf("%s%s", v, msg)
	}

	if code == 0 {
		c.JSON(httpCode, gin.H{
			"code":   code,
			"msg":    m,
			"result": result,
		})
	} else {
		c.JSON(httpCode, gin.H{
			"code": code,
			"msg":  m,
		})
	}
}
