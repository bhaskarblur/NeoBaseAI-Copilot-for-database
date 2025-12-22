package dtos

import "neobase-ai/internal/models"

type SignupRequest struct {
	Username         string `json:"username" binding:"required"`
	Email            string `json:"email" binding:"required,email"`
	Password         string `json:"password" binding:"required,min=6"`
	UserSignupSecret string `json:"user_signup_secret"`
}

type LoginRequest struct {
	UsernameOrEmail string `json:"username_or_email" binding:"required"`
	Password        string `json:"password" binding:"required"`
}

type UserSignupSecretRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         models.User `json:"user"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	OTP         string `json:"otp" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type ForgotPasswordResponse struct {
	Message string `json:"message"`
}

// Google OAuth DTOs
type GoogleOAuthRequest struct {
	Code             string  `json:"code" binding:"required"`         // Authorization code from Google
	RedirectURI      string  `json:"redirect_uri" binding:"required"` // Must match the one registered
	UserSignupSecret *string `json:"user_signup_secret,omitempty"`    // Required for signup in production
	Purpose          string  `json:"purpose" binding:"required"`      // "auth" or "spreadsheet"
	Action           string  `json:"action,omitempty"`                // "login" or "signup" (for auth purpose)
}

type GoogleOAuthCallbackRequest struct {
	Code        string `json:"code" binding:"required"`
	State       string `json:"state" binding:"required"` // Contains purpose and optional signup secret
	RedirectURI string `json:"redirect_uri" binding:"required"`
}

type ValidateSignupSecretRequest struct {
	UserSignupSecret string `json:"user_signup_secret" binding:"required"`
}

type ValidateSignupSecretResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token,omitempty"`
}

type GoogleOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	UserEmail    string `json:"user_email"`
}
