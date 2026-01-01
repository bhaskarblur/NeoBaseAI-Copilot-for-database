package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"

	"neobase-ai/config"
	"neobase-ai/internal/models"
)

// EncryptConnection encrypts sensitive fields in a connection
func EncryptConnection(conn *models.Connection) error {
	key := []byte(config.Env.SchemaEncryptionKey)

	// Encrypt host
	if encryptedHost, err := encrypt(conn.Host, key); err == nil {
		conn.Host = encryptedHost
	} else {
		return fmt.Errorf("failed to encrypt host: %v", err)
	}

	// Encrypt port if present
	if conn.Port != nil {
		if encryptedPort, err := encrypt(*conn.Port, key); err == nil {
			*conn.Port = encryptedPort
		} else {
			return fmt.Errorf("failed to encrypt port: %v", err)
		}
	}

	// Encrypt username if present
	if conn.Username != nil {
		if encryptedUsername, err := encrypt(*conn.Username, key); err == nil {
			*conn.Username = encryptedUsername
		} else {
			return fmt.Errorf("failed to encrypt username: %v", err)
		}
	}

	// Encrypt password if present
	if conn.Password != nil {
		if encryptedPassword, err := encrypt(*conn.Password, key); err == nil {
			*conn.Password = encryptedPassword
		} else {
			return fmt.Errorf("failed to encrypt password: %v", err)
		}
	}

	// Encrypt database
	if encryptedDatabase, err := encrypt(conn.Database, key); err == nil {
		conn.Database = encryptedDatabase
	} else {
		return fmt.Errorf("failed to encrypt database: %v", err)
	}

	// Encrypt SSL certificate URLs if present
	if conn.SSLCertURL != nil {
		if encryptedURL, err := encrypt(*conn.SSLCertURL, key); err == nil {
			*conn.SSLCertURL = encryptedURL
		} else {
			return fmt.Errorf("failed to encrypt SSL certificate URL: %v", err)
		}
	}

	if conn.SSLKeyURL != nil {
		if encryptedURL, err := encrypt(*conn.SSLKeyURL, key); err == nil {
			*conn.SSLKeyURL = encryptedURL
		} else {
			return fmt.Errorf("failed to encrypt SSL key URL: %v", err)
		}
	}

	if conn.SSLRootCertURL != nil {
		if encryptedURL, err := encrypt(*conn.SSLRootCertURL, key); err == nil {
			*conn.SSLRootCertURL = encryptedURL
		} else {
			return fmt.Errorf("failed to encrypt SSL root certificate URL: %v", err)
		}
	}

	// Encrypt Google Sheets URL if present
	if conn.GoogleSheetURL != nil {
		if encryptedURL, err := encrypt(*conn.GoogleSheetURL, key); err == nil {
			*conn.GoogleSheetURL = encryptedURL
		} else {
			return fmt.Errorf("failed to encrypt Google Sheets URL: %v", err)
		}
	}

	// Encrypt Google auth tokens if present
	if conn.GoogleAuthToken != nil {
		if encryptedToken, err := encrypt(*conn.GoogleAuthToken, key); err == nil {
			*conn.GoogleAuthToken = encryptedToken
		} else {
			return fmt.Errorf("failed to encrypt Google auth token: %v", err)
		}
	}

	if conn.GoogleRefreshToken != nil {
		if encryptedToken, err := encrypt(*conn.GoogleRefreshToken, key); err == nil {
			*conn.GoogleRefreshToken = encryptedToken
		} else {
			return fmt.Errorf("failed to encrypt Google refresh token: %v", err)
		}
	}

	// Encrypt SSH authentication fields if present
	if conn.SSHPrivateKey != nil {
		if encryptedKey, err := encrypt(*conn.SSHPrivateKey, key); err == nil {
			*conn.SSHPrivateKey = encryptedKey
		} else {
			return fmt.Errorf("failed to encrypt SSH private key: %v", err)
		}
	}

	if conn.SSHPassword != nil {
		if encryptedPassword, err := encrypt(*conn.SSHPassword, key); err == nil {
			*conn.SSHPassword = encryptedPassword
		} else {
			return fmt.Errorf("failed to encrypt SSH password: %v", err)
		}
	}

	if conn.SSHPassphrase != nil {
		if encryptedPassphrase, err := encrypt(*conn.SSHPassphrase, key); err == nil {
			*conn.SSHPassphrase = encryptedPassphrase
		} else {
			return fmt.Errorf("failed to encrypt SSH passphrase: %v", err)
		}
	}

	if conn.SSHPrivateKeyURL != nil {
		if encryptedURL, err := encrypt(*conn.SSHPrivateKeyURL, key); err == nil {
			*conn.SSHPrivateKeyURL = encryptedURL
		} else {
			return fmt.Errorf("failed to encrypt SSH private key URL: %v", err)
		}
	}

	return nil
}

