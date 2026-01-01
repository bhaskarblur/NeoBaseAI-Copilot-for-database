package models

import "neobase-ai/internal/constants"

type User struct {
	Username           string             `bson:"username" json:"username"`
	Email              string             `bson:"email" json:"email"`
	Password           string             `bson:"password" json:"-"`
	AuthType           constants.AuthType `bson:"auth_type" json:"auth_type"`                                         // AuthTypeEmailPassword or AuthTypeGoogle, default is AuthTypeEmailPassword
	GoogleID           *string            `bson:"google_id,omitempty" json:"google_id,omitempty"`                     // Google user ID
	GoogleAccessToken  *string            `bson:"google_access_token,omitempty" json:"-"`                             // Google OAuth access token (not exposed in JSON)
	GoogleRefreshToken *string            `bson:"google_refresh_token,omitempty" json:"-"`                            // Google OAuth refresh token (not exposed in JSON)
	GoogleTokenExpiry  *int64             `bson:"google_token_expiry,omitempty" json:"google_token_expiry,omitempty"` // Token expiry timestamp
	Base               `bson:",inline"`
}

func NewUser(username, email, password string) *User {
	return &User{
		Username: username,
		Email:    email,
		Password: password,
		AuthType: constants.AuthTypeEmailPassword,
		Base:     NewBase(),
	}
}

// GetAuthType returns the auth type, defaulting to AuthTypeEmailPassword for backward compatibility
func (u *User) GetAuthType() constants.AuthType {
	if u.AuthType == "" {
		return constants.AuthTypeEmailPassword
	}
	return u.AuthType
}
