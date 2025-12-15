package dbmanager

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSHTunnel represents an SSH tunnel connection
type SSHTunnel struct {
	SSHConfig  *ssh.ClientConfig
	SSHHost    string
	SSHPort    string
	Local      net.Listener
	ServerConn *ssh.Client
	AuthMethod SSHAuthMethod // publickey or password
}

// Dialer function for SQL connections through SSH tunnel
type SSHTunnelDialer struct {
	tunnel *SSHTunnel
}

// Dial implements the net.Dialer interface for SSH tunneling
func (td *SSHTunnelDialer) Dial(network, addr string) (net.Conn, error) {
	return td.tunnel.ServerConn.Dial(network, addr)
}

// CreateSSHTunnel creates an SSH tunnel connection
func CreateSSHTunnel(sshHost, sshPort, sshUsername, sshPrivateKey, sshPassphrase string) (*SSHTunnel, error) {
	if sshHost == "" || sshPort == "" || sshUsername == "" {
		return nil, fmt.Errorf("incomplete SSH configuration: host, port, and username are required")
	}

	log.Printf("SSHTunnel -> CreateSSHTunnel -> Establishing SSH tunnel to %s:%s as %s", sshHost, sshPort, sshUsername)

	// Parse private key
	var signer ssh.Signer
	var err error
	var authMethod SSHAuthMethod

	// Try public key authentication if private key is provided
	if sshPrivateKey != "" {
		signer, err = parseSSHPrivateKey(sshPrivateKey, sshPassphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key: %v", err)
		}
		authMethod = SSHAuthMethodPublicKey
	} else {
		return nil, fmt.Errorf("SSH private key is required for public key authentication")
	}

	// Create SSH client config
	sshConfig := &ssh.ClientConfig{
		User: sshUsername,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Not secure, but needed for dynamic hosts
	}

	// Connect to SSH server
	serverAddr := net.JoinHostPort(sshHost, sshPort)
	serverConn, err := ssh.Dial("tcp", serverAddr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH server: %v", err)
	}

	log.Printf("SSHTunnel -> CreateSSHTunnel -> Successfully established SSH tunnel to %s", serverAddr)

	return &SSHTunnel{
		SSHConfig:  sshConfig,
		SSHHost:    sshHost,
		SSHPort:    sshPort,
		ServerConn: serverConn,
		AuthMethod: authMethod,
	}, nil
}

// CreateSSHTunnelWithPassword creates an SSH tunnel using password authentication
func CreateSSHTunnelWithPassword(sshHost, sshPort, sshUsername, sshPassword string) (*SSHTunnel, error) {
	if sshHost == "" || sshPort == "" || sshUsername == "" || sshPassword == "" {
		return nil, fmt.Errorf("incomplete SSH configuration: host, port, username, and password are required")
	}

	log.Printf("SSHTunnel -> CreateSSHTunnelWithPassword -> Establishing SSH tunnel to %s:%s as %s using password auth", sshHost, sshPort, sshUsername)

	// Create SSH client config with password authentication
	sshConfig := &ssh.ClientConfig{
		User: sshUsername,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Not secure, but needed for dynamic hosts
	}

	// Connect to SSH server
	serverAddr := net.JoinHostPort(sshHost, sshPort)
	serverConn, err := ssh.Dial("tcp", serverAddr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH server: %v", err)
	}

	log.Printf("SSHTunnel -> CreateSSHTunnelWithPassword -> Successfully established SSH tunnel to %s", serverAddr)

	return &SSHTunnel{
		SSHConfig:  sshConfig,
		SSHHost:    sshHost,
		SSHPort:    sshPort,
		ServerConn: serverConn,
		AuthMethod: SSHAuthMethodPassword,
	}, nil
}

