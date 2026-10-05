package registration

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
)

type WeChatEvent struct {
	ToUserName   string
	FromUserName string
	CreateTime   int64
	MsgType      string
	Event        string
	EventKey     string
	Ticket       string
}

func WeChatSignature(token, timestamp, nonce, encrypted string) string {
	values := []string{token, timestamp, nonce, encrypted}
	sort.Strings(values)
	sum := sha1.Sum([]byte(strings.Join(values, "")))
	return hex.EncodeToString(sum[:])
}

// VerifyWeChatURL is the Official Account setup handshake. Its signature does
// not cover a message body, so it is never used to authenticate follow events.
func VerifyWeChatURL(s domain.RegistrationSettings, query url.Values, now time.Time) (string, error) {
	timestamp, err := strconv.ParseInt(query.Get("timestamp"), 10, 64)
	echo := query.Get("echostr")
	if err != nil || timestamp < now.Unix()-300 || timestamp > now.Unix()+300 || s.VerificationToken == "" || query.Get("nonce") == "" || len(query.Get("nonce")) > 128 || echo == "" || len(echo) > 1024 {
		return "", domain.ErrUnauthenticated
	}
	expected := WeChatSignature(s.VerificationToken, query.Get("timestamp"), query.Get("nonce"), "")
	if subtle.ConstantTimeCompare([]byte(expected), []byte(query.Get("signature"))) != 1 {
		return "", domain.ErrUnauthenticated
	}
	return echo, nil
}

// Only safe-mode callbacks are accepted; a signed plaintext message does not
// authenticate its body. WeChat's encrypted envelope authenticates the event.
func DecryptWeChat(s domain.RegistrationSettings, query url.Values, encrypted string, now time.Time) ([]byte, error) {
	timestamp, err := strconv.ParseInt(query.Get("timestamp"), 10, 64)
	if err != nil || query.Get("nonce") == "" || timestamp < now.Unix()-300 || timestamp > now.Unix()+300 || encrypted == "" {
		return nil, domain.ErrUnauthenticated
	}
	expected := WeChatSignature(s.VerificationToken, query.Get("timestamp"), query.Get("nonce"), encrypted)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(query.Get("msg_signature"))) != 1 {
		return nil, domain.ErrUnauthenticated
	}
	key, err := base64.StdEncoding.DecodeString(s.EncodingAESKey + "=")
	if err != nil || len(key) != 32 {
		return nil, domain.ErrUnauthenticated
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil || len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 || len(ciphertext) > 64<<10 {
		return nil, domain.ErrUnauthenticated
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, domain.ErrUnauthenticated
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plain, ciphertext)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > 32 || padding > len(plain) {
		return nil, domain.ErrUnauthenticated
	}
	for _, b := range plain[len(plain)-padding:] {
		if int(b) != padding {
			return nil, domain.ErrUnauthenticated
		}
	}
	plain = plain[:len(plain)-padding]
	if len(plain) < 20 {
		return nil, domain.ErrUnauthenticated
	}
	length := int(binary.BigEndian.Uint32(plain[16:20]))
	if length > len(plain)-20 {
		return nil, domain.ErrUnauthenticated
	}
	if string(plain[20+length:]) != s.AppID {
		return nil, domain.ErrUnauthenticated
	}
	return plain[20 : 20+length], nil
}
func ParseWeChatEvent(s domain.RegistrationSettings, plain []byte, now time.Time) (WeChatEvent, string, error) {
	var event WeChatEvent
	if xml.Unmarshal(plain, &event) != nil || event.ToUserName != s.OfficialAccountID || event.FromUserName == "" || event.MsgType != "event" || event.CreateTime < now.Unix()-300 || event.CreateTime > now.Unix()+300 {
		return event, "", domain.ErrUnauthenticated
	}
	scene := event.EventKey
	switch event.Event {
	case "subscribe":
		if !strings.HasPrefix(scene, "qrscene_") {
			return event, "", nil
		}
		scene = strings.TrimPrefix(scene, "qrscene_")
	case "SCAN":
	default:
		return event, "", nil
	}
	if scene == "" || event.Ticket == "" {
		return event, "", domain.ErrUnauthenticated
	}
	return event, scene, nil
}
