package service

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"im-project/global"
	"im-project/models"
	"log"
	"sync"
	"time"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second
	writeWait  = 10 * time.Second
)

var (
	clients                   = make(map[uint]*Client)
	clientsMu                 sync.RWMutex
	serviceCtx, serviceCancel = context.WithCancel(context.Background())
)

type Client struct {
	UID       uint
	Conn      *websocket.Conn
	Send      chan models.Message
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func StartRedisListener() {
	if global.RDB == nil {
		log.Println("[Redis监听失败] Redis客户端不存在")
		return
	}

	pubsub := global.RDB.Subscribe(serviceCtx, "im_global_channel")
	defer pubsub.Close()

	if _, err := pubsub.Receive(serviceCtx); err != nil {
		log.Printf("[Redis监听失败] 订阅频道失败: %v", err)
		return
	}

	ch := pubsub.Channel()
	log.Println("Redis跨节点广播已就绪...")
	for {
		select {
		case msg := <-ch:
			if msg == nil {
				return
			}
			var imMsg models.Message
			if err := json.Unmarshal([]byte(msg.Payload), &imMsg); err != nil {
				log.Printf("[Redis监听失败] JSON解析失败: %v", err)
				continue
			}
			SendToLocal(imMsg)
		case <-serviceCtx.Done():
			return
		}
	}
}
func SendToLocal(msg models.Message) bool {
	clientsMu.RLock()
	client, isLocal := clients[msg.To]
	clientsMu.RUnlock()
	if isLocal {
		select {
		case client.Send <- msg:
			return true
		case <-client.ctx.Done():
			return false
		}
	}
	return false
}
func BroadcastMessage(msg models.Message) {
	if global.RDB == nil {
		log.Println("[Redis发送失败] Redis客户端不存在")
		return
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Redis发送失败] JSON序列化失败: %v", err)
		return
	}

	if err := global.RDB.Publish(serviceCtx, "im_global_channel", bytes).Err(); err != nil {
		log.Printf("[Redis发送失败] Publish失败: %v", err)
	}
}
func (c *Client) ReadLoop() {
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	defer func() {
		clientsMu.Lock()
		if current, ok := clients[c.UID]; ok && current == c {
			delete(clients, c.UID)
		}
		clientsMu.Unlock()
		c.Close()
	}()
	for {
		var msg models.Message
		if err := c.Conn.ReadJSON(&msg); err != nil {
			break
		}
		msg.From = c.UID
		if global.DB == nil {
			log.Printf("[消息保存失败] MySQL连接不存在，UID: %d", c.UID)
			continue
		}
		if err := global.DB.Create(&msg).Error; err != nil {
			log.Printf("[消息保存失败] UID: %d, error: %v", c.UID, err)
			continue
		}
		if !SendToLocal(msg) {
			BroadcastMessage(msg)
		}
	}
}
func (c *Client) WriteLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteJSON(msg); err != nil {
				c.Close()
				return
			}
		case <-ticker.C:
			deadline := time.Now().Add(writeWait)
			if err := c.Conn.WriteControl(websocket.PingMessage, []byte("ping"), deadline); err != nil {
				c.Close()
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
		c.Conn.Close()
	})
}

func RegisterClient(uid uint, conn *websocket.Conn) {
	clientCtx, cancel := context.WithCancel(serviceCtx)
	client := &Client{
		UID:    uid,
		Conn:   conn,
		Send:   make(chan models.Message, 100),
		ctx:    clientCtx,
		cancel: cancel,
	}

	clientsMu.Lock()
	oldClient := clients[uid]
	clients[uid] = client
	clientsMu.Unlock()

	if oldClient != nil {
		oldClient.Close()
	}

	go client.ReadLoop()
	go client.WriteLoop()
}

func Shutdown() {
	serviceCancel()

	clientsMu.Lock()
	activeClients := make([]*Client, 0, len(clients))
	for uid, client := range clients {
		activeClients = append(activeClients, client)
		delete(clients, uid)
	}
	clientsMu.Unlock()

	for _, client := range activeClients {
		client.Close()
	}
	if global.RDB != nil {
		_ = global.RDB.Close()
	}
}