// DecryptConnection decrypts sensitive fields in a connection
// If decryption fails for any field, it returns the original value for backward compatibility
func DecryptConnection(conn *models.Connection) {
	key := []byte(config.Env.SchemaEncryptionKey)

	// Decrypt host
	if decryptedHost, err := decrypt(conn.Host, key); err == nil {
		conn.Host = decryptedHost
	} else {
		log.Printf("Warning: Failed to decrypt host, using as-is: %v", err)
	}

	// Decrypt port if present
	if conn.Port != nil {
		if decryptedPort, err := decrypt(*conn.Port, key); err == nil {
			*conn.Port = decryptedPort
		} else {
			log.Printf("Warning: Failed to decrypt port, using as-is: %v", err)
		}
	}

	// Decrypt username if present
	if conn.Username != nil {
		if decryptedUsername, err := decrypt(*conn.Username, key); err == nil {
			*conn.Username = decryptedUsername
		} else {
			log.Printf("Warning: Failed to decrypt username, using as-is: %v", err)
		}
	}

	// Decrypt password if present
	if conn.Password != nil {
		if decryptedPassword, err := decrypt(*conn.Password, key); err == nil {
			*conn.Password = decryptedPassword
		} else {
			log.Printf("Warning: Failed to decrypt password, using as-is: %v", err)
		}
	}

	// Decrypt database
	if decryptedDatabase, err := decrypt(conn.Database, key); err == nil {
		conn.Database = decryptedDatabase
	} else {
		log.Printf("Warning: Failed to decrypt database, using as-is: %v", err)
	}

	// Decrypt SSL certificate URLs if present
	if conn.SSLCertURL != nil {
		if decryptedURL, err := decrypt(*conn.SSLCertURL, key); err == nil {
			*conn.SSLCertURL = decryptedURL
		} else {
			log.Printf("Warning: Failed to decrypt SSL certificate URL, using as-is: %v", err)
		}
	}

	if conn.SSLKeyURL != nil {
		if decryptedURL, err := decrypt(*conn.SSLKeyURL, key); err == nil {
			*conn.SSLKeyURL = decryptedURL
		} else {
			log.Printf("Warning: Failed to decrypt SSL key URL, using as-is: %v", err)
		}
	}

	if conn.SSLRootCertURL != nil {
		if decryptedURL, err := decrypt(*conn.SSLRootCertURL, key); err == nil {
			*conn.SSLRootCertURL = decryptedURL
		} else {
			log.Printf("Warning: Failed to decrypt SSL root certificate URL, using as-is: %v", err)
		}
	}

	// Decrypt Google Sheets URL if present
	if conn.GoogleSheetURL != nil {
		if decryptedURL, err := decrypt(*conn.GoogleSheetURL, key); err == nil {
			*conn.GoogleSheetURL = decryptedURL
		} else {
			log.Printf("Warning: Failed to decrypt Google Sheets URL, using as-is: %v", err)
		}
	}

	// Decrypt Google auth tokens if present
	if conn.GoogleAuthToken != nil {
		if decryptedToken, err := decrypt(*conn.GoogleAuthToken, key); err == nil {
			*conn.GoogleAuthToken = decryptedToken
		} else {
			log.Printf("Warning: Failed to decrypt Google auth token, using as-is: %v", err)
		}
	}

	if conn.GoogleRefreshToken != nil {
		if decryptedToken, err := decrypt(*conn.GoogleRefreshToken, key); err == nil {
			*conn.GoogleRefreshToken = decryptedToken
		} else {
			log.Printf("Warning: Failed to decrypt Google refresh token, using as-is: %v", err)
		}
	}

	// Decrypt SSH authentication fields if present
	if conn.SSHPrivateKey != nil {
		if decryptedKey, err := decrypt(*conn.SSHPrivateKey, key); err == nil {
			*conn.SSHPrivateKey = decryptedKey
		} else {
			log.Printf("Warning: Failed to decrypt SSH private key, using as-is: %v", err)
		}
	}

	if conn.SSHPassword != nil {
		if decryptedPassword, err := decrypt(*conn.SSHPassword, key); err == nil {
			*conn.SSHPassword = decryptedPassword
		} else {
			log.Printf("Warning: Failed to decrypt SSH password, using as-is: %v", err)
		}
	}

	if conn.SSHPassphrase != nil {
		if decryptedPassphrase, err := decrypt(*conn.SSHPassphrase, key); err == nil {
			*conn.SSHPassphrase = decryptedPassphrase
		} else {
			log.Printf("Warning: Failed to decrypt SSH passphrase, using as-is: %v", err)
		}
	}

	if conn.SSHPrivateKeyURL != nil {
		if decryptedURL, err := decrypt(*conn.SSHPrivateKeyURL, key); err == nil {
			*conn.SSHPrivateKeyURL = decryptedURL
		} else {
			log.Printf("Warning: Failed to decrypt SSH private key URL, using as-is: %v", err)
		}
	}
}

// encrypt encrypts a string using AES-GCM
func encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Generate a random nonce
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Create GCM cipher
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Encrypt and append nonce
	ciphertext := aesgcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a string using AES-GCM
func decrypt(encodedData string, key []byte) (string, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < 12 {
		return "", fmt.Errorf("data too short")
	}

	nonce := data[:12]
	ciphertext := data[12:]

	// Decrypt
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
