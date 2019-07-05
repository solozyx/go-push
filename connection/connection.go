package connection

import (
	"errors"
	"github.com/gorilla/websocket"
	"sync"
)

type Connection struct {
	// websocket 长连接
	wsConn *websocket.Conn
	// 服务端读取到的消息
	inChan chan []byte
	// 服务端发送给客户端的消息
	outChan chan []byte
	//
	closeChan chan byte
	//
	isClosed bool
	//保证 Connection.Close 线程安全 加锁
	mutex sync.Mutex
}

func InitConnection(wsConn *websocket.Conn) (conn *Connection, err error) {
	conn = &Connection{
		wsConn,
		// 消息容量 1000 满了就无法放入新的消息
		make(chan []byte, 1000),
		make(chan []byte, 1000),
		make(chan byte, 1),
		false,
		sync.Mutex{},
	}
	// 开启读协程
	go conn.readLoop()
	// 开启写协程
	go conn.writeLoop()
	return
}

// 封装线程安全的 ReadMessage 可以被并发调用线程安全
func (conn *Connection) ReadMessage() (data []byte, err error) {
	// Connection.ReadMessage 用户在调用的时候 底层网络连接出错
	// 用户阻塞在 data = <- conn.inChan 取数据取不出来 阻塞
	// data = <- conn.inChan
	select {
	// 要么阻塞等待消息到来
	case data = <-conn.inChan:
	// 要么closeChan可读了
	case <-conn.closeChan:
		// closeChan能够进入分支说明它被关闭了
		err = errors.New("connection is closed ")
	}
	return
}

// API

// 封装线程安全的 WriteMessage 可以被并发调用线程安全
func (conn *Connection) WriteMessage(data []byte) (err error) {
	select {
	// 发送队列没满 就写入channel了
	// 如果满了就 阻塞在 case conn.outChan <- data:
	case conn.outChan <- data:
	// 如果网络连接出错了
	case <-conn.closeChan:
		err = errors.New("connection is closed ")
	}
	return
}

// websocket框架的Close方法是线程安全的 可以被并发多线程调用 可以被调用多次 成为 可重入调用
func (conn *Connection) Close() {
	// 底层的websocket关闭
	conn.wsConn.Close()
	// 关闭channel
	// channnel只能关闭1次
	// 加锁 确保关闭channel的操作线程安全
	// isClosed保证这段代码可重复执行
	conn.mutex.Lock()
	if !conn.isClosed {
		// 关闭channel就通知readLoop writeLoop 让它们退出
		// 整个资源也就被释放干净了
		close(conn.closeChan)
		conn.isClosed = true
	}
	// 释放锁
	conn.mutex.Unlock()
}

// Implementation

// 轮询websocket长连接读取到的消息
func (conn *Connection) readLoop() {
	var (
		data []byte
		err  error
	)
	for {
		// *websocket.Conn 长连接 做数据的收发
		// 无参数
		// 返回 (消息类型 数据 错误)
		// websocket支持传输的类型 text binary
		if _, data, err = conn.wsConn.ReadMessage(); err != nil {
			// 跳到 ERR 标签
			goto ERR
		}
		// 读取客户端发送消息成功写入 inChan
		select {
		case conn.inChan <- data:
		case <-conn.closeChan:
			// closeChan关闭的时候
			goto ERR
		}
		// 如果没有错误 程序就在for一直轮询了
	}
ERR:
	conn.Close()
}

func (conn *Connection) writeLoop() {
	var (
		data []byte
		err  error
	)
	for {
		select {
		// 从outChan取要发送的消息,如果没有要发送的消息会阻塞在这里
		case data = <-conn.outChan:
		// readLoop发现网络错误 closeChan一旦关闭
		case <-conn.closeChan:
			goto ERR
		}

		// 发送消息到客户端 测试 返回客户端发来的消息
		if err = conn.wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
			goto ERR
		}
		// 如果没有错误 程序就在for一直轮询了
	}
ERR:
	conn.Close()
}
