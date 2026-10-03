package messagechannel

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Endpoint approvals come from Administrator YAML, never from inbound messages.
type ApprovedEndpoint struct {
	URL                 string
	AllowPrivateNetwork bool
}

func validEndpoint(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host != "" && !strings.Contains(u.Host, "*") && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.RawPath == "" && !strings.HasSuffix(value, "/") && !strings.Contains(u.Path, "..") && !strings.ContainsAny(value, "\r\n\x00")
}
func (h *HTTP) approve(endpoints []ApprovedEndpoint, transport http.RoundTripper) {
	h.approved = map[string]*http.Client{}
	for _, endpoint := range endpoints {
		if !validEndpoint(endpoint.URL) {
			continue
		}
		rt := transport
		if rt == nil {
			clone := http.DefaultTransport.(*http.Transport).Clone()
			clone.Proxy = nil // Explicit endpoints cannot be redirected through environment proxies.
			clone.DialContext = approvedDialer(endpoint.AllowPrivateNetwork)
			rt = clone
		}
		h.approved[endpoint.URL] = &http.Client{Transport: rt, Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
}
func approvedAddress(ip netip.Addr, allowPrivate bool) bool {
	ip = ip.Unmap()
	return publicProviderAddress(ip) || allowPrivate && (ip.IsPrivate() || ip.IsLoopback())
}
func approvedDialer(allowPrivate bool) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, providerError("provider_endpoint_invalid")
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(addresses) == 0 {
			return nil, providerError("provider_network_error")
		}
		for _, ip := range addresses {
			if !approvedAddress(ip, allowPrivate) {
				return nil, providerError("provider_endpoint_invalid")
			}
		}
		dialer := net.Dialer{Timeout: 10 * time.Second}
		for _, ip := range addresses {
			if connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port)); err == nil {
				return connection, nil
			}
		}
		return nil, providerError("provider_network_error")
	}
}
func (h *HTTP) endpoint(base, path string) (string, error) {
	if h.approved[base] == nil || !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return "", providerError("provider_endpoint_not_approved")
	}
	return base + path, nil
}
func (h *HTTP) clientFor(target *url.URL) (*http.Client, error) {
	fixed := map[string]bool{"api.telegram.org": true, "slack.com": true, "discord.com": true, "api.dingtalk.com": true, "oapi.dingtalk.com": true, "open.feishu.cn": true, "open.larksuite.com": true, "graph.facebook.com": true, "bots.qq.com": true, "q.qq.com": true, "api.sgroup.qq.com": true, "ilinkai.weixin.qq.com": true, "bot.yuanbao.tencent.com": true}
	_, wechatErr := wechatAPIBase("https://" + target.Host)
	if fixed[target.Host] || wechatErr == nil {
		return h.client, nil
	}
	for base, client := range h.approved {
		u, _ := url.Parse(base)
		if u.Host == target.Host && (u.Path == "" || target.Path == u.Path || strings.HasPrefix(target.Path, u.Path+"/")) {
			return client, nil
		}
	}
	return nil, providerError("provider_endpoint_invalid")
}
