package services

import (
	"errors"
	"fmt"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/models"
	"neobase-ai/internal/repositories"
	"neobase-ai/internal/utils"
	"net/http"
	"strings"
	"time"
)

type AuthService interface {
	Signup(req *dtos.SignupRequest) (*dtos.AuthResponse, uint, error)
	Login(req *dtos.LoginRequest) (*dtos.AuthResponse, uint, error)
	GenerateUserSignupSecret(req *dtos.UserSignupSecretRequest) (*models.UserSignupSecret, uint, error)
	ValidateSignupSecret(secret string) (bool, error)
	GoogleOAuthLogin(req *dtos.GoogleOAuthRequest) (*dtos.AuthResponse, uint, error)
	GoogleOAuthSignup(req *dtos.GoogleOAuthRequest) (*dtos.AuthResponse, uint, error)
	RefreshToken(refreshToken string) (*dtos.RefreshTokenResponse, uint32, error)
	Logout(refreshToken string, accessToken string) (uint32, error)
	GetUser(userID string) (*models.User, uint, error)
	SetChatService(chatService ChatService)
	ForgotPassword(req *dtos.ForgotPasswordRequest) (*dtos.ForgotPasswordResponse, uint, error)
	ResetPassword(req *dtos.ResetPasswordRequest) (uint, error)
}

type authService struct {
	chatService        ChatService
	userRepo           repositories.UserRepository
	jwtService         utils.JWTService
	tokenRepo          repositories.TokenRepository
	emailService       EmailService
	googleOAuthService GoogleOAuthService
}

func NewAuthService(userRepo repositories.UserRepository, jwtService utils.JWTService, tokenRepo repositories.TokenRepository, emailService EmailService, googleOAuthService GoogleOAuthService) AuthService {
	return &authService{
		userRepo:           userRepo,
		jwtService:         jwtService,
		tokenRepo:          tokenRepo,
		emailService:       emailService,
		googleOAuthService: googleOAuthService,
	}
}

func (s *authService) SetChatService(chatService ChatService) {
	s.chatService = chatService
}

func (s *authService) Signup(req *dtos.SignupRequest) (*dtos.AuthResponse, uint, error) {
	if config.Env.Environment == "DEVELOPMENT" {
		log.Println("Development mode, skipping user signup secret validation")
	} else {
		validUserSignupSecret := s.userRepo.ValidateUserSignupSecret(req.UserSignupSecret)
		if !validUserSignupSecret {
			return nil, http.StatusUnauthorized, errors.New("invalid user signup secret")
		}
	}

	// Check if email already exists
	existingUserByEmail, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if existingUserByEmail != nil {
		return nil, http.StatusBadRequest, errors.New("User with this email already exists")
	}

	// Check if username already exists
	existingUserByUsername, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if existingUserByUsername != nil {
		return nil, http.StatusBadRequest, errors.New("User with this username already exists")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	// Create user
	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		Base: models.Base{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, http.StatusBadRequest, err
	}

	// Send welcome email (async, don't block signup process)
	go func() {
		err := s.emailService.SendWelcomeEmail(req.Email, req.Username)
		if err != nil {
			log.Printf("⚠️  Failed to send welcome email to %s: %v", req.Email, err)
		}
	}()

	// Generate token
	accessToken, err := s.jwtService.GenerateToken(user.ID.Hex())
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID.Hex())
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	err = s.tokenRepo.StoreRefreshToken(user.ID.Hex(), *refreshToken)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	go func() {
		if config.Env.Environment == "DEVELOPMENT" {
			log.Println("Development mode, skipping user signup secret deletion")
		} else {
			err := s.userRepo.DeleteUserSignupSecret(req.UserSignupSecret)
			if err != nil {
				log.Println("failed to delete user signup secret:" + err.Error())
			}
		}
	}()

	// Create a default chat for the user in development mode
	if config.Env.Environment == "DEVELOPMENT" {
		chat, _, err := s.chatService.CreateWithoutConnectionPing(user.ID.Hex(), &dtos.CreateChatRequest{
			Connection: dtos.CreateConnectionRequest{
				Type:     config.Env.ExampleDatabaseType,
				Host:     config.Env.ExampleDatabaseHost,
				Port:     utils.StringPtr(config.Env.ExampleDatabasePort),
				Database: config.Env.ExampleDatabaseName,
				Username: config.Env.ExampleDatabaseUsername,
				Password: utils.StringPtr(config.Env.ExampleDatabasePassword),
			},
			Settings: dtos.CreateChatSettings{
				AutoExecuteQuery: utils.TruePtr(),
				ShareDataWithAI:  utils.FalsePtr(), // Disable sharing data with AI by default.
			},
		})
		if err != nil {
			log.Println("failed to create chat:" + err.Error())
		}
		if chat != nil {
			log.Println("chat created:", chat.ID)
		}

	}
	return &dtos.AuthResponse{
		AccessToken:  *accessToken,
		RefreshToken: *refreshToken,
		User:         *user,
	}, http.StatusCreated, nil
}

