package constants

// AuthType represents the authentication method used for a user account
type AuthType string

const (
	// AuthTypeEmailPassword represents traditional email-password authentication
	AuthTypeEmailPassword AuthType = "email-password"

	// AuthTypeGoogle represents Google OAuth2 authentication
	AuthTypeGoogle AuthType = "google"
)

// String returns the string representation of the AuthType
func (at AuthType) String() string {
	return string(at)
}

// IsValid checks if the AuthType is a valid value
func (at AuthType) IsValid() bool {
	return at == AuthTypeEmailPassword || at == AuthTypeGoogle
}
