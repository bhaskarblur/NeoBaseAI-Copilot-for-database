package dbmanager

// SSHAuthMethod represents the SSH authentication method enum
type SSHAuthMethod string

const (
	// SSHAuthMethodPublicKey represents public key authentication
	SSHAuthMethodPublicKey SSHAuthMethod = "publickey"
	// SSHAuthMethodPassword represents password-based authentication
	SSHAuthMethodPassword SSHAuthMethod = "password"
)

// String returns the string representation of the auth method
func (m SSHAuthMethod) String() string {
	return string(m)
}

// IsValid checks if the auth method is valid
func (m SSHAuthMethod) IsValid() bool {
	return m == SSHAuthMethodPublicKey || m == SSHAuthMethodPassword
}

// ToSSHAuthMethod converts a string to SSHAuthMethod
func ToSSHAuthMethod(s string) SSHAuthMethod {
	switch s {
	case "publickey":
		return SSHAuthMethodPublicKey
	case "password":
		return SSHAuthMethodPassword
	default:
		return SSHAuthMethodPublicKey // Default to public key
	}
}