func (s *authService) Login(req *dtos.LoginRequest) (*dtos.AuthResponse, uint, error) {
	var authUser *models.User
	var err error
	// Check if it's Admin User
	if req.UsernameOrEmail == config.Env.AdminUser {
		log.Println("Admin User Login")
		if req.Password != config.Env.AdminPassword {
			return nil, http.StatusUnauthorized, errors.New("invalid password")
		}
		user, err := s.userRepo.FindByUsername(req.UsernameOrEmail)
		// Checking if Admin user exists in the DB, if not then create user for admin creds
		if err != nil || user == nil {
			log.Println("Admin User not found, creating user")
			// Hash password
			hashedPassword, err := utils.HashPassword(req.Password)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}

			// Create user
			authUser = &models.User{
				Username: req.UsernameOrEmail,
				Email:    "", // Admin user doesn't need email
				Password: hashedPassword,
				Base: models.Base{
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}

			if err = s.userRepo.Create(authUser); err != nil {
				log.Println("Failed to create admin user:" + err.Error())
				return nil, http.StatusBadRequest, err
			}
		} else {
			authUser = user
		}
	} else {
		log.Println("Non-Admin User Login")
		authUser, err = s.userRepo.FindByUsernameOrEmail(req.UsernameOrEmail)
		if err != nil {
			log.Println("Failed to find user:" + err.Error())
			return nil, http.StatusUnauthorized, err
		}
		if authUser == nil {
			log.Println("User not found")
			return nil, http.StatusUnauthorized, errors.New("Invalid credentials, User does not exist.")
		}

		if !utils.CheckPasswordHash(req.Password, authUser.Password) {
			log.Println("Invalid credentials")
			return nil, http.StatusUnauthorized, errors.New("Invalid credentials. Please try again.")
		}
	}
	accessToken, err := s.jwtService.GenerateToken(authUser.ID.Hex())
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(authUser.ID.Hex())
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	err = s.tokenRepo.StoreRefreshToken(authUser.ID.Hex(), *refreshToken)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	return &dtos.AuthResponse{
		AccessToken:  *accessToken,
		RefreshToken: *refreshToken,
		User:         *authUser,
	}, http.StatusOK, nil
}

func (s *authService) GenerateUserSignupSecret(req *dtos.UserSignupSecretRequest) (*models.UserSignupSecret, uint, error) {

	secret := utils.GenerateSecret()

	createdSecret, err := s.userRepo.CreateUserSignUpSecret(secret)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	return createdSecret, http.StatusCreated, nil
}

func (s *authService) RefreshToken(refreshToken string) (*dtos.RefreshTokenResponse, uint32, error) {
	// Validate the refresh token
	claims, err := s.jwtService.ValidateToken(refreshToken)
	if err != nil {
		return nil, http.StatusUnauthorized, fmt.Errorf("invalid refresh token")
	}

	log.Println("Validating refresh token:", refreshToken)
	// Check if the refresh token exists in Redis
	if !s.tokenRepo.ValidateRefreshToken(*claims, refreshToken) {
		return nil, http.StatusUnauthorized, fmt.Errorf("refresh token not found")
	}

	// Generate new tokens
	accessToken, err := s.jwtService.GenerateToken(*claims)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return &dtos.RefreshTokenResponse{
		AccessToken: *accessToken,
	}, http.StatusOK, nil
}

