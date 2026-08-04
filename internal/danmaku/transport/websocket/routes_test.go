package danmakuws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	danmakuv1 "github.com/yywencs/courseforge/gen/courseforge/danmaku/v1"
	"github.com/yywencs/courseforge/internal/danmaku/domain"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type tokenVerifierStub struct {
	studentID uint64
	err       error
}

func (s tokenVerifierStub) Verify(string) (uint64, error) {
	return s.studentID, s.err
}

type videoReaderStub struct {
	video *danmaku.VideoTarget
	err   error
}

func (s videoReaderStub) GetVideo(context.Context, uint64) (*danmaku.VideoTarget, error) {
	return s.video, s.err
}

func TestRoutesRegisterRealtimeContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewRoutes(nil, nil, nil).RegisterAPIRoutes(engine.Group("/api/v1"))
	routes := engine.Routes()
	if len(routes) != 1 || routes[0].Method != http.MethodGet ||
		routes[0].Path != "/api/v1/course-videos/:video_id/danmakus/realtime" {
		t.Fatalf("routes = %#v", routes)
	}
}

func TestConnectAuthenticatesFirstBinaryFrameAndRegistersClient(t *testing.T) {
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })
	routes := NewRoutes(
		hub,
		tokenVerifierStub{studentID: 1001},
		videoReaderStub{video: playableVideo(7)},
	)
	server := newRealtimeTestServer(t, routes)
	connection := dialRealtime(t, server.URL, "/api/v1/course-videos/7/danmakus/realtime")
	defer connection.Close()

	writeAuthenticationFrame(t, connection, "request-1", "student-token")
	frame := readServerFrame(t, connection)
	ready := frame.GetConnectionReady()
	if frame.GetRequestId() != "request-1" || ready == nil || ready.GetVideoId() != 7 ||
		ready.GetConnectedAt() == nil {
		t.Fatalf("connection ready frame = %v", frame)
	}
	if count := hub.connections.Counts()[7]; count != 1 {
		t.Fatalf("video connection count = %d, want 1", count)
	}

	if err := connection.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"),
	); err != nil {
		t.Fatalf("write close frame: %v", err)
	}
	waitForCondition(t, func() bool { return len(hub.connections.Counts()) == 0 })
}

func TestConnectRejectsInvalidTokenAfterUpgrade(t *testing.T) {
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })
	routes := NewRoutes(
		hub,
		tokenVerifierStub{err: errors.New("invalid token")},
		videoReaderStub{video: playableVideo(7)},
	)
	server := newRealtimeTestServer(t, routes)
	connection := dialRealtime(t, server.URL, "/api/v1/course-videos/7/danmakus/realtime")
	defer connection.Close()

	writeAuthenticationFrame(t, connection, "request-2", "bad-token")
	frame := readServerFrame(t, connection)
	if frame.GetRequestId() != "request-2" || frame.GetError() == nil ||
		frame.GetError().GetCode() != danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_UNAUTHENTICATED {
		t.Fatalf("authentication error frame = %v", frame)
	}
	if counts := hub.connections.Counts(); len(counts) != 0 {
		t.Fatalf("connection counts = %v, want empty", counts)
	}
}

func TestConnectRejectsTextAuthenticationFrame(t *testing.T) {
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })
	routes := NewRoutes(
		hub,
		tokenVerifierStub{studentID: 1001},
		videoReaderStub{video: playableVideo(7)},
	)
	server := newRealtimeTestServer(t, routes)
	connection := dialRealtime(t, server.URL, "/api/v1/course-videos/7/danmakus/realtime")
	defer connection.Close()

	if err := connection.WriteMessage(websocket.TextMessage, []byte(`{"token":"bad-shape"}`)); err != nil {
		t.Fatal(err)
	}
	frame := readServerFrame(t, connection)
	if frame.GetError() == nil ||
		frame.GetError().GetCode() != danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_INVALID_MESSAGE {
		t.Fatalf("invalid message frame = %v", frame)
	}
}

