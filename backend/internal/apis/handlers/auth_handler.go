package handlers

import (
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService services.AuthService
}

func NewAuthHandler(authService services.AuthService) *AuthHandler {
	if authService == nil {
		log.Fatal("Auth service cannot be nil")
	}
	return &AuthHandler{
		authService: authService,
	}
}

// @Summary Signup
// @Description Signup a new user
// @Accept json
// @Produce json
// @Param signupRequest body dtos.SignupRequest true "Signup request"
// @Success 200 {object} dtos.Response

func (h *AuthHandler) Signup(c *gin.Context) {
	var req dtos.SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	if h.authService == nil {
		log.Println("Auth service is nil")
	}
	response, statusCode, err := h.authService.Signup(&req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Login
// @Description Login a user
// @Accept json
// @Produce json
// @Param loginRequest body dtos.LoginRequest true "Login request"
// @Success 200 {object} dtos.Response
func (h *AuthHandler) Login(c *gin.Context) {
	var req dtos.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	response, statusCode, err := h.authService.Login(&req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Generate User Signup Secret
// @Description Generate a secret for user signup
// @Accept json
// @Produce json
// @Param userSignupSecretRequest body dtos.UserSignupSecretRequest true "User signup secret request"
// @Success 200 {object} dtos.Response

func (h *AuthHandler) GenerateUserSignupSecret(c *gin.Context) {
	var req dtos.UserSignupSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	response, statusCode, err := h.authService.GenerateUserSignupSecret(&req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Refresh Token
// @Description Refresh a user's access token
// @Accept json
// @Produce json
// @Param refreshToken header string true "Refresh token"
// @Success 200 {object} dtos.Response

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken := c.GetHeader("Authorization")
	parts := strings.Split(refreshToken, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		errorMsg := "Invalid authorization header"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}
	refreshToken = parts[1]

	response, statusCode, err := h.authService.RefreshToken(refreshToken)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Logout
// @Description Logout a user
// @Accept json
// @Produce json
// @Param logoutRequest body dtos.LogoutRequest true "Logout request"
// @Success 200 {object} dtos.Response

func (h *AuthHandler) Logout(c *gin.Context) {
	var req dtos.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Get the access token from Authorization header
	authHeader := c.GetHeader("Authorization")
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		errorMsg := "Invalid authorization header"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}
	accessToken := parts[1]

	statusCode, err := h.authService.Logout(req.RefreshToken, accessToken)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Successfully logged out",
	})
}

// @Summary Get User
// @Description Get user details
// @Accept json
// @Produce json
// @Success 200 {object} dtos.Response
func (h *AuthHandler) GetUser(c *gin.Context) {
	userID := c.GetString("userID")
	user, statusCode, err := h.authService.GetUser(userID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    user,
	})
}

// @Summary Forgot Password
// @Description Send password reset OTP to user's email
// @Accept json
// @Produce json
// @Param forgotPasswordRequest body dtos.ForgotPasswordRequest true "Forgot password request"
// @Success 200 {object} dtos.Response
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req dtos.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	response, statusCode, err := h.authService.ForgotPassword(&req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Reset Password
// @Description Reset user password using OTP
// @Accept json
// @Produce json
// @Param resetPasswordRequest body dtos.ResetPasswordRequest true "Reset password request"
// @Success 200 {object} dtos.Response
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dtos.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	statusCode, err := h.authService.ResetPassword(&req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Password reset successfully",
	})
}
