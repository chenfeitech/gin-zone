package mobile

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gitee.com/jiang-xia/gin-zone/server/app/model"
	"gitee.com/jiang-xia/gin-zone/server/app/service"
	"gitee.com/jiang-xia/gin-zone/server/pkg/log"
	"gitee.com/jiang-xia/gin-zone/server/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
)

type Chat struct {
}

// 升级协议
var upGrader = websocket.Upgrader{
	// 解决跨域问题
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client 客户端结构体
type Client struct {
	ID         string
	IpAddress  string
	IpSource   string
	UserId     string
	Socket     *websocket.Conn
	SendChan   chan []byte
	Start      time.Time
	ExpireTime time.Duration // 一段时间没有接收到心跳则过期
}

// ClientManager 客户端管理结构体
type ClientManager struct {
	Clients        map[string]*Client // 记录在线用户 (上线增加一个客户端)
	BroadcastChan  chan []byte        //触发消息广播 (当前广播的消息)
	RegisterChan   chan *Client       // 触发新用户登陆 (当前登录的客户端)
	UnRegisterChan chan *Client       // 触发用户退出(当前退出的客户端)
}

// WsMessage 消息模板结构体
type WsMessage struct {
	Cmd   string `json:"cmd"`
	Count int    `json:"count"`
	model.ChatLog
	UserInfo interface{} `json:"userInfo"`
}

// Manager 管理实例声明
var Manager = ClientManager{
	Clients:        make(map[string]*Client), // 初始化chan(实例化)
	BroadcastChan:  make(chan []byte),
	RegisterChan:   make(chan *Client),
	UnRegisterChan: make(chan *Client),
}

// Read 读取客户端发送过来的消息
func (c *Client) Read() {
	// 出现故障后把当前客户端注销(即不是无限循环 阻塞着就关闭)
	defer func() {
		fmt.Println("用户:", c.UserId, "关闭Socket连接")
		_ = c.Socket.Close()
		// 退出
		Manager.UnRegisterChan <- c
	}()
	//sum := 0
	// 无限循环 应用服务器常用于监听
	for {
		//sum++
		_, data, err := c.Socket.ReadMessage()
		//var err error
		//var data []byte
		if err != nil {
			log.Error(err.Error())
			break
		}
		//ioutil.WriteFile("./nani.mp4", []byte(data), 0644)
		var msg WsMessage
		err = json.Unmarshal(data, &msg)
		// fmt.Println("WsMessage===========>", msg)
		if err != nil {
			log.Error(err.Error())
			break
		}
		//fmt.Printf("客户端所发信息:%+v\n ", msg)
		switch msg.Cmd {
		case "heartbeat":
			var hMsg = map[string]interface{}{}
			hMsg["cmd"] = msg.Cmd
			hMsg["content"] = "ok"
			hMsg["senderId"] = msg.SenderId
			// 如果是心跳监测消息（利用心跳监测来判断对应客户端是否在线）
			resp, _ := json.Marshal(hMsg)
			c.Start = time.Now() // 重新刷新时间
			// 发送变量到 SendChan 通道中
			c.SendChan <- resp
		case "online":
			// 获取在线人数
			count := len(Manager.Clients)
			msg.Count = count
			resp, _ := json.Marshal(msg)
			c.SendChan <- resp
		case "text":
			// 发送文本消息
			chatLog := &model.ChatLog{
				SenderId:   msg.SenderId,
				ReceiverId: msg.ReceiverId,
				GroupId:    msg.GroupId,
				Content:    msg.Content,
				LogType:    msg.LogType,
				MsgType:    msg.MsgType,
			}
			service.Chat.CreateChatLog(chatLog)
			//查询用户信息
			UserInfo, _ := service.User.GetByUserId(chatLog.SenderId)
			msg.UserInfo = UserInfo
			UserInfo.Password = ""
			resp, _ := json.Marshal(msg)
			Manager.BroadcastChan <- resp
		case "recall":
			// 你的撤回消息的操作
			c.SendChan <- []byte("回复消息")

		}
		//fmt.Println("sum:", sum, "次")
	}
}

// Write 把对应消息写回客户端
func (c *Client) Write() {
	defer func() {
		_ = c.Socket.Close()
		Manager.UnRegisterChan <- c
	}()
	for {
		select {
		//监听是否有消息发送
		case msg, ok := <-c.SendChan:
			if !ok {
				// 没有消息则发送空响应
				err := c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					log.Error(err.Error())
					return
				}
				return
			}
			var wsMsg WsMessage
			err := json.Unmarshal(msg, &wsMsg)
			//fmt.Printf("服务端所发信息:%+v ", wsMsg)
			err = c.Socket.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}
	}
}

