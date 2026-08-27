package secretcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

type Box struct{ aead cipher.AEAD }

func New(base64Key string) (*Box, error) {
	key, err := base64.RawStdEncoding.DecodeString(base64Key)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(base64Key)
	}
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("data encryption key must be base64-encoded 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create data cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create data AEAD: %w", err)
	}
	return &Box{aead: aead}, nil
}

func (box *Box) Encrypt(plaintext []byte, context string) ([]byte, error) {
	if box == nil || box.aead == nil {
		return nil, fmt.Errorf("data encryption is unavailable")
	}
	nonce := make([]byte, box.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate encryption nonce: %w", err)
	}
	return box.aead.Seal(nonce, nonce, plaintext, []byte(context)), nil
}

func (box *Box) Decrypt(ciphertext []byte, context string) ([]byte, error) {
	if box == nil || box.aead == nil || len(ciphertext) < box.aead.NonceSize() {
		return nil, fmt.Errorf("encrypted data is invalid")
	}
	nonce := ciphertext[:box.aead.NonceSize()]
	plaintext, err := box.aead.Open(nil, nonce, ciphertext[box.aead.NonceSize():], []byte(context))
	if err != nil {
		return nil, fmt.Errorf("decrypt data: %w", err)
	}
	return plaintext, nil
}
