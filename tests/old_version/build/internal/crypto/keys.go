package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"
)

// GenerateKeyPair creates a new Ed25519 key pair
func GenerateKeyPair() (privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey, err error) {
	publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
	return
}

// GenerateToken creates a signed token with timestamp and nonce
// Token format: base64(signature(64) | timestamp(8) | nonce(16) | username)
func GenerateToken(privateKey ed25519.PrivateKey, username string) (string, error) {
	timestamp := make([]byte, 8)
	binary.BigEndian.PutUint64(timestamp, uint64(time.Now().Unix()))

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	// Message to sign: username | timestamp | nonce
	message := append([]byte(username), timestamp...)
	message = append(message, nonce...)

	signature := ed25519.Sign(privateKey, message)

	// Token: signature | timestamp | nonce
	token := append(signature, timestamp...)
	token = append(token, nonce...)

	return base64.StdEncoding.EncodeToString(token), nil
}

// VerifyToken verifies a token against a public key and username
func VerifyToken(publicKeyBase64, tokenBase64, username string) (bool, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return false, err
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return false, errors.New("invalid public key size")
	}

	publicKey := ed25519.PublicKey(publicKeyBytes)

	tokenBytes, err := base64.StdEncoding.DecodeString(tokenBase64)
	if err != nil {
		return false, err
	}

	// Token format: signature(64) | timestamp(8) | nonce(16)
	if len(tokenBytes) < 64+8+16 {
		return false, errors.New("invalid token size")
	}

	signature := tokenBytes[:64]
	timestamp := tokenBytes[64:72]
	nonce := tokenBytes[72:88]

	// Reconstruct message: username | timestamp | nonce
	message := append([]byte(username), timestamp...)
	message = append(message, nonce...)

	return ed25519.Verify(publicKey, message, signature), nil
}

// PublicKeyToBase64 encodes a public key to base64
func PublicKeyToBase64(publicKey ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(publicKey)
}

// PrivateKeyToBytes converts private key to bytes for storage
func PrivateKeyToBytes(privateKey ed25519.PrivateKey) []byte {
	return privateKey
}

// BytesToPrivateKey converts bytes back to private key
func BytesToPrivateKey(data []byte) ed25519.PrivateKey {
	return ed25519.PrivateKey(data)
}