func (s *authService) Logout(refreshToken string, accessToken string) (uint32, error) {
	// Validate the refresh token
	claims, err := s.jwtService.ValidateToken(refreshToken)
	if err != nil {
		return http.StatusUnauthorized, fmt.Errorf("invalid refresh token")
	}

	// Delete the refresh token from Redis
	if err := s.tokenRepo.DeleteRefreshToken(*claims, refreshToken); err != nil {
		return http.StatusInternalServerError, err
	}

	// Blacklist the access token until its original expiration
	_, err = s.jwtService.ValidateToken(accessToken)
	if err != nil {
		return http.StatusUnauthorized, fmt.Errorf("invalid access token")
	}

	if err := s.tokenRepo.BlacklistToken(accessToken, time.Duration(config.Env.JWTExpirationMilliseconds)); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func (s *authService) GetUser(userID string) (*models.User, uint, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, http.StatusNotFound, err
	}
	if user == nil {
		return nil, http.StatusNotFound, errors.New("user not found")
	}

	return user, http.StatusOK, nil
}

func (s *authService) ForgotPassword(req *dtos.ForgotPasswordRequest) (*dtos.ForgotPasswordResponse, uint, error) {
	// Check if user exists with this email
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if user == nil {
		// For security reasons, don't reveal if email exists or not
		return &dtos.ForgotPasswordResponse{
			Message: "If an account with this email exists, you will receive a password reset email shortly.",
		}, http.StatusOK, nil
	}

	// Generate 6-digit OTP
	otp := utils.GenerateOTP()

	// Store OTP in Redis
	err = s.userRepo.StorePasswordResetOTP(req.Email, otp)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	// Send OTP email (non-blocking)
	err = s.emailService.SendPasswordResetOTP(req.Email, user.Username, otp)
	if err != nil {
		log.Printf("⚠️  Failed to send password reset email to %s: %v", req.Email, err)
		// Don't return error to user for security reasons - email failure shouldn't block the flow
	}

	return &dtos.ForgotPasswordResponse{
		Message: "If an account with this email exists, you will receive a password reset email shortly.",
	}, http.StatusOK, nil
}

func (s *authService) ResetPassword(req *dtos.ResetPasswordRequest) (uint, error) {
	// Validate OTP
	isValidOTP := s.userRepo.ValidatePasswordResetOTP(req.Email, req.OTP)
	if !isValidOTP {
		return http.StatusBadRequest, errors.New("Invalid or expired OTP")
	}

	// Find user by email
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if user == nil {
		return http.StatusNotFound, errors.New("User not found")
	}

	// Hash new password
	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Update password
	err = s.userRepo.UpdatePassword(user.ID.Hex(), hashedPassword)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Delete OTP from Redis (mark as used)
	err = s.userRepo.DeletePasswordResetOTP(req.Email)
	if err != nil {
		log.Printf("Failed to delete OTP from Redis: %v", err)
		// Don't return error as password is already updated
	}

	return http.StatusOK, nil
}

// ValidateSignupSecret validates if a signup secret is valid
func (s *authService) ValidateSignupSecret(secret string) (bool, error) {
	if secret == "" {
		return false, errors.New("signup secret is required")
	}
	return s.userRepo.ValidateUserSignupSecret(secret), nil
}

