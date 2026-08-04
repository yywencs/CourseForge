package danmakuws

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	danmakuv1 "github.com/yywencs/courseforge/gen/courseforge/danmaku/v1"
	"github.com/yywencs/courseforge/internal/danmaku/domain"
	"github.com/yywencs/courseforge/server/http/common"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultAuthenticationTimeout = 10 * time.Second
	maxRequestIDLength           = 128
)

// TokenVerifier 将首帧携带的学生访问令牌验证为学生编号。
type TokenVerifier interface {
	// Verify 校验学生访问令牌，并返回令牌中已经验证的学生编号。
	Verify(string) (uint64, error)
}

// VideoReader 提供实时连接建立前所需的视频事实快照。
type VideoReader interface {
	// GetVideo 返回指定课程视频的播放状态快照。
	GetVideo(context.Context, uint64) (*danmaku.VideoTarget, error)
}

// Routes 提供实时弹幕 WebSocket 升级和首帧鉴权入口。
type Routes struct {
	hub                   *Hub
	tokens                TokenVerifier
	videos                VideoReader
	upgrader              websocket.Upgrader
	connectionSettings    connectionSettings
	authenticationTimeout time.Duration
	now                   func() time.Time
}

// NewRoutes 创建实时弹幕 WebSocket 路由。
func NewRoutes(hub *Hub, tokens TokenVerifier, videos VideoReader) *Routes {
	return &Routes{
		hub:    hub,
		tokens: tokens,
		videos: videos,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     checkWebSocketOrigin,
		},
		connectionSettings:    connectionSettings{}.withDefaults(),
		authenticationTimeout: defaultAuthenticationTimeout,
		now:                   time.Now,
	}
}

// RegisterAPIRoutes 注册实时弹幕 WebSocket 入口。
// JWT 必须通过升级后的首个 Protobuf 二进制帧传递，因此这里不安装 HTTP 鉴权中间件。
func (r *Routes) RegisterAPIRoutes(root *gin.RouterGroup) {
	root.GET("/course-videos/:video_id/danmakus/realtime", r.Connect)
}

// Connect 将 HTTP 请求升级为 WebSocket，并在首帧鉴权成功后注册本机连接。
func (r *Routes) Connect(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("video_id"), 10, 64)
	if err != nil || videoID == 0 {
		common.Error(c, http.StatusBadRequest, "课程视频编号不正确")
		return
	}

	connection, err := r.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	authentication, failure := r.authenticate(c.Request.Context(), connection, videoID)
	if failure != nil {
		r.rejectConnection(connection, failure.requestID, *failure)
		return
	}

	client := newClientConn(
		authentication.studentID,
		videoID,
		connection,
		r.connectionSettings,
	)
	if _, err := r.hub.Register(c.Request.Context(), client); err != nil {
		r.rejectConnection(connection, authentication.requestID, protocolFailure{
			code:    danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_INTERNAL,
			message: "实时弹幕服务暂时不可用", retryable: true,
		})
		return
	}

	readyPayload, err := proto.Marshal(&danmakuv1.ServerFrame{
		RequestId: authentication.requestID,
		Payload: &danmakuv1.ServerFrame_ConnectionReady{
			ConnectionReady: &danmakuv1.ConnectionReady{
				VideoId: videoID, ConnectedAt: timestamppb.New(r.now()),
			},
		},
	})
	if err != nil {
		r.unregisterClient(client)
		return
	}
	select {
	case client.send <- readyPayload:
	default:
		r.unregisterClient(client)
		return
	}

	go client.writePump(c.Request.Context())
	client.readPump(c.Request.Context(), r.hub, r.handleAuthenticatedFrame)
}

// authenticationResult 保存首帧鉴权成功后建立连接所需的可信身份信息。
type authenticationResult struct {
	studentID uint64
	requestID string
}

// protocolFailure 描述可以安全返回给客户端的稳定协议错误。
type protocolFailure struct {
	requestID string
	code      danmakuv1.RealtimeErrorCode
	message   string
	retryable bool
}

