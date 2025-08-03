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
	"time"
)

type AuthService interface {
	Signup(req *dtos.SignupRequest) (*dtos.AuthResponse, uint, error)
	Login(req *dtos.LoginRequest) (*dtos.AuthResponse, uint, error)
	GenerateUserSignupSecret(req *dtos.UserSignupSecretRequest) (*models.UserSignupSecret, uint, error)
	RefreshToken(refreshToken string) (*dtos.RefreshTokenResponse, uint32, error)
	Logout(refreshToken string, accessToken string) (uint32, error)
	GetUser(userID string) (*models.User, uint, error)
	SetChatService(chatService ChatService)
}

type authService struct {
	chatService ChatService
	userRepo    repositories.UserRepository
	jwtService  utils.JWTService
	tokenRepo   repositories.TokenRepository
}

func NewAuthService(userRepo repositories.UserRepository, jwtService utils.JWTService, tokenRepo repositories.TokenRepository) AuthService {
	return &authService{
		userRepo:   userRepo,
		jwtService: jwtService,
		tokenRepo:  tokenRepo,
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
				Port:     utils.ToStringPtr(config.Env.ExampleDatabasePort),
				Database: config.Env.ExampleDatabaseName,
				Username: config.Env.ExampleDatabaseUsername,
				Password: utils.ToStringPtr(config.Env.ExampleDatabasePassword),
			},
			Settings: dtos.CreateChatSettings{
				AutoExecuteQuery: utils.ToBoolPtr(true),
				ShareDataWithAI:  utils.ToBoolPtr(false), // Disable sharing data with AI by default.
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
