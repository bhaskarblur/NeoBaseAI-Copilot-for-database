package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"neobase-ai/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

type GoogleOAuthHandler struct {
	oauthConfig *oauth2.Config
}

func NewGoogleOAuthHandler() *GoogleOAuthHandler {
	return &GoogleOAuthHandler{
		oauthConfig: &oauth2.Config{
			ClientID:     config.Env.GoogleClientID,
			ClientSecret: config.Env.GoogleClientSecret,
			RedirectURL:  config.Env.GoogleRedirectURL,
			Scopes: []string{
				sheets.SpreadsheetsReadonlyScope,
				"https://www.googleapis.com/auth/userinfo.email",
			},
			Endpoint: google.Endpoint,
		},
	}
}

// InitiateGoogleAuth starts the Google OAuth flow
func (h *GoogleOAuthHandler) InitiateGoogleAuth(c *gin.Context) {
	// Get optional state parameter from query
	state := c.DefaultQuery("state", "")
	
	// Generate OAuth URL
	authURL := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	
	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
	})
}

// HandleGoogleCallback handles the OAuth callback from Google
func (h *GoogleOAuthHandler) HandleGoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code not provided"})
		return
	}

	// Exchange code for token
	token, err := h.oauthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		log.Printf("Failed to exchange authorization code: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange authorization code"})
		return
	}

	// Get user info to verify the token works
	client := h.oauthConfig.Client(c.Request.Context(), token)
	userInfoResp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}
	defer userInfoResp.Body.Close()

	var userInfo map[string]interface{}
	if err := json.NewDecoder(userInfoResp.Body).Decode(&userInfo); err != nil {
		log.Printf("Failed to decode user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user info"})
		return
	}

	// Return tokens and user info
	c.JSON(http.StatusOK, gin.H{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expiry":        token.Expiry,
		"user_email":    userInfo["email"],
	})
}

// ValidateGoogleSheetAccess validates that the user has access to a specific Google Sheet
func (h *GoogleOAuthHandler) ValidateGoogleSheetAccess(c *gin.Context) {
	var req struct {
		AccessToken  string `json:"access_token" binding:"required"`
		RefreshToken string `json:"refresh_token" binding:"required"`
		SheetID      string `json:"sheet_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create token
	token := &oauth2.Token{
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		TokenType:    "Bearer",
	}

	// Create client and test access to the sheet
	client := h.oauthConfig.Client(c.Request.Context(), token)
	
	// Try to get spreadsheet metadata
	url := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s", req.SheetID)
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Failed to access Google Sheet: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access Google Sheet"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to the specified Google Sheet"})
		return
	}

	// Parse spreadsheet info
	var sheetInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sheetInfo); err != nil {
		log.Printf("Failed to decode sheet info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode sheet info"})
		return
	}

	// Get sheet names
	sheets := []string{}
	if sheetsData, ok := sheetInfo["sheets"].([]interface{}); ok {
		for _, sheet := range sheetsData {
			if sheetMap, ok := sheet.(map[string]interface{}); ok {
				if props, ok := sheetMap["properties"].(map[string]interface{}); ok {
					if title, ok := props["title"].(string); ok {
						sheets = append(sheets, title)
					}
				}
			}
		}
	}

	// Construct sheet URL from the sheet ID
	sheetURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit", req.SheetID)

	c.JSON(http.StatusOK, gin.H{
		"valid":       true,
		"title":       sheetInfo["properties"].(map[string]interface{})["title"],
		"sheet_count": len(sheets),
		"sheets":      sheets,
		"sheet_url":   sheetURL,
	})
}

// RefreshGoogleToken refreshes an expired Google OAuth token
func (h *GoogleOAuthHandler) RefreshGoogleToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create token source with refresh token
	tokenSource := h.oauthConfig.TokenSource(c.Request.Context(), &oauth2.Token{
		RefreshToken: req.RefreshToken,
	})

	// Get new token
	newToken, err := tokenSource.Token()
	if err != nil {
		log.Printf("Failed to refresh token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newToken.AccessToken,
		"refresh_token": newToken.RefreshToken,
		"expiry":        newToken.Expiry,
	})
}