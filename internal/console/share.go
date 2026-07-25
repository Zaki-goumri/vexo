package console

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const shareSecretLen = 32

type ShareSigner struct {
	secret []byte
}

func NewShareSigner(secret []byte) *ShareSigner {
	return &ShareSigner{secret: secret}
}

func LoadOrCreateShareSecret(dataDir string) ([]byte, error) {
	keyFile := filepath.Join(dataDir, ".vexo.share.key")

	data, err := os.ReadFile(keyFile)
	if err == nil {
		secret, decodeErr := hex.DecodeString(string(data))
		if decodeErr != nil {
			return nil, fmt.Errorf("decode share secret: %w", decodeErr)
		}
		return secret, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read share secret: %w", err)
	}

	secret := make([]byte, shareSecretLen)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate share secret: %w", err)
	}
	if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(secret)), 0o600); err != nil {
		return nil, fmt.Errorf("write share secret: %w", err)
	}
	return secret, nil
}

func (s *ShareSigner) payload(bucket, key string, expiresAt int64) []byte {
	return []byte(bucket + "\n" + key + "\n" + strconv.FormatInt(expiresAt, 10))
}

func (s *ShareSigner) Sign(bucket, key string, expiresAt int64) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(s.payload(bucket, key, expiresAt))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *ShareSigner) Verify(bucket, key string, expiresAt int64, sig string) bool {
	expected := s.Sign(bucket, key, expiresAt)
	return hmac.Equal([]byte(expected), []byte(sig))
}