// GoogleOAuthLogin handles login via Google OAuth - user must exist
func (s *authService) GoogleOAuthLogin(req *dtos.GoogleOAuthRequest) (*dtos.AuthResponse, uint, error) {
	// If purpose is spreadsheet, just exchange code for tokens (no user auth)
	if req.Purpose == "spreadsheet" {
		// Exchange code for tokens
		tokenResp, err := s.googleOAuthService.ExchangeCodeForToken(req.Code, req.RedirectURI)
		if err != nil {
			log.Printf("Failed to exchange Google OAuth code for spreadsheet: %v", err)
			return nil, http.StatusBadRequest, fmt.Errorf("failed to authenticate with Google: %v", err)
		}

		// Get user info from Google
		userInfo, err := s.googleOAuthService.GetUserInfo(tokenResp.AccessToken)
		if err != nil {
			log.Printf("Failed to get Google user info: %v", err)
			return nil, http.StatusBadRequest, fmt.Errorf("failed to get user information from Google: %v", err)
		}

		// For spreadsheet OAuth, return tokens with user email
		// We'll return these in the RefreshToken field as a workaround since AuthResponse doesn't have all fields
		// The frontend will parse the response appropriately
		return &dtos.AuthResponse{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			User: models.User{
				Email: userInfo.Email,
			},
		}, http.StatusOK, nil
	}

	// Exchange code for tokens
	tokenResp, err := s.googleOAuthService.ExchangeCodeForToken(req.Code, req.RedirectURI)
	if err != nil {
		log.Printf("Failed to exchange Google OAuth code: %v", err)
		return nil, http.StatusBadRequest, fmt.Errorf("failed to authenticate with Google: %v", err)
	}

	// Get user info from Google
	userInfo, err := s.googleOAuthService.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		log.Printf("Failed to get Google user info: %v", err)
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get user information from Google: %v", err)
	}

	if !userInfo.VerifiedEmail {
		return nil, http.StatusBadRequest, errors.New("Google email is not verified")
	}

	// Check if user exists by Google ID
	existingUser, err := s.userRepo.FindByGoogleID(userInfo.ID)
	if err != nil {
		log.Printf("Error checking for existing Google user: %v", err)
		// Continue, might not be a Google user yet
	}

	// If user doesn't exist by Google ID, check by email
	if existingUser == nil {
		existingUser, err = s.userRepo.FindByEmail(userInfo.Email)
		if err != nil {
			log.Printf("Error checking for existing email user: %v", err)
		}
	}

	// User must exist for login
	if existingUser == nil {
		return nil, http.StatusUnauthorized, errors.New("No account found with this email. Please sign up first")
	}

	authUser := existingUser

	// Update Google tokens
	expiresAt := time.Now().Unix() + int64(tokenResp.ExpiresIn)
	authUser.GoogleAccessToken = &tokenResp.AccessToken
	authUser.GoogleTokenExpiry = &expiresAt
	if tokenResp.RefreshToken != "" {
		authUser.GoogleRefreshToken = &tokenResp.RefreshToken
	}
	authUser.UpdatedAt = time.Now()

	// Ensure Google ID is set
	if authUser.GoogleID == nil || *authUser.GoogleID == "" {
		authUser.GoogleID = &userInfo.ID
	}

	// Update user in database
	if err := s.userRepo.Update(authUser.ID.Hex(), authUser); err != nil {
		log.Printf("Failed to update user Google tokens: %v", err)
		// Continue anyway, tokens are not critical for login
	}

	log.Printf("Google OAuth: User logged in successfully - Email: %s", authUser.Email)

	// Generate JWT tokens
	accessToken, err := s.jwtService.GenerateToken(authUser.ID.Hex())
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate access token: %v", err)
	}

	jwtRefreshToken, err := s.jwtService.GenerateRefreshToken(authUser.ID.Hex())
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate refresh token: %v", err)
	}

	err = s.tokenRepo.StoreRefreshToken(authUser.ID.Hex(), *jwtRefreshToken)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to store refresh token: %v", err)
	}

	return &dtos.AuthResponse{
		AccessToken:  *accessToken,
		RefreshToken: *jwtRefreshToken,
		User:         *authUser,
	}, http.StatusOK, nil
}

