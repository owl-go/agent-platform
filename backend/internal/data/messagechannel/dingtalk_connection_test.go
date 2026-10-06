package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

func dingTalkConnectionFixture(t *testing.T) *DingTalk {
	t.Helper()
	return &DingTalk{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			ID            string                         `json:"clientId"`
			Secret        string                         `json:"clientSecret"`
			Subscriptions []struct{ Type, Topic string } `json:"subscriptions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || r.URL.Path != "/v1.0/gateway/connections/open" || body.ID != "app" || body.Secret != "secret" || len(body.Subscriptions) != 1 || body.Subscriptions[0].Type != "CALLBACK" || body.Subscriptions[0].Topic != payload.BotMessageCallbackTopic {
			t.Fatal("connection did not authenticate and subscribe to bot messages")
		}
		return jsonResponse(`{"endpoint":"wss://wss-open-connection.dingtalk.com/connect","ticket":"protected-ticket"}`, 200), nil
	}))}
}

func TestDingTalkStreamHandshakeReportsHealthBeforeReceivingStaffID(t *testing.T) {
	a := dingTalkConnectionFixture(t)
	socket := newFakeSocket()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connected, received, acknowledged := false, false, false
	event := map[string]any{"robotCode": "app", "chatbotCorpId": "corp", "senderCorpId": "corp", "chatbotUserId": "bot", "senderId": "encrypted-sender", "senderStaffId": "staff", "conversationId": "direct-chat", "conversationType": "1", "msgId": "pair-event", "msgtype": "text", "text": map[string]string{"content": "pair exact-code"}, "sessionWebhook": "https://oapi.dingtalk.com/robot/sendBySession?session=protected", "createAt": time.Now().UnixMilli()}
	body, _ := json.Marshal(event)
	frame := &payload.DataFrame{Type: "CALLBACK", Headers: payload.DataFrameHeader{payload.DataFrameHeaderKTopic: payload.BotMessageCallbackTopic, payload.DataFrameHeaderKMessageId: "frame"}, Data: string(body)}
	data, _ := json.Marshal(frame)
	socket.onWrite = func(data []byte) {
		var ack payload.DataFrameResponse
		if json.Unmarshal(data, &ack) != nil || !received || ack.Headers[payload.DataFrameHeaderKMessageId] != "frame" || ack.Code != payload.DataFrameResponseStatusCodeKOK {
			t.Error("callback receipt preceded sender processing or lost correlation")
		}
		acknowledged = true
		cancel()
	}
	err := a.connect(ctx, application.ChannelStored{Channel: domain.MessageChannel{AccountID: "app"}}, application.ChannelCredentials{"client_id": "app", "client_secret": "secret"}, func(_ context.Context, m domain.ChannelMessage) error {
		if !connected || m.SenderID != "staff" || m.TenantID != "corp" || m.Group || m.Text != "pair exact-code" {
			t.Error("Staff ID was received before handshake or from the wrong identity")
		}
		received = true
		return nil
	}, func(_ context.Context, health string) error {
		if health != "connected" {
			t.Error("unexpected connection health")
		}
		connected = true
		socket.incoming <- data
		return nil
	}, func(_ context.Context, target string, _ http.Header) (channelSocket, error) {
		u, _ := url.Parse(target)
		if u.Hostname() != "wss-open-connection.dingtalk.com" || u.Query().Get("ticket") != "protected-ticket" || connected {
			t.Error("health was reported before ticket-authenticated handshake")
		}
		return socket, nil
	})
	if !errors.Is(err, context.Canceled) || !connected || !received || !acknowledged {
		t.Fatalf("stream result: connected=%v received=%v ack=%v err=%v", connected, received, acknowledged, err)
	}
	select {
	case <-socket.closed:
	default:
		t.Fatal("cancelled stream socket leaked")
	}
}

func TestDingTalkStreamDoesNotReportConnectedOnFailedHandshake(t *testing.T) {
	a := dingTalkConnectionFixture(t)
	err := a.connect(context.Background(), application.ChannelStored{}, application.ChannelCredentials{"client_id": "app", "client_secret": "secret"}, nil, func(context.Context, string) error {
		t.Fatal("failed handshake enabled pairing")
		return nil
	}, func(context.Context, string, http.Header) (channelSocket, error) {
		return nil, errors.New("private-network-detail")
	})
	if err == nil {
		t.Fatal("failed stream handshake accepted")
	}
	if _, ok := NewTransports(nil)["dingtalk"].StreamReceiver.(application.ChannelStreamHealthReceiver); !ok {
		t.Fatal("DingTalk pairing health receiver not registered")
	}
}