func TestConnectRejectsUnplayableVideo(t *testing.T) {
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })
	video := playableVideo(7)
	video.Status = "uploading"
	routes := NewRoutes(
		hub,
		tokenVerifierStub{studentID: 1001},
		videoReaderStub{video: video},
	)
	server := newRealtimeTestServer(t, routes)
	connection := dialRealtime(t, server.URL, "/api/v1/course-videos/7/danmakus/realtime")
	defer connection.Close()

	writeAuthenticationFrame(t, connection, "request-3", "student-token")
	frame := readServerFrame(t, connection)
	if frame.GetError() == nil ||
		frame.GetError().GetCode() != danmakuv1.RealtimeErrorCode_REALTIME_ERROR_CODE_VIDEO_NOT_PLAYABLE {
		t.Fatalf("video error frame = %v", frame)
	}
}

func TestConnectRejectsInvalidVideoBeforeUpgrade(t *testing.T) {
	routes := NewRoutes(nil, nil, nil)
	server := newRealtimeTestServer(t, routes)
	webSocketURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/api/v1/course-videos/0/danmakus/realtime"
	connection, response, err := websocket.DefaultDialer.Dial(webSocketURL, nil)
	if connection != nil {
		connection.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusBadRequest {
		t.Fatalf("Dial() = response %v, error %v; want HTTP 400", response, err)
	}
	response.Body.Close()
}

func TestCheckWebSocketOrigin(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "missing origin", host: "courseforge.example", want: true},
		{name: "same host", host: "courseforge.example", origin: "https://courseforge.example", want: true},
		{name: "loopback development", host: "127.0.0.1:8080", origin: "http://localhost:5173", want: true},
		{name: "foreign host", host: "courseforge.example", origin: "https://evil.example", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://"+test.host+"/realtime", nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if got := checkWebSocketOrigin(request); got != test.want {
				t.Fatalf("checkWebSocketOrigin() = %v, want %v", got, test.want)
			}
		})
	}
}

func playableVideo(videoID uint64) *danmaku.VideoTarget {
	return &danmaku.VideoTarget{
		ID: videoID, Kind: danmaku.VideoKindPreview, Status: danmaku.VideoStatusReady,
	}
}

func newRealtimeTestServer(t *testing.T, routes *Routes) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	routes.RegisterAPIRoutes(engine.Group("/api/v1"))
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	return server
}

func dialRealtime(t *testing.T, serverURL, path string) *websocket.Conn {
	t.Helper()
	webSocketURL := "ws" + strings.TrimPrefix(serverURL, "http") + path
	connection, response, err := websocket.DefaultDialer.Dial(webSocketURL, nil)
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		t.Fatalf("dial realtime endpoint: %v", err)
	}
	return connection
}

func writeAuthenticationFrame(
	t *testing.T,
	connection *websocket.Conn,
	requestID string,
	token string,
) {
	t.Helper()
	payload, err := proto.Marshal(&danmakuv1.ClientFrame{
		RequestId: requestID,
		Payload: &danmakuv1.ClientFrame_Authenticate{Authenticate: &danmakuv1.AuthenticateRequest{
			AccessToken: token,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatalf("write authentication frame: %v", err)
	}
}

func readServerFrame(t *testing.T, connection *websocket.Conn) *danmakuv1.ServerFrame {
	t.Helper()
	if err := connection.SetReadDeadline(time.Now().Add(hubTestTimeout)); err != nil {
		t.Fatal(err)
	}
	messageType, payload, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("read server frame: %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("message type = %d, want BinaryMessage", messageType)
	}
	var frame danmakuv1.ServerFrame
	if err := proto.Unmarshal(payload, &frame); err != nil {
		t.Fatalf("unmarshal server frame: %v", err)
	}
	return &frame
}
