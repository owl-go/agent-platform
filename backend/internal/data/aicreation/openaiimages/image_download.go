package openaiimages

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"

	"agent-platform/backend/internal/biz/aicreation/application"
)

const maxImageBytes = 25 * 1024 * 1024

func newPublicImageClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialPublicAddress
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many image redirects")
			}
			return validateRemoteImageURL(request.URL)
		},
	}
}

func dialPublicAddress(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("parse image address: %w", err)
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve image host: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("image host resolved without addresses")
	}
	for _, address := range addresses {
		if !isPublicAddress(address) {
			return nil, fmt.Errorf("image host resolved to a non-public address")
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
}

func isPublicAddress(address netip.Addr) bool {
	return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() &&
		!address.IsLinkLocalUnicast() && !address.IsUnspecified() && !address.IsMulticast()
}

func validateRemoteImageURL(parsed *url.URL) error {
	if parsed == nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("provider returned an untrusted image URL")
	}
	if address, err := netip.ParseAddr(parsed.Hostname()); err == nil && !isPublicAddress(address) {
		return fmt.Errorf("provider returned a non-public image URL")
	}
	return nil
}

func (provider *Provider) downloadImageURL(ctx context.Context, value string) ([]byte, error) {
	parsed, err := url.Parse(value)
	if err != nil || validateRemoteImageURL(parsed) != nil {
		return nil, &application.ProviderFailure{Code: "image_output_invalid", Cause: fmt.Errorf("Images API returned an untrusted image URL")}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, value, nil)
	if err != nil {
		return nil, &application.ProviderFailure{Code: "image_output_invalid", Cause: err}
	}
	response, err := provider.imageClient.Do(request)
	if err != nil {
		return nil, &application.ProviderFailure{Code: "image_outcome_unknown", OutcomeUnknown: true, Cause: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &application.ProviderFailure{Code: "image_provider_unavailable", Cause: fmt.Errorf("image download returned status %d", response.StatusCode)}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxImageBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxImageBytes {
		return nil, &application.ProviderFailure{Code: "image_output_invalid", Cause: fmt.Errorf("image download is invalid")}
	}
	return data, nil
}