// parseSSHPrivateKey handles both OpenSSH format and legacy RSA format
func parseSSHPrivateKey(privateKey, passphrase string) (ssh.Signer, error) {
	keyBytes := []byte(privateKey)

	// Try parsing as-is first (OpenSSH format or unencrypted legacy format)
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err == nil {
		return signer, nil
	}

	// If we have a passphrase, try parsing with it
	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
		if err == nil {
			return signer, nil
		}
	}

	// Check if it's a legacy RSA format and needs conversion
	if strings.Contains(privateKey, "BEGIN RSA PRIVATE KEY") {
		log.Printf("SSHTunnel -> parseSSHPrivateKey -> Detected legacy RSA format, attempting conversion")
		// Convert legacy RSA to OpenSSH format
		convertedKey, err := convertLegacyRSAToOpenSSH(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to convert legacy RSA key: %v", err)
		}

		// Try parsing the converted key
		if passphrase != "" {
			return ssh.ParsePrivateKeyWithPassphrase([]byte(convertedKey), []byte(passphrase))
		}
		return ssh.ParsePrivateKey([]byte(convertedKey))
	}

	// If all parsing attempts failed, return the original error
	return nil, fmt.Errorf("unable to parse SSH private key - ensure it's in OpenSSH format (-----BEGIN OPENSSH PRIVATE KEY-----) or encrypted RSA format")
}

// convertLegacyRSAToOpenSSH converts legacy RSA format to OpenSSH format
// This is a simplified converter - for production use, consider using openssh library
func convertLegacyRSAToOpenSSH(legacyKey string) (string, error) {
	// This is a placeholder implementation
	// In production, you might want to use github.com/mikesmitty/edkey or similar
	// For now, we'll return an error with helpful message
	return "", fmt.Errorf("legacy RSA format conversion not fully implemented. Please convert your key to OpenSSH format using: ssh-keygen -p -N \"\" -m pem -f /path/to/key && ssh-keygen -p -N \"\" -m RFC4716 -f /path/to/key")
}

// LoadPrivateKeyFromURL fetches a private key from a URL
func LoadPrivateKeyFromURL(keyURL string) (string, error) {
	if keyURL == "" {
		return "", fmt.Errorf("SSH private key URL is empty")
	}

	log.Printf("SSHTunnel -> LoadPrivateKeyFromURL -> Fetching private key from URL: %s", keyURL)

	// Validate URL
	_, err := url.Parse(keyURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %v", err)
	}

	// Fetch the key from URL
	resp, err := http.Get(keyURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch private key from URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch private key: HTTP %d", resp.StatusCode)
	}

	keyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read private key content: %v", err)
	}

	log.Printf("SSHTunnel -> LoadPrivateKeyFromURL -> Successfully fetched private key from URL")
	return string(keyBytes), nil
}

// Close closes the SSH tunnel
func (t *SSHTunnel) Close() error {
	if t.ServerConn != nil {
		log.Printf("SSHTunnel -> Close -> Closing SSH connection to %s:%s", t.SSHHost, t.SSHPort)
		return t.ServerConn.Close()
	}
	return nil
}

// TestSSHTunnel tests the SSH tunnel connection
func TestSSHTunnel(sshHost, sshPort, sshUsername, sshPrivateKey, sshPassphrase string) error {
	tunnel, err := CreateSSHTunnel(sshHost, sshPort, sshUsername, sshPrivateKey, sshPassphrase)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	log.Printf("SSHTunnel -> TestSSHTunnel -> Successfully tested SSH tunnel")
	return nil
}

// TestSSHTunnelWithPassword tests the SSH tunnel connection using password authentication
func TestSSHTunnelWithPassword(sshHost, sshPort, sshUsername, sshPassword string) error {
	tunnel, err := CreateSSHTunnelWithPassword(sshHost, sshPort, sshUsername, sshPassword)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	log.Printf("SSHTunnel -> TestSSHTunnelWithPassword -> Successfully tested SSH tunnel with password")
	return nil
}

// CreatePostgresSSLURL creates a PostgreSQL connection string for use with SSH tunnel
func CreatePostgresSSLURL(host string, port string, username string, password string, database string) string {
	// When using SSH tunnel, connect to localhost with the tunneled port
	u := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(username, password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + database,
	}
	return u.String()
}

// DialSSHTunnel is a helper to dial through SSH tunnel
func (t *SSHTunnel) DialSSH(network, addr string) (net.Conn, error) {
	log.Printf("SSHTunnel -> DialSSH -> Dialing %s through SSH tunnel to %s:%s", addr, t.SSHHost, t.SSHPort)
	return t.ServerConn.Dial(network, addr)
}

// StreamForward handles port forwarding through the SSH tunnel
func (t *SSHTunnel) StreamForward(remoteAddr string) (net.Conn, error) {
	connection, err := t.ServerConn.Dial("tcp", remoteAddr)
	if err != nil {
		return nil, err
	}

	go io.Copy(connection, connection)
	return connection, nil
}
