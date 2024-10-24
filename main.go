package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

const expirationTime = time.Hour * 24 * 30 // 30 天

var rdb *redis.Client

func init() {
	// 从环境变量中获取 REDIS_ADDR，如果没有则使用默认值 "localhost:6379"
	redisAddr := "localhost:6379"
	if envRedisAddr := os.Getenv("REDIS_ADDR"); envRedisAddr != "" {
		redisAddr = envRedisAddr
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})
}

type Message struct {
	UUID           string `json:"uuid"`
	Content        string `json:"content"`
	UserID         int    `json:"userId"`
	ConversationID string `json:"conversationId"`
	Time           string `json:"time"`
	Type           string `json:"type"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleWebSocket(c *gin.Context) {
	userId := c.Query("userId")
	if userId == "" {
		c.JSON(400, gin.H{"error": "userId is required"})
		return
	}

	conversationIds := getConversationsByUserId(c, userId)
	conversationKeys := make([]string, len(conversationIds))
	for i, conversationId := range conversationIds {
		conversationKeys[i] = fmt.Sprintf("conversation:%s", conversationId)
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("升级为 WebSocket 失败：", err)
		return
	}
	defer ws.Close()

	pubsub := rdb.Subscribe(c, conversationKeys...)
	defer pubsub.Close()

	ch := pubsub.Channel()
	go func() {
		for msg := range ch {
			var message Message
			// 判断数据类型，如果是数字类型则跳过
			if _, err := strconv.Atoi(msg.Payload); err == nil {
				fmt.Println("无效数据，跳过：", msg.Payload)
				continue
			}
			err := json.Unmarshal([]byte(msg.Payload), &message)
			if err != nil {
				fmt.Println("反序列化错误：", err)
				continue
			}

			// 检查消息接收者是否在当前会话中
			if isUserInConversation(c, message.UserID, message.ConversationID) {
				err = ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
				if err != nil {
					fmt.Println("发送 WebSocket 消息失败：", err)
					break
				}
			}
		}
	}()

	for {
		_, messageBytes, err := ws.ReadMessage()
		if err != nil {
			fmt.Println("读取 WebSocket 消息错误：", err)
			break
		}
		var msg Message
		err = json.Unmarshal(messageBytes, &msg)
		if err != nil {
			fmt.Println("反序列化 WebSocket 消息错误：", err)
			continue
		}
		// msg.UUID = uuid.New().String()
		msg.Time = time.Now().Format(time.RFC3339)
		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("序列化消息错误：", err)
			continue
		}
		key := fmt.Sprintf("conversation:%s", msg.ConversationID)
		err = rdb.Publish(c, key, jsonMsg).Err()
		if err != nil {
			fmt.Println("发布消息到 Redis 错误：", err)
			continue
		}
		err = rdb.RPush(c, key, jsonMsg).Err()
		if err != nil {
			fmt.Println("存储历史消息失败：", err)
		} else {
			// 设置一个月后过期
			expiration := expirationTime
			err = rdb.Expire(c, key, expiration).Err()
			if err != nil {
				fmt.Println("设置过期时间失败：", err)
			}
		}
		jsonMsg, err = json.Marshal(msg)
		if err != nil {
			fmt.Println("重新序列化消息错误：", err)
			continue
		}
		err = ws.WriteMessage(websocket.TextMessage, []byte(jsonMsg))
		if err != nil {
			fmt.Println("发送更新后的 WebSocket 消息失败：", err)
			break
		}
	}
}

func getUsersByConversation(c *gin.Context, conversationID string) []string {
	key := fmt.Sprintf("conversation:%s:users", conversationID)
	users, err := rdb.SMembers(c, key).Result()
	if err != nil {
		fmt.Println("获取会话参与者失败：", err)
		return nil
	}
	return users
}

func getChatHistory(c *gin.Context) {
	conversationID := c.Query("conversationId")
	if conversationID == "" {
		c.JSON(400, gin.H{"error": "conversationId is required"})
		return
	}
	key := fmt.Sprintf("conversation:%s", conversationID)
	// 使用 Redis 的 SORT 命令对列表进行排序
	result, err := rdb.Sort(c, key, &redis.Sort{
		By:    "time:*",
		Order: "ASC",
	}).Result()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	var msgs []Message
	for _, item := range result {
		var message Message
		// 判断数据类型，如果是数字类型则跳过
		if _, err := strconv.Atoi(item); err == nil {
			fmt.Println("无效数据，跳过：", item)
			continue
		}
		err := json.Unmarshal([]byte(item), &message)
		if err != nil {
			fmt.Println("反序列化错误：", err)
			continue
		}
		msgs = append(msgs, message)
	}
	c.JSON(200, gin.H{"msgs": msgs})
}

func createConversation(c *gin.Context) {
	var data struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// fromUserId := data.from
	// toUserId := data.to
	// 小的为from，大的为 to
	fromUserId := data.From
	toUserId := data.To
	// 打印
	fmt.Println("fromUserId:", fromUserId)
	fmt.Println("toUserId:", toUserId)
	if fromUserId > toUserId {
		fromUserId, toUserId = toUserId, fromUserId
	}
	// 打印
	fmt.Println("fromUserId:", fromUserId)
	fmt.Println("toUserId:", toUserId)
	// 检查用户 ID 是否存在

	// if rdb.Exists(c, fmt.Sprintf("user:%d:*", fromUserId)).Val() == 0 {
	// 	c.JSON(400, gin.H{"error": fmt.Sprintf("user:%d", fromUserId)})
	// 	return
	// }
	// if rdb.Exists(c, fmt.Sprintf("user:%d", toUserId)).Val() == 0 {
	// 	c.JSON(400, gin.H{"error": "userId2 not found"})
	// 	return
	// }
	// 生成新的会话 ID
	// conversationID := uuid.New().String()
	conversationID := fmt.Sprintf("%dT%d", fromUserId, toUserId)
	// 将会话 ID 存储到两个用户的哈希表中
	user1Key := fmt.Sprintf("user:%d:conversations", fromUserId)
	user2Key := fmt.Sprintf("user:%d:conversations", toUserId)
	err := rdb.HSet(c, user1Key, conversationID, "1").Err()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	err = rdb.HSet(c, user2Key, conversationID, "1").Err()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// 将会话 ID 存储到会话的用户列表中
	key1 := fmt.Sprintf("conversation:%s:users", conversationID)
	err = rdb.SAdd(c, key1, fromUserId, toUserId).Err()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"conversationId": conversationID})
}

func joinConversation(c *gin.Context) {
	userId := c.Query("userId")
	conversationID := c.Query("conversationId")
	if userId == "" || conversationID == "" {
		c.JSON(400, gin.H{"error": "userId and conversationId are required"})
		return
	}
	// 将用户 ID 加入到会话的用户列表中
	key := fmt.Sprintf("conversation:%s:users", conversationID)
	err := rdb.SAdd(c, key, userId).Err()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// 将会话 ID 存储到用户的哈希表中
	userKey := fmt.Sprintf("user:%s:conversations", userId)
	err = rdb.HSet(c, userKey, conversationID, "1").Err()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "joined conversation successfully"})
}

func leaveConversation(c *gin.Context) {
	userId := c.Query("userId")
	conversationID := c.Query("conversationId")
	if userId == "" || conversationID == "" {
		c.JSON(400, gin.H{"error": "userId and conversationId are required"})
		return
	}
	// 从会话的用户列表中移除用户 ID
	key := fmt.Sprintf("conversation:%s:users", conversationID)
	err := rdb.SRem(c, key, userId).Err()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// 从用户的哈希表中移除会话 ID
	userKey := fmt.Sprintf("user:%s:conversations", userId)
	err = rdb.HDel(c, userKey, conversationID).Err()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "left conversation successfully"})
}

func isUserInConversation(c *gin.Context, userID int, conversationID string) bool {
	key := fmt.Sprintf("conversation:%s:users", conversationID)
	isMember, err := rdb.SIsMember(c, key, fmt.Sprintf("%d", userID)).Result()
	if err != nil {
		fmt.Println("检查用户是否在会话中错误：", err)
		return false
	}
	return isMember
}

func getConversationsByUserId(c *gin.Context, userId string) []string {
	key := fmt.Sprintf("user:%s:conversations", userId)
	ids, err := rdb.HKeys(c, key).Result()
	if err != nil {
		fmt.Println("获取用户会话 ID 错误：", err)
		return nil
	}
	return ids
}

func getUserConversations(c *gin.Context) {
	userId := c.Query("userId")
	if userId == "" {
		c.JSON(400, gin.H{"error": "userId is required"})
		return
	}
	ids := getConversationsByUserId(c, userId)
	if ids == nil {
		c.JSON(500, gin.H{"error": "no conversations found for this user"})
		return
	}
	c.JSON(200, gin.H{"conversations": ids})
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization, Token")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")

		//放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		// 处理请求
		c.Next()
	}
}

func main() {
	router := gin.Default()

	// 处理 WebSocket 连接，包括发送和订阅消息
	// 加一层路由/api/v1
	v1 := router.Group("api/v1")
	{

		// 允许跨域请求
		v1.Use(Cors())
		v1.GET("/ws", handleWebSocket)
		v1.GET("/history", getChatHistory)
		v1.POST("/create", createConversation)
		v1.POST("/join", joinConversation)
		v1.POST("/leave", leaveConversation)
		v1.GET("/list", getUserConversations)
	}

	// redchat-frontend/dist 扫描静态文件目录

	router.Run(":8080")
}
