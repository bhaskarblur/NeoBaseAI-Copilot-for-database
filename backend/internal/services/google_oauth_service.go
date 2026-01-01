package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/dtos"
	"net/http"
	"net/url"
	"strings"
)

type GoogleOAuthService interface {
	ExchangeCodeForToken(code, redirectURI string) (*dtos.GoogleTokenResponse, error)
	GetUserInfo(accessToken string) (*dtos.GoogleUserInfo, error)
	RefreshAccessToken(refreshToken string) (*dtos.GoogleTokenResponse, error)
}

type googleOAuthService struct {
	clientID     string
	clientSecret string
}

func NewGoogleOAuthService() GoogleOAuthService {
	return &googleOAuthService{
		clientID:     config.Env.GoogleClientID,
		clientSecret: config.Env.GoogleClientSecret,
	}
}

// ExchangeCodeForToken exchanges the authorization code for access and refresh tokens
func (s *googleOAuthService) ExchangeCodeForToken(code, redirectURI string) (*dtos.GoogleTokenResponse, error) {
	tokenURL := "https://oauth2.googleapis.com/token"

	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", s.clientID)
	data.Set("client_secret", s.clientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(context.Background(), "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Google OAuth token exchange failed: %s", string(body))
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse dtos.GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %v", err)
	}

	log.Printf("Token exchange successful. Access token length: %d, Has refresh token: %v, Expires in: %d",
		len(tokenResponse.AccessToken), tokenResponse.RefreshToken != "", tokenResponse.ExpiresIn)

	return &tokenResponse, nil
}

// GetUserInfo retrieves user information from Google using the access token
func (s *googleOAuthService) GetUserInfo(accessToken string) (*dtos.GoogleUserInfo, error) {
	userInfoURL := "https://www.googleapis.com/oauth2/v2/userinfo"

	log.Printf("Getting user info with token (length: %d, first 10 chars: %s...)",
		len(accessToken), accessToken[:10])

	req, err := http.NewRequestWithContext(context.Background(), "GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Google user info request failed: %s", string(body))
		return nil, fmt.Errorf("user info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var userInfo dtos.GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info response: %v", err)
	}

	return &userInfo, nil
}

// RefreshAccessToken refreshes the access token using the refresh token
func (s *googleOAuthService) RefreshAccessToken(refreshToken string) (*dtos.GoogleTokenResponse, error) {
	if refreshToken == "" {
		return nil, errors.New("refresh token is required")
	}

	tokenURL := "https://oauth2.googleapis.com/token"

	data := url.Values{}
	data.Set("client_id", s.clientID)
	data.Set("client_secret", s.clientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(context.Background(), "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh access token: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh token response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Google OAuth token refresh failed: %s", string(body))
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse dtos.GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse refresh token response: %v", err)
	}

	return &tokenResponse, nil
}
