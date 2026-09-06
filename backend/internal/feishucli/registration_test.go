package feishucli

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRegistrarBeginsRegistrationWithoutExposingDeviceCodeInURL(t *testing.T) {
	client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(request.Body)
		if request.URL.String() != "https://accounts.example.test/oauth/v1/app/registration" || !strings.Contains(string(data), "action=begin") {
			t.Fatalf("request = %s body=%q", request.URL, data)
		}
		return jsonResponse(`{"device_code":"device-secret","user_code":"visible-code","expire_in":3600,"interval":5}`), nil
	})
	now := time.Unix(100, 0).UTC()
	registrar := &Registrar{client: client, accountsBase: "https://accounts.example.test", openBaseURL: "https://open.example.test", now: func() time.Time { return now }}
	registration, err := registrar.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if registration.DeviceCode != "device-secret" || strings.Contains(registration.ActionURL, registration.DeviceCode) || !strings.Contains(registration.ActionURL, "user_code=visible-code") || !registration.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("registration = %#v", registration)
	}
}

func TestRegistrarPollsPendingAndCompleteRegistration(t *testing.T) {
	responses := []string{`{"error":"authorization_pending"}`, `{"client_id":"cli-app","client_secret":"app-secret"}`}
	registrar := &Registrar{client: roundTripFunc(func(*http.Request) (*http.Response, error) {
		response := responses[0]
		responses = responses[1:]
		return jsonResponse(response), nil
	}), accountsBase: "https://accounts.example.test"}
	if _, err := registrar.Poll(context.Background(), "device-secret"); err != ErrPending {
		t.Fatalf("pending error = %v", err)
	}
	application, err := registrar.Poll(context.Background(), "device-secret")
	if err != nil || application.AppID != "cli-app" || application.AppSecret != "app-secret" {
		t.Fatalf("application=%#v error=%v", application, err)
	}
}

func TestRegistrarAcceptsStructuredPendingResponseWithHTTP400(t *testing.T) {
	registrar := &Registrar{client: roundTripFunc(func(*http.Request) (*http.Response, error) {
		response := jsonResponse(`{"error":"authorization_pending"}`)
		response.StatusCode = http.StatusBadRequest
		return response, nil
	}), accountsBase: "https://accounts.example.test"}
	if _, err := registrar.Poll(context.Background(), "device-secret"); err != ErrPending {
		t.Fatalf("pending error = %v", err)
	}
}

func TestRegistrarRealRegistrationStartsPending(t *testing.T) {
	if os.Getenv("FEISHU_CLI_REAL_TEST") != "1" {
		t.Skip("set FEISHU_CLI_REAL_TEST=1 to call the Feishu application registration endpoint")
	}
	registrar := NewRegistrar(nil)
	registration, err := registrar.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if registration.ActionURL == "" || registration.DeviceCode == "" || !registration.ExpiresAt.After(time.Now()) {
		t.Fatal("Feishu returned an incomplete application registration")
	}
	if _, err := registrar.Poll(context.Background(), registration.DeviceCode); err != ErrPending {
		t.Fatalf("initial registration state = %v, want pending", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
}
