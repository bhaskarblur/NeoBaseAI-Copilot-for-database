package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"neobase-ai/config"
)

// AESGCMCrypto provides AES-GCM encryption and decryption
type AESGCMCrypto struct {
	key []byte
}

// NewAESGCMCrypto creates a new AES-GCM crypto instance
func NewAESGCMCrypto(key string) (*AESGCMCrypto, error) {
	// Validate key length (AES-GCM supports 16, 24, or 32 bytes)
	keyBytes := []byte(key)
	keyLen := len(keyBytes)
	
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, fmt.Errorf("invalid key length: %d bytes. AES-GCM requires 16, 24, or 32 bytes", keyLen)
	}
	
	return &AESGCMCrypto{
		key: keyBytes,
	}, nil
}

// NewFromConfig creates a new AES-GCM crypto instance from config
func NewFromConfig() (*AESGCMCrypto, error) {
	return NewAESGCMCrypto(config.Env.SpreadsheetDataEncryptionKey)
}

// Encrypt encrypts plaintext using AES-GCM
func (c *AESGCMCrypto) Encrypt(plaintext string) (string, error) {
	// Create cipher block
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 for storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-GCM
func (c *AESGCMCrypto) Decrypt(ciphertext string) (string, error) {
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextData := data[:nonceSize], data[nonceSize:]

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EncryptBytes encrypts byte array using AES-GCM
func (c *AESGCMCrypto) EncryptBytes(plaintext []byte) ([]byte, error) {
	// Create cipher block
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data (prepend nonce to ciphertext)
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptBytes decrypts byte array using AES-GCM
func (c *AESGCMCrypto) DecryptBytes(ciphertext []byte) ([]byte, error) {
	// Create cipher block
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextData := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt data
	return gcm.Open(nil, nonce, ciphertextData, nil)
}

// EncryptField encrypts a database field value
// Returns the encrypted value with a prefix to identify encrypted fields
func (c *AESGCMCrypto) EncryptField(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	encrypted, err := c.Encrypt(value)
	if err != nil {
		return "", err
	}

	// Add prefix to identify encrypted fields
	return "ENC:" + encrypted, nil
}

// DecryptField decrypts a database field value
// Checks for the encryption prefix before attempting decryption
func (c *AESGCMCrypto) DecryptField(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	// Check if field is encrypted
	if len(value) < 4 || value[:4] != "ENC:" {
		// Not encrypted, return as-is
		return value, nil
	}

	// Remove prefix and decrypt
	return c.Decrypt(value[4:])
}

// IsEncrypted checks if a field value is encrypted
func (c *AESGCMCrypto) IsEncrypted(value string) bool {
	return len(value) >= 4 && value[:4] == "ENC:"
}