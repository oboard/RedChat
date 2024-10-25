package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

/**
 * @Author Administrator
 * @Description ip限速算法
 * @Date 2023/12/20 19:27
 * @Version 1.0
 */

// RequestInfo
// @Description: 请求信息
type RequestInfo struct {
	LastAccessTime time.Time // 上次访问时间
	RequestCount   int       // 请求计数
}

var (
	requestInfoMap = make(map[string]*RequestInfo) // IP到请求信息的映射
	mutex          = &sync.Mutex{}                 // 用于保护requestInfoMap的互斥锁
	maxRequests    = 30                            // 允许的最大请求数
	timeWindow     = 5 * time.Second               // 时间窗口
)

// RateLimitMiddleware
//
//	@Description: ip限速算法
//	@param c
func RateLimitMiddleware(c *gin.Context) {
	ip := c.ClientIP()
	mutex.Lock()
	defer mutex.Unlock()

	// 检查IP是否在map中
	info, exists := requestInfoMap[ip]

	// 如果IP不存在，初始化并添加到map中
	if !exists {
		requestInfoMap[ip] = &RequestInfo{LastAccessTime: time.Now(), RequestCount: 1}
		return
	}

	// 如果IP存在，检查时间窗口
	if time.Since(info.LastAccessTime) > timeWindow {
		// 如果超过时间窗口，重置请求计数
		info.RequestCount = 1
		info.LastAccessTime = time.Now()
		return
	}

	// 如果在时间窗口内，增加请求计数
	info.RequestCount++

	// 如果请求计数超过限制，禁止访问
	if info.RequestCount > maxRequests {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
		c.Abort()
		return
	}

	// 更新最后访问时间
	info.LastAccessTime = time.Now()
}
