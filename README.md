# RedChat

## 一、软件概述
本软件是一个基于 Vue 和 Gin 框架开发的即时通讯应用，支持用户之间的实时聊天、查看会话列表、发送和接收消息等功能。通过 WebSocket 和 Redis 实现了高效的消息推送和存储，为用户提供了流畅的聊天体验。

## 二、技术栈
- **前端**：Vue.js、TypeScript
- **后端**：Gin 框架、Redis

## 三、安装与运行
1. 确保已经安装了以下软件：
   - Node.js 和 npm（用于前端开发）
   - Go（用于后端开发）
   - Redis（用于消息存储和推送）
2. 克隆项目到本地：
   ```
   git clone https://github.com/oboard/RedChat.git
   ```
3. 安装前端依赖：
   ```
   cd RedChat/redchat-frontend
   npm install
   ```
4. 安装后端依赖：
   ```
   cd RedChat
   go mod download
   ```
5. 启动 Redis 服务：
   ```
   redis-server
   ```
6. 运行后端服务：
   ```
   cd RedChat
   go run main.go
   ```
7. 运行前端服务：
   ```
   cd RedChat/redchat-frontend
   bun install
   bun dev
   ```

## 四、功能介绍
1. **会话列表**：在左侧面板展示用户的会话列表，用户可以通过点击会话切换聊天窗口。
2. **发送和接收消息**：用户可以在聊天窗口输入消息并发送，同时可以接收其他用户发送的消息。消息状态会显示为“发送中...”、“已接收”或“发送失败”。
3. **实时聊天**：通过 WebSocket 实现实时聊天，消息可以即时推送给参与会话的用户。
4. **历史聊天记录**：用户可以查看当前会话的历史聊天记录。
5. **加入和离开会话**：用户可以通过输入会话 ID 加入会话，也可以离开当前会话。

## 五、接口说明
1. **连接 WebSocket**：
   - 接口地址：`ws://127.0.0.1:8080/api/v1/ws?userId=<用户 ID>`
   - 方法：GET
   - 说明：建立 WebSocket 连接，用户 ID 为必填参数。
2. **获取历史聊天记录**：
   - 接口地址：`http://127.0.0.1:8080/api/v1/history?conversationId=<会话 ID>`
   - 方法：GET
   - 说明：获取指定会话的历史聊天记录，会话 ID 为必填参数。
3. **加入会话**：
   - 接口地址：`http://127.0.0.1:8080/api/v1/join`
   - 方法：POST
   - 参数：`userId=<用户 ID>&conversationId=<会话 ID>`
   - 说明：用户加入指定会话，用户 ID 和会话 ID 为必填参数。
4. **离开会话**：
   - 接口地址：`http://127.0.0.1:8080/api/v1/leave`
   - 方法：POST
   - 参数：`userId=<用户 ID>&conversationId=<会话 ID>`
   - 说明：用户离开指定会话，用户 ID 和会话 ID 为必填参数。
5. **获取用户会话列表**：
   - 接口地址：`http://127.0.0.1:8080/api/v1/list`
   - 方法：GET
   - 参数：`userId=<用户 ID>`
   - 说明：获取指定用户的会话列表，用户 ID 为必填参数。

## 六、注意事项
1. 确保 Redis 服务正常运行，并且配置正确的连接地址和端口。
2. 在生产环境中，需要配置合适的安全策略，如限制 WebSocket 连接的来源、对用户输入进行验证和过滤等。
3. 由于使用了本地开发环境的 IP 地址和端口，在实际部署时需要根据实际情况进行调整。