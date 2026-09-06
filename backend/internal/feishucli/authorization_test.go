package feishucli

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRegistrarBeginsUserAuthorization(t *testing.T) {
	client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://accounts.example.test/oauth/v1/device_authorization" {
			t.Fatalf("URL = %s", request.URL)
		}
		if got := request.Header.Get("Authorization"); got != "Basic "+base64.StdEncoding.EncodeToString([]byte("app:secret")) {
			t.Fatalf("authorization header = %q", got)
		}
		data, _ := io.ReadAll(request.Body)
		values, _ := url.ParseQuery(string(data))
		if values.Get("client_id") != "app" || values.Get("scope") != "calendar:calendar:read offline_access" {
			t.Fatalf("form = %q", data)
		}
		return jsonResponse(`{"device_code":"device-secret","verification_uri_complete":"https://open.example.test/authorize","expires_in":240}`), nil
	})
	now := time.Unix(100, 0).UTC()
	registrar := &Registrar{client: client, accountsBase: "https://accounts.example.test", openBaseURL: "https://open.example.test", now: func() time.Time { return now }}
	result, err := registrar.BeginAuthorization(context.Background(), "app", "secret", []string{"calendar:calendar:read"})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeviceCode != "device-secret" || result.ActionURL != "https://open.example.test/authorize" || !result.ExpiresAt.Equal(now.Add(240*time.Second)) {
		t.Fatalf("authorization request = %#v", result)
	}
}

func TestRegistrarPollsAuthorizationAndLoadsUser(t *testing.T) {
	requests := 0
	client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			if request.URL.String() != "https://open.example.test/open-apis/authen/v2/oauth/token" {
				t.Fatalf("token URL = %s", request.URL)
			}
			return jsonResponse(`{"access_token":"access-secret","refresh_token":"refresh-secret","expires_in":7200,"scope":"im:message offline_access"}`), nil
		}
		if request.URL.String() != "https://open.example.test/open-apis/authen/v1/user_info" || request.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("user info request = %s %#v", request.URL, request.Header)
		}
		return jsonResponse(`{"code":0,"data":{"open_id":"ou_user","name":"Tester"}}`), nil
	})
	now := time.Unix(100, 0).UTC()
	registrar := &Registrar{client: client, accountsBase: "https://accounts.example.test", openBaseURL: "https://open.example.test", now: func() time.Time { return now }}
	result, err := registrar.PollAuthorization(context.Background(), "app", "secret", "device-secret")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExternalID != "ou_user" || result.DisplayName != "Tester" || result.AccessToken != "access-secret" || result.RefreshToken != "refresh-secret" || !result.ExpiresAt.Equal(now.Add(2*time.Hour)) || strings.Join(result.Scopes, " ") != "im:message offline_access" {
		t.Fatalf("authorization = %#v", result)
	}
}

func TestRegistrarPollAuthorizationPreservesPending(t *testing.T) {
	response := jsonResponse(`{"error":"authorization_pending"}`)
	response.StatusCode = http.StatusBadRequest
	registrar := &Registrar{client: roundTripFunc(func(*http.Request) (*http.Response, error) { return response, nil }), openBaseURL: "https://open.example.test"}
	if _, err := registrar.PollAuthorization(context.Background(), "app", "secret", "device"); err != ErrPending {
		t.Fatalf("error = %v", err)
	}
}