// GoogleOAuthSignup handles signup via Google OAuth - user must NOT exist
func (s *authService) GoogleOAuthSignup(req *dtos.GoogleOAuthRequest) (*dtos.AuthResponse, uint, error) {
	// Exchange code for tokens
	tokenResp, err := s.googleOAuthService.ExchangeCodeForToken(req.Code, req.RedirectURI)
	if err != nil {
		log.Printf("Failed to exchange Google OAuth code: %v", err)
		return nil, http.StatusBadRequest, fmt.Errorf("failed to authenticate with Google: %v", err)
	}

	// Get user info from Google
	userInfo, err := s.googleOAuthService.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		log.Printf("Failed to get Google user info: %v", err)
		return nil, http.StatusBadRequest, fmt.Errorf("failed to get user information from Google: %v", err)
	}

	if !userInfo.VerifiedEmail {
		return nil, http.StatusBadRequest, errors.New("Google email is not verified")
	}

	// Check if user exists by Google ID
	existingUser, err := s.userRepo.FindByGoogleID(userInfo.ID)
	if err != nil {
		log.Printf("Error checking for existing Google user: %v", err)
		// Continue, might be a new user
	}

	// If user doesn't exist by Google ID, check by email
	if existingUser == nil {
		existingUser, err = s.userRepo.FindByEmail(userInfo.Email)
		if err != nil {
			log.Printf("Error checking for existing email user: %v", err)
		}
	}

	// User must NOT exist for signup
	if existingUser != nil {
		return nil, http.StatusBadRequest, errors.New("An account with this email already exists. Please log in instead")
	}

	// Validate signup secret in production
	if config.Env.Environment != "DEVELOPMENT" {
		if req.UserSignupSecret == nil || *req.UserSignupSecret == "" {
			return nil, http.StatusBadRequest, errors.New("signup secret is required for new users in production")
		}

		validSecret := s.userRepo.ValidateUserSignupSecret(*req.UserSignupSecret)
		if !validSecret {
			return nil, http.StatusUnauthorized, errors.New("invalid user signup secret")
		}
	}

	// Extract username from email (before @) or use name
	username := userInfo.Email
	if atIndex := strings.Index(userInfo.Email, "@"); atIndex > 0 {
		username = userInfo.Email[:atIndex]
	}
	if userInfo.Name != "" {
		// Replace spaces with underscores for username
		username = strings.ReplaceAll(userInfo.Name, " ", "_")
	}

	// Ensure username is unique
	baseUsername := username
	counter := 1
	for {
		existingByUsername, _ := s.userRepo.FindByUsername(username)
		if existingByUsername == nil {
			break
		}
		username = fmt.Sprintf("%s_%d", baseUsername, counter)
		counter++
	}

	expiresAt := time.Now().Unix() + int64(tokenResp.ExpiresIn)
	refreshToken := tokenResp.RefreshToken

	// Create new user
	authUser := &models.User{
		Username:           username,
		Email:              userInfo.Email,
		Password:           "", // No password for Google OAuth users
		AuthType:           "google",
		GoogleID:           &userInfo.ID,
		GoogleAccessToken:  &tokenResp.AccessToken,
		GoogleRefreshToken: &refreshToken,
		GoogleTokenExpiry:  &expiresAt,
		Base: models.Base{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := s.userRepo.Create(authUser); err != nil {
		log.Printf("Failed to create Google OAuth user: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create user account: %v", err)
	}

	// Send welcome email (async, don't block signup process)
	go func() {
		err := s.emailService.SendWelcomeEmail(authUser.Email, authUser.Username)
		if err != nil {
			log.Printf("⚠️  Failed to send welcome email to %s: %v", authUser.Email, err)
		}
	}()

	// Delete signup secret in production
	if config.Env.Environment != "DEVELOPMENT" && req.UserSignupSecret != nil {
		go func() {
			err := s.userRepo.DeleteUserSignupSecret(*req.UserSignupSecret)
			if err != nil {
				log.Printf("Failed to delete user signup secret: %v", err)
			}
		}()
	}

	// Create default chat in development mode
	if config.Env.Environment == "DEVELOPMENT" {
		chat, _, err := s.chatService.CreateWithoutConnectionPing(authUser.ID.Hex(), &dtos.CreateChatRequest{
			Connection: dtos.CreateConnectionRequest{
				Type:     config.Env.ExampleDatabaseType,
				Host:     config.Env.ExampleDatabaseHost,
				Port:     utils.StringPtr(config.Env.ExampleDatabasePort),
				Database: config.Env.ExampleDatabaseName,
				Username: config.Env.ExampleDatabaseUsername,
				Password: utils.StringPtr(config.Env.ExampleDatabasePassword),
			},
			Settings: dtos.CreateChatSettings{
				AutoExecuteQuery: utils.TruePtr(),
				ShareDataWithAI:  utils.FalsePtr(),
			},
		})
		if err != nil {
			log.Printf("Failed to create default chat for Google OAuth user: %v", err)
		} else if chat != nil {
			log.Printf("Default chat created for Google OAuth user: %s", chat.ID)
		}
	}

	log.Printf("Google OAuth: New user signed up successfully - Email: %s, Username: %s", authUser.Email, authUser.Username)

	// Generate JWT tokens
	accessToken, err := s.jwtService.GenerateToken(authUser.ID.Hex())
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate access token: %v", err)
	}

	jwtRefreshToken, err := s.jwtService.GenerateRefreshToken(authUser.ID.Hex())
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate refresh token: %v", err)
	}

	err = s.tokenRepo.StoreRefreshToken(authUser.ID.Hex(), *jwtRefreshToken)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to store refresh token: %v", err)
	}

	return &dtos.AuthResponse{
		AccessToken:  *accessToken,
		RefreshToken: *jwtRefreshToken,
		User:         *authUser,
	}, http.StatusOK, nil
}