// authenticate 读取并校验升级后的首个二进制帧。
// 只有令牌有效且目标视频可播放时，连接才有资格注册到 Hub。
func (r *Routes) authenticate(
	ctx context.Context,
	connection websocketConnection,
	videoID uint64,
) (authenticationResult, *protocolFailure) {
	connection.SetReadLimit(r.connectionSettings.readLimit)
	if err := connection.SetReadDeadline(r.now().Add(r.authenticationTimeout)); err != nil {
		return authenticationResult{}, internalProtocolFailure("")
	}
	messageType, payload, err := connection.ReadMessage()
	if err != nil || messageType != websocket.BinaryMessage {
		return authenticationResult{}, &protocolFailure{
			code:    danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_INVALID_MESSAGE,
			message: "首条消息必须是鉴权 Protobuf 二进制帧",
		}
	}

	var frame danmakuv1.ClientFrame
	if err := proto.Unmarshal(payload, &frame); err != nil {
		return authenticationResult{}, &protocolFailure{
			code:    danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_INVALID_MESSAGE,
			message: "鉴权消息格式不正确",
		}
	}
	requestID := strings.TrimSpace(frame.GetRequestId())
	authenticate := frame.GetAuthenticate()
	if requestID == "" || len(requestID) > maxRequestIDLength || authenticate == nil ||
		strings.TrimSpace(authenticate.GetAccessToken()) == "" {
		return authenticationResult{}, &protocolFailure{
			requestID: requestID,
			code:      danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_INVALID_MESSAGE,
			message:   "鉴权消息缺少必要字段",
		}
	}
	if r.tokens == nil || r.videos == nil || r.hub == nil {
		return authenticationResult{}, internalProtocolFailure(requestID)
	}

	studentID, err := r.tokens.Verify(authenticate.GetAccessToken())
	if err != nil || studentID == 0 {
		return authenticationResult{}, &protocolFailure{
			requestID: requestID,
			code:      danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_UNAUTHENTICATED,
			message:   "访问令牌无效或已过期",
		}
	}
	video, err := r.videos.GetVideo(ctx, videoID)
	if err != nil {
		if errors.Is(err, danmaku.ErrVideoNotFound) || errors.Is(err, danmaku.ErrVideoNotPlayable) {
			return authenticationResult{}, videoNotPlayableFailure(requestID)
		}
		return authenticationResult{}, internalProtocolFailure(requestID)
	}
	if video == nil || video.EnsurePlayable(videoID) != nil {
		return authenticationResult{}, videoNotPlayableFailure(requestID)
	}
	return authenticationResult{studentID: studentID, requestID: requestID}, nil
}

// handleAuthenticatedFrame 处理鉴权完成后的客户端数据帧。
// 当前协议只允许服务端主动推送弹幕，因此额外数据帧统一返回协议错误。
func (r *Routes) handleAuthenticatedFrame(
	_ context.Context,
	client *clientConn,
	payload []byte,
) {
	var frame danmakuv1.ClientFrame
	_ = proto.Unmarshal(payload, &frame)
	r.enqueueServerFrame(client, &danmakuv1.ServerFrame{
		RequestId: frame.GetRequestId(),
		Payload: &danmakuv1.ServerFrame_Error{Error: &danmakuv1.RealtimeError{
			Code:    danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_INVALID_MESSAGE,
			Message: "连接鉴权完成后不接受客户端数据消息",
		}},
	})
}

// enqueueServerFrame 编码服务端协议帧并非阻塞地写入连接发送队列。
// 队列已满意味着客户端无法及时消费，此时关闭连接以避免持续堆积。
func (r *Routes) enqueueServerFrame(client *clientConn, frame *danmakuv1.ServerFrame) {
	payload, err := proto.Marshal(frame)
	if err != nil {
		client.close()
		return
	}
	select {
	case client.send <- payload:
	default:
		client.close()
	}
}

// rejectConnection 在 WebSocket 升级后返回协议错误，并使用策略违规关闭码终止连接。
func (r *Routes) rejectConnection(
	connection websocketConnection,
	requestID string,
	failure protocolFailure,
) {
	defer connection.Close()
	payload, err := proto.Marshal(&danmakuv1.ServerFrame{
		RequestId: requestID,
		Payload: &danmakuv1.ServerFrame_Error{Error: &danmakuv1.RealtimeError{
			Code: failure.code, Message: failure.message, Retryable: failure.retryable,
		}},
	})
	if err == nil && connection.SetWriteDeadline(r.now().Add(r.connectionSettings.writeTimeout)) == nil {
		_ = connection.WriteMessage(websocket.BinaryMessage, payload)
	}
	_ = connection.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, failure.message),
		r.now().Add(r.connectionSettings.writeTimeout),
	)
}

// unregisterClient 清理已注册但尚未启动完整读写循环的连接。
func (r *Routes) unregisterClient(client *clientConn) {
	ctx, cancel := context.WithTimeout(context.Background(), r.connectionSettings.writeTimeout)
	defer cancel()
	_, _, _ = r.hub.Unregister(ctx, client)
	_ = client.connection.Close()
}

// internalProtocolFailure 创建允许客户端稍后重试的内部错误。
func internalProtocolFailure(requestID string) *protocolFailure {
	return &protocolFailure{
		requestID: requestID,
		code:      danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_INTERNAL,
		message:   "实时弹幕服务暂时不可用",
		retryable: true,
	}
}

// videoNotPlayableFailure 创建不暴露视频具体状态的统一播放错误。
func videoNotPlayableFailure(requestID string) *protocolFailure {
	return &protocolFailure{
		requestID: requestID,
		code:      danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_VIDEO_NOT_PLAYABLE,
		message:   "课程视频当前不可播放",
	}
}

// checkWebSocketOrigin 允许同源请求以及本机不同端口间的开发代理请求。
// 非浏览器客户端可能不携带 Origin，此时仍依赖首帧 JWT 完成身份校验。
func checkWebSocketOrigin(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	requestHost := request.Host
	if host, _, err := net.SplitHostPort(requestHost); err == nil {
		requestHost = host
	}
	if strings.EqualFold(parsed.Hostname(), requestHost) {
		return true
	}
	return isLoopbackHost(parsed.Hostname()) && isLoopbackHost(requestHost)
}

// isLoopbackHost 判断主机名或 IP 是否指向本机回环地址。
func isLoopbackHost(host string) bool {
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}