// Check 实时监测过期
func (c *Client) Check() {
	for {
		now := time.Now()
		var duration = now.Sub(c.Start)
		if duration >= c.ExpireTime {
			// 过期退出
			Manager.UnRegisterChan <- c
			break
		}
	}
}

// Start 开始监听有人连接
func (manager *ClientManager) Start() {
	for {
		select {
		case conn := <-Manager.RegisterChan:
			Manager.Clients[conn.ID] = conn
			// 如果有新用户连接则发送最近聊天记录和在线人数给他
			count := len(Manager.Clients)
			Manager.InitSend(conn, count)
		}
	}
}

// InitSend 初始化客户端管理器
func (manager *ClientManager) InitSend(cur *Client, count int) {
	// 初始化时发送在线人数
	resp, _ := json.Marshal(&WsMessage{Cmd: "online", Count: count})
	Manager.BroadcastChan <- resp
}

// BroadcastSend 群发消息
func (manager *ClientManager) BroadcastSend() {
	for {
		select {
		// 只要有一方发消息就广播
		case msg := <-Manager.BroadcastChan:
			wsMsg := WsMessage{}
			err := json.Unmarshal(msg, &wsMsg)
			if err != nil {
				wsMsg.Content = "消息解析错误:" + err.Error()
				log.Error(wsMsg.Content)
			}
			//fmt.Printf("WsMessage消息模板:%+v\n", wsMsg)
			//fmt.Println("群聊id:", wsMsg.GroupId)
			//fmt.Println("当前连接者id:", wsMsg.GroupId)
			//fmt.Println("接受者id:", wsMsg.ReceiverId)
			//群聊时找到所有群成员广播消息
			if wsMsg.GroupId != 0 {
				list := service.Chat.ChatGroupMember(wsMsg.GroupId)
				// fmt.Println("群组成员列表：", len(list))

				//遍历所有群成员
				for _, member := range list {
					//找到所有在线的群成员用户(在线实例用户id和群成员用户id一致)
					for _, conn := range Manager.Clients {
						//自己发消息时，不用广播给自己
						if wsMsg.SenderId == conn.UserId {
							continue
						}
						if member.UserId == conn.UserId {
							// fmt.Println("群组成员：", conn.UserId)
							conn.SendChan <- msg
							break
						}
					}
				}
			} else if wsMsg.ReceiverId != "" {
				//	私聊时找到对应结接收方用户广播消息
				for _, conn := range Manager.Clients {
					//掉线了可能就找不到对应的在线实例
					fmt.Println("找到对应接受者用户", wsMsg.ReceiverId, conn.UserId)
					//接受者is和当前连接实例相等时
					if wsMsg.ReceiverId == conn.UserId {
						conn.SendChan <- msg
						break
					}
				}
			}
			//fmt.Printf("客户端发送的消息:%+v", wsMsg)
			////广播给所有的在线客户端
			//for _, conn := range Manager.Clients {
			//	conn.SendChan <- msg
			//}
		}
	}
}

// Quit 离线用户触发删除
func (manager *ClientManager) Quit() {
	for {
		select {
		case conn := <-Manager.UnRegisterChan:
			//删除对应在线客户端
			delete(Manager.Clients, conn.ID)
			// 给客户端刷新在线人数
			resp, _ := json.Marshal(&WsMessage{Cmd: "online", Count: len(Manager.Clients)})
			//有人退出时 广播刷新在线人数
			manager.BroadcastChan <- resp
		}
	}
}

// 初始化 执行客户端管理方法
func init() {
	go Manager.Start()
	go Manager.Quit()
	go Manager.BroadcastSend()
}

