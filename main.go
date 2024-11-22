package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	defaultExpiration     = 30 * 24 * time.Hour // 默认30天过期时间
	maxMessageLength      = 650                 // 消息最大长度
	maxMessagesPerChannel = 1000                // 每个频道最大消息数
	defaultPageSize       = 100                 // 默认分页大小
)

var (
	rdb      *redis.Client
	writeMux sync.Mutex
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func init() {
	// 初始化 Redis 连接
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		// 配置连接池
		PoolSize:     10, // 最大连接数
		MinIdleConns: 3,  // 最小空闲连接数
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
}

type Message struct {
	UUID           string `json:"uuid"`
	Content        string `json:"content"`
	UserID         int    `json:"userId"`
	ConversationID string `json:"convId"`
	Time           string `json:"time"`
	Type           string `json:"type"`
}

// Conversation 定义对话结构
type Conversation struct {
	ID      string `json:"id"`
	Members []int  `json:"members"`
	Name    string `json:"name"`
}

// WebSocket处理函数
func handleWebSocket(c *gin.Context) {
	userId := c.Query("userId")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}

	// 升级为 WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade failed:", err)
		return
	}
	defer ws.Close()

	// 订阅 Redis 消息
	pubsub := rdb.Subscribe(c, fmt.Sprintf("user:%s:msgs", userId))
	defer pubsub.Close()

	// 启动 Goroutine 处理 Redis 消息
	go handleRedisMessages(ws, pubsub.Channel())

	for {
		// 读取 WebSocket 消息
		_, messageBytes, err := ws.ReadMessage()
		if err != nil {
			fmt.Println("WebSocket read error:", err)
			break
		}
		// 处理 WebSocket 消息
		handleWebSocketMessage(c, messageBytes)
	}
}

// 处理 Redis 消息并发送到 WebSocket
func handleRedisMessages(ws *websocket.Conn, ch <-chan *redis.Message) {
	for msg := range ch {
		writeMux.Lock()
		err := ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		writeMux.Unlock()
		if err != nil {
			fmt.Println("WebSocket write error:", err)
			break
		}
	}
}

// 处理从 WebSocket 接收到的消息
func handleWebSocketMessage(c *gin.Context, messageBytes []byte) {
	var msg Message
	if err := json.Unmarshal(messageBytes, &msg); err != nil || len(msg.Content) > maxMessageLength {
		fmt.Println("Invalid message received")
		return
	}

	// 设置消息时间
	msg.Time = time.Now().Format(time.RFC3339)
	jsonMsg, _ := json.Marshal(msg)

	// 将消息存储到 Redis
	key := fmt.Sprintf("conv:%s", msg.ConversationID)
	rdb.ZAdd(c, key, &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: jsonMsg,
	})
	rdb.Expire(c, key, defaultExpiration)

	// 发布消息到所有订阅用户
	for _, user := range getUsersByConversation(c, msg.ConversationID) {
		rdb.Publish(c, fmt.Sprintf("user:%s:msgs", user), jsonMsg)
	}
}

// 获取对话中的用户列表
func getUsersByConversation(c *gin.Context, conversationID string) []string {
	users, err := rdb.SMembers(c, fmt.Sprintf("conv:%s:users", conversationID)).Result()
	if err != nil {
		fmt.Println("Error fetching users:", err)
		return nil
	}
	return users
}

// 获取聊天历史记录
func getChatHistory(c *gin.Context) {
	conversationID := c.Query("convId")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversationId is required"})
		return
	}

	pageSize := defaultPageSize
	page := 1

	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeParam := c.Query("pageSize"); pageSizeParam != "" {
		if ps, err := strconv.Atoi(pageSizeParam); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	key := fmt.Sprintf("conv:%s", conversationID)
	start := int64((page - 1) * pageSize)
	end := start + int64(pageSize) - 1

	result, err := rdb.ZRange(c, key, start, end).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var msgs []Message
	for _, item := range result {
		var message Message
		if err := json.Unmarshal([]byte(item), &message); err == nil {
			msgs = append(msgs, message)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"msgs":        msgs,
		"page":        page,
		"pageSize":    pageSize,
		"hasNextPage": len(result) == pageSize,
	})
}

