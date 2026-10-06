package registration

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
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

type WeChatMessage struct {
	ToUserName   string
	FromUserName string
	CreateTime   int64
	MsgType      string
	Event        string
	Content      string
	MsgID        uint64 `xml:"MsgId"`
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
func ParseWeChatMessage(s domain.RegistrationSettings, plain []byte, now time.Time) (WeChatMessage, error) {
	var message WeChatMessage
	if xml.Unmarshal(plain, &message) != nil || message.ToUserName != s.OfficialAccountID || message.FromUserName == "" || len(message.FromUserName) > 256 || message.MsgType == "" || message.CreateTime < now.Unix()-300 || message.CreateTime > now.Unix()+300 {
		return message, domain.ErrUnauthenticated
	}
	return message, nil
}

// Passive replies use the same safe-mode envelope as incoming messages and
// require no customer-service or parameterized QR API permission.
func WeChatTextReply(s domain.RegistrationSettings, incoming WeChatMessage, content string, now time.Time) ([]byte, error) {
	reply := struct {
		XMLName      xml.Name `xml:"xml"`
		ToUserName   string
		FromUserName string
		CreateTime   int64
		MsgType      string
		Content      string
	}{ToUserName: incoming.FromUserName, FromUserName: s.OfficialAccountID, CreateTime: now.Unix(), MsgType: "text", Content: content}
	body, err := xml.Marshal(reply)
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(s.EncodingAESKey + "=")
	if err != nil || len(key) != 32 {
		return nil, domain.ErrUnauthenticated
	}
	plain := make([]byte, 20, 20+len(body)+len(s.AppID)+32)
	if _, err = rand.Read(plain[:16]); err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint32(plain[16:20], uint32(len(body)))
	plain = append(plain, body...)
	plain = append(plain, s.AppID...)
	padding := 32 - len(plain)%32
	for range padding {
		plain = append(plain, byte(padding))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ciphertext, plain)
	encrypted := base64.StdEncoding.EncodeToString(ciphertext)
	nonceBytes := make([]byte, 16)
	if _, err = rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	envelope := struct {
		XMLName      xml.Name `xml:"xml"`
		Encrypt      string
		MsgSignature string
		TimeStamp    int64
		Nonce        string
	}{Encrypt: encrypted, MsgSignature: WeChatSignature(s.VerificationToken, timestamp, nonce, encrypted), TimeStamp: now.Unix(), Nonce: nonce}
	return xml.Marshal(envelope)
}