// WebSocketHandle webSocket升级协议，并且初始化上线用户数据
func (ch *Chat) WebSocketHandle(ctx *gin.Context) {
	conn, err := (&websocket.Upgrader{
		// 决解跨域问题
		CheckOrigin: func(r *http.Request) bool { return true },
	}).Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		http.NotFound(ctx.Writer, ctx.Request)
		log.Error(err.Error())
		return
	}
	userId := ctx.Query("userId")
	ip := ctx.ClientIP()
	//addr, err := common.GetIpAddressAndSource(ip)
	if err != nil {
		http.NotFound(ctx.Writer, ctx.Request)
		log.Error(err.Error())
		return
	}
	ua := ctx.GetHeader("User-Agent")
	id := ip + ua
	idMd5 := fmt.Sprintf("%x", md5.Sum([]byte(id)))
	//新用户(新的客户端) 连上就新建一个客户端实例
	client := &Client{
		ID:         idMd5,
		Socket:     conn,
		SendChan:   make(chan []byte),
		IpAddress:  ip,
		IpSource:   "未知",
		UserId:     userId,
		Start:      time.Now(),
		ExpireTime: time.Minute * 1,
	}
	// 使用通道Register发送变量client
	Manager.RegisterChan <- client
	fmt.Printf("client%v\n", client)
	go client.Read() // 以goroutine的方式调用Client的Read、Write、Check方法
	go client.Write()
	go client.Check()
}

/*
*
* 聊天相关接口开始==============================>
*
 */

//	FriendList godoc
//
// @Summary     好友列表
// @Description 好友列表
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       userId   query     int            true  "User.ID"
// @Success     200  {object} response.ResType
// @Router      /mobile/chat/friends [get]
func (ch *Chat) FriendList(c *gin.Context) {
	userId := c.Query("userId")
	if userId == "" {
		response.Fail(c, "用户id不能为空", []string{})
		return
	}
	friends := service.Chat.ChatFriends(userId)
	response.Success(c, friends, "")
}

// AddFriend godoc
//
// @Summary     添加好友关系
// @Description 添加好友或加入群聊
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       user body     model.AddFriend true "需要上传的json"
// @Success     200  {object} model.AddFriend
// @Router      /mobile/chat/friends [post]
func (ch *Chat) AddFriend(c *gin.Context) {
	addFriend := &model.AddFriend{}
	if err := c.ShouldBindJSON(&addFriend); err != nil {
		// 字段参数校验
		response.Fail(c, "参数错误", err.Error())
		return
	}
	err := service.Chat.CreateChatFriends(&model.ChatFriends{
		UserId:       addFriend.UserId,
		FriendId:     addFriend.FriendId,
		GroupId:      addFriend.GroupId,
		LastReadTime: model.Time{},
		LastInfoTime: model.Time{},
	})
	if addFriend.FriendId != "" {
		//对方 好友列表同时加上自身
		err = service.Chat.CreateChatFriends(&model.ChatFriends{
			UserId:       addFriend.FriendId,
			FriendId:     addFriend.UserId,
			LastReadTime: model.Time{},
			LastInfoTime: model.Time{},
		})
	}
	if err != nil {
		response.Fail(c, err.Error(), nil)
		return
	}
	response.Success(c, addFriend, "添加成功")
}

// DelFriend godoc
//
// @Summary     删除好友
// @Description 删除好友
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       friendId path  string true "好友id"
// @Success     200  {object}  response.ResType
// @Router      /mobile/chat/friends/{friendId} [delete]
func (ch *Chat) DelFriend(c *gin.Context) {
	userId := model.GetUserUid(c)
	friendId := c.Param("friendId")
	fmt.Println(userId, "friendId", friendId)
	bool := service.Chat.DeleteChatFriends(userId, friendId)
	//互删
	bool = service.Chat.DeleteChatFriends(friendId, userId)
	if !bool {
		response.Fail(c, "删除失败", nil)
		return
	}
	response.Success(c, bool, "删除成功")
}

// UpdateReadTime godoc
//
// @Summary     更新阅读时间
// @Description 更新上次阅读信息时间
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       query body     model.UpdateReadTime true "需要上传的json"
// @Success     200  {boolean} true
// @Router      /mobile/chat/updateReadTime [post]
func (ch *Chat) UpdateReadTime(c *gin.Context) {
	var query model.UpdateReadTime
	if err := c.ShouldBindJSON(&query); err != nil {
		response.Fail(c, "参数错误", err.Error())
		return
	}
	err := service.Chat.UpdateLastReadTime(&query)
	if err != nil {
		response.Fail(c, err.Error(), nil)
		return
	}
	response.Success(c, true, "操作成功")
}