func createConversation(c *gin.Context) {
	var data struct {
		Name    string `json:"name"`    // 对话名称
		Members []int  `json:"members"` // 支持多个成员
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// 验证 `Members` 和 `Name`
	if len(data.Members) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least two members are required"})
		return
	}
	if data.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Conversation name is required"})
		return
	}

	// 生成唯一的对话 ID
	conversationID := uuid.New().String()

	// 存储对话的用户列表
	memberKeys := make([]interface{}, len(data.Members))
	for i, member := range data.Members {
		memberKeys[i] = member
		userKey := fmt.Sprintf("user:%d:convs", member)
		rdb.HSet(c, userKey, conversationID, 1)
	}
	rdb.SAdd(c, fmt.Sprintf("conv:%s:users", conversationID), memberKeys...)
	// 存储对话的名称
	rdb.HSet(c, fmt.Sprintf("conv:%s:meta", conversationID), "name", data.Name)

	// 返回对话 ID 和名称
	c.JSON(http.StatusOK, gin.H{
		"id": conversationID,
	})
}

func joinOrLeaveConversation(c *gin.Context, action string) {
	userId := c.Query("userId")
	conversationID := c.Query("conversationId")
	if userId == "" || conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId and conversationId are required"})
		return
	}

	key := fmt.Sprintf("conv:%s:users", conversationID)
	userKey := fmt.Sprintf("user:%s:convs", userId) // 改为哈希存储用户对话列表
	var err error
	if action == "join" {
		// 加入对话
		err = rdb.SAdd(c, key, userId).Err()      // 在对话成员集合中加入用户
		rdb.HSet(c, userKey, conversationID, "1") // 标记用户参与该对话
	} else {
		// 离开对话
		err = rdb.SRem(c, key, userId).Err() // 从对话成员集合中移除用户
		rdb.HDel(c, userKey, conversationID) // 删除用户的对话记录
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%sed conversation successfully", action)})
}

// 获取对话信息（成员和名称）
func getConversationInfo(c *gin.Context, convId string) (Conversation, error) {
	// 获取对话的成员信息
	memberStrings, err := rdb.SMembers(c, fmt.Sprintf("conv:%s:users", convId)).Result()
	if err != nil {
		return Conversation{}, err
	}

	// 将成员信息转换为 int 类型
	var members []int
	for _, member := range memberStrings {
		userIDInt, err := strconv.Atoi(member)
		if err != nil {
			fmt.Println("Error converting userID to int:", err)
			continue
		}
		members = append(members, userIDInt)
	}

	// 获取对话名称
	metaKey := fmt.Sprintf("conv:%s:meta", convId)
	name, err := rdb.HGet(c, metaKey, "name").Result()
	if err != nil {
		return Conversation{}, err
	}

	return Conversation{
		ID:      convId,
		Members: members,
		Name:    name,
	}, nil
}

// 获取用户参与的对话列表
func getConversationsByUser(c *gin.Context) {
	userId := c.Query("userId")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}

	userKey := fmt.Sprintf("user:%s:convs", userId)
	// 获取用户参与的对话 ID
	conversationIDs, err := rdb.HKeys(c, userKey).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var conversations []Conversation
	for _, convId := range conversationIDs {
		conv, err := getConversationInfo(c, convId)
		if err != nil {
			fmt.Println("Error retrieving conversation info:", err)
			continue
		}
		conversations = append(conversations, conv)
	}

	c.JSON(http.StatusOK, gin.H{
		"convs": conversations,
	})
}

// 获取单个对话的元数据（成员和名称）
func getConversation(c *gin.Context) {
	convId := c.DefaultQuery("convId", "") // 获取 convId 查询参数
	if convId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "convId is required"})
		return
	}

	conv, err := getConversationInfo(c, convId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch conversation information"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      conv.ID,
		"members": conv.Members,
		"name":    conv.Name,
	})
}

// 修改对话名称
func renameConversation(c *gin.Context) {
	var data struct {
		ConversationID string `json:"conversationId"`
		NewName        string `json:"newName"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	if data.ConversationID == "" || data.NewName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversationId and newName are required"})
		return
	}

	// 检查对话是否存在
	convKey := fmt.Sprintf("conv:%s:users", data.ConversationID)
	exists, err := rdb.Exists(c, convKey).Result()
	if err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}

	// 更新对话名称
	metaKey := fmt.Sprintf("conv:%s:meta", data.ConversationID)
	if err := rdb.HSet(c, metaKey, "name", data.NewName).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conversation renamed successfully"})
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Token")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	}
}

func main() {
	r := gin.Default()
	v1 := r.Group("api/v1")
	v1.Use(Cors())
	v1.GET("/ws", handleWebSocket)
	v1.GET("/history", getChatHistory)
	v1.POST("/create", createConversation)
	v1.POST("/join", func(c *gin.Context) { joinOrLeaveConversation(c, "join") })
	v1.POST("/leave", func(c *gin.Context) { joinOrLeaveConversation(c, "leave") })
	v1.GET("/list", getConversationsByUser)
	v1.POST("/rename", renameConversation)
	v1.GET("/conv", getConversation)
	r.Run(":8080")
}
