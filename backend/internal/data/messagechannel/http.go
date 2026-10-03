// Package messagechannel implements provider transports; it never executes Runs.
package messagechannel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

type HTTP struct{ client *http.Client }

func NewHTTP(transport http.RoundTripper) *HTTP {
	if transport == nil {
		secured := http.DefaultTransport.(*http.Transport).Clone()
		secured.DialContext = dialPublicProvider
		transport = secured
	}
	return &HTTP{client: &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func dialPublicProvider(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, providerError("provider_endpoint_invalid")
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, providerError("provider_network_error")
	}
	for _, ip := range addresses {
		if !publicProviderAddress(ip) {
			return nil, providerError("provider_endpoint_invalid")
		}
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	for _, ip := range addresses {
		connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
	}
	return nil, providerError("provider_network_error")
}
func publicProviderAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, reserved := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(reserved).Contains(ip) {
			return false
		}
	}
	return true
}
func NewAdapters(transport http.RoundTripper) map[string]application.ChannelAdapter {
	h := NewHTTP(transport)
	return map[string]application.ChannelAdapter{"telegram": &Telegram{h}, "slack": &Slack{h}, "discord": &Discord{h}, "dingtalk": &DingTalk{h}, "feishu": &Feishu{h}}
}
func providerError(code string) error {
	if code == "callback_authentication_failed" {
		return fmt.Errorf("%w: %w", domain.ErrInvalid, domain.ErrChannelCallbackAuthentication)
	}
	return fmt.Errorf("%w: %s", domain.ErrInvalid, code)
}
func (h *HTTP) request(ctx context.Context, method, target, token string, body any) (map[string]json.RawMessage, int, time.Duration, error) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return nil, 0, 0, providerError("provider_endpoint_invalid")
	}
	allowed := map[string]bool{"api.telegram.org": true, "slack.com": true, "discord.com": true, "api.dingtalk.com": true, "oapi.dingtalk.com": true, "open.feishu.cn": true, "open.larksuite.com": true}
	if !allowed[u.Host] {
		return nil, 0, 0, providerError("provider_endpoint_invalid")
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, 0, 0, providerError("provider_payload_invalid")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, 0, providerError("provider_request_invalid")
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	response, err := h.client.Do(req)
	if err != nil {
		return nil, 0, 0, providerError("provider_network_error")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 256*1024+1))
	if err != nil || len(data) > 256*1024 {
		return nil, response.StatusCode, 0, providerError("provider_response_invalid")
	}
	retry := time.Second
	if seconds, err := strconv.ParseFloat(response.Header.Get("Retry-After"), 64); err == nil {
		retry = time.Duration(seconds * float64(time.Second))
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(data, &result) != nil {
		return nil, response.StatusCode, retry, providerError("provider_response_invalid")
	}
	return result, response.StatusCode, retry, nil
}
func rawString(value json.RawMessage) string { var s string; _ = json.Unmarshal(value, &s); return s }
func rawBool(value json.RawMessage) bool     { var b bool; _ = json.Unmarshal(value, &b); return b }
func rawNumber(value json.RawMessage) int64  { var n int64; _ = json.Unmarshal(value, &n); return n }
func failedSend(status int, retry time.Duration, err error) application.ChannelSendResult {
	if status == 429 {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_rate_limited", RetryAfter: retry}
	}
	if status >= 400 && status < 500 {
		return application.ChannelSendResult{State: "failed", Code: "provider_rejected"}
	}
	return application.ChannelSendResult{State: "outcome_unknown", Code: "provider_send_unconfirmed"}
}

func noCallback(context.Context, application.ChannelStored, application.ChannelCredentials, http.Header, []byte) (application.ChannelCallback, error) {
	return application.ChannelCallback{}, providerError("callback_unsupported")
}