// ChatLogList godoc
//
// @Summary     聊天记录
// @Description 聊天记录
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       query body     model.ChatLogQuery true "需要上传的json"
// @Success     200 {array} model.ChatLog
// @Router     /mobile/chat/logs [post]
func (ch *Chat) ChatLogList(c *gin.Context) {
	query := &model.ChatLogQuery{}
	if err := c.ShouldBindJSON(&query); err != nil {
		response.Fail(c, "参数错误", err.Error())
		return
	}
	// fmt.Printf("ChatLogList查询参数: %+v", query)
	list, total := service.Chat.ChatLogList(query.Page, query.PageSize, query)
	data := model.ListRes{List: list, Total: total}
	response.Success(c, data, "")
}

//	GroupList godoc
//
// @Summary     群组列表
// @Description 群组列表
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       userId   query     int            false  "User.ID"
// @Param       groupName   query     string            false  "Group.groupName"
// @Success     200  {object} response.ResType
// @Router      /mobile/chat/groups [get]
func (ch *Chat) GroupList(c *gin.Context) {
	userId := c.Query("userId")
	groupName := c.Query("groupName")
	list := service.Chat.ChatGroup(userId, groupName)
	response.Success(c, list, "")
}

// AddGroup godoc
//
// @Summary     创建群聊
// @Description 创建群聊
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       user body     model.ChatGroup true "需要上传的json"
// @Success     200  {object} model.ChatGroup
// @Router      /mobile/chat/groups [post]
func (ch *Chat) AddGroup(c *gin.Context) {
	model := &model.ChatGroup{}
	if err := c.ShouldBindJSON(&model); err != nil {
		// 字段参数校验
		response.Fail(c, "参数错误", err.Error())
		return
	}
	err := service.Chat.CreateGroup(model)
	if err != nil {
		response.Fail(c, "创建失败", err.Error())
		return
	}
	response.Success(c, model.ID, "添加成功")
}

// DelGroup godoc
//
// @Summary     删除群聊
// @Description 删除群聊
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       groupId path    string true "群组id"
// @Success     200  {object} response.ResType
// @Router      /mobile/chat/groups/{groupId} [delete]
func (ch *Chat) DelGroup(c *gin.Context) {
	userId := model.GetUserUid(c)
	groupId := cast.ToInt(c.Param("groupId"))
	bool := service.Chat.DeleteGroup(userId, groupId)
	if !bool {
		response.Fail(c, "删除失败", nil)
		return
	}
	response.Success(c, bool, "删除成功")
}

//	GroupList godoc
//
// @Summary     群成员列表
// @Description 群成员列表
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       groupId   query     int            true  "ChatGroup.ID"
// @Success     200  {object} response.ResType
// @Router      /mobile/chat/groupMembers [get]
func (ch *Chat) GroupMemberList(c *gin.Context) {
	groupId := c.Query("groupId")
	list := service.Chat.ChatGroupMember(cast.ToInt(groupId))
	response.Success(c, list, "")
}

// AddGroupMember godoc
//
// @Summary     添加群成员
// @Description 添加群成员
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       user body     model.ChatGroupMember true "需要上传的json"
// @Success     200  {object} model.ChatGroupMember
// @Router      /mobile/chat/groupMembers [post]
func (ch *Chat) AddGroupMember(c *gin.Context) {
	model := &model.ChatGroupMember{}
	if err := c.ShouldBindJSON(&model); err != nil {
		// 字段参数校验
		response.Fail(c, "参数错误", err)
		return
	}
	err := service.Chat.CreateChatGroupMember(model)
	if err != nil {
		response.Fail(c, err.Error(), nil)
		return
	}
	response.Success(c, model.ID, "添加成功")
}

// ExitGroupMember godoc
//
// @Summary     退出群聊
// @Description 退出群聊
// @Tags        聊天模块
// @Security	Authorization
// @Accept      json
// @Produce     json
// @Param       groupId path   string true "需要上传的json"
// @Success     200  {object} response.ResType
// @Router      /mobile/chat/groupMembers/{groupId} [delete]
func (ch *Chat) ExitGroupMember(c *gin.Context) {
	userId := model.GetUserUid(c)
	groupId := cast.ToInt(c.Param("groupId"))
	bool := service.Chat.DeleteChatGroupMember(userId)
	bool = service.Chat.DeleteGroupFriends(userId, groupId)
	//删除群关系
	if !bool {
		response.Fail(c, "退出失败", nil)
		return
	}
	response.Success(c, bool, "退出成功")
}
