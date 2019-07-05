package main

import (
	"github.com/gorilla/websocket"
	"go-push/connection"
	"net/http"
	"time"
)

var (
	// 转换器 完成http->websocket转换的握手
	upgrader = websocket.Upgrader{
		// CheckOrigin 回调函数
		// 握手过程中涉及到跨域问题是允许它跨域的
		// websocket的服务端一般是独立部署在一个域名下
		// 当从浏览器的直播间页面发起到websocket服务连接的时候
		// 本身是跨域的 直播.com --> 推送websocket.com 跨域
		// 告诉upgrader允许跨域 return true
		// 当 r *http.Request 请求过来请求跨域的websocket 允许跨域
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// 变量定义放在头部
	var (
		wsConn *websocket.Conn
		// 自封装Connection
		conn *connection.Connection
		err  error
		data []byte
	)
	// 完成握手switching应答
	// 返回对象 请求对象 response需返回header upgrade
	// 如果不需要返回其他header字段 传前2个参数即可
	// 底层就把 Upgrade:websocket 封装上了
	// 成功则返回websocket长连接
	if wsConn, err = upgrader.Upgrade(w, r, nil); err != nil {
		// 连接错误底层会默认终端websocket连接
		return
	}
	if conn, err = connection.InitConnection(wsConn); err != nil {
		goto ERR
	}

	// 演示线程安全
	// 启动一个协程 每秒给客户端发1个心跳消息
	go func() {
		var (
			err error
		)
		for {
			if err = conn.WriteMessage([]byte("heart beat")); err != nil {
				continue
			}
			time.Sleep(1 * time.Second)
		}
	}()

	for {
		if data, err = conn.ReadMessage(); err != nil {
			goto ERR
		}
		if err = conn.WriteMessage(data); err != nil {
			goto ERR
		}
	}

ERR:
	conn.Close()
}

func main() {
	// http://localhost:7777/ws

	// 1.路由接口地址
	// 2.回调函数-当有请求来的时候这个函数会被回调
	// http 配置路由 /ws 接口
	// 当客户端调用 /ws 接口 wsHandler函数就会被调用
	http.HandleFunc("/ws", wsHandler)
	// 启动服务端
	// 绑定监听端口对外提供http服务
	// 前面配置好了 /ws 路由 这里的第2个参数就可以忽略
	http.ListenAndServe("0.0.0.0:7777", nil)
}
