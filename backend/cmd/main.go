package main

import (
	"context"
	"fmt"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/routes"
	"neobase-ai/internal/di"
	"neobase-ai/internal/middleware"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load environment variables
	err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Failed to load environment variables: %v", err)
	}

	// Initialize dependencies
	di.Initialize()

	// Setup Gin
	ginApp := gin.New() // Use gin.New() instead of gin.Default()

	// Add custom recovery middleware
	ginApp.Use(middleware.CustomRecoveryMiddleware())

	// Add logging middleware
	ginApp.Use(gin.Logger())

	// Add CORS middleware
	// Build allowed origins list
	allowedOrigins := []string{config.Env.CorsAllowedOrigin}
	if config.Env.LandingPageCorsAllowedOrigin != "" {
		allowedOrigins = append(allowedOrigins, config.Env.LandingPageCorsAllowedOrigin)
	}

	// CORS
	ginApp.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"User-Agent",
			"Referer",
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-platform",
			"Access-Control-Allow-Origin",
			"Access-Control-Allow-Credentials",
		},
		ExposeHeaders:    []string{"Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Setup routes
	routes.SetupDefaultRoutes(ginApp)

	// Create server
	srv := &http.Server{
		Addr:    ":" + config.Env.Port,
		Handler: ginApp,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", config.Env.Port)
		fmt.Printf("✨ Welcome to NeoBase! Running in %s Mode\n", config.Env.Environment)
		fmt.Printf("📱 Client UI: %s\n", config.Env.CorsAllowedOrigin)
		if config.Env.LandingPageCorsAllowedOrigin != "" {
			fmt.Printf("🌐 Landing Page: %s\n", config.Env.LandingPageCorsAllowedOrigin)
		}
		fmt.Printf("🚀 CORS Origins: %v\n", allowedOrigins)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("NeoBase failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🔻 NeoBase is shutting down...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("NeoBase forced to shutdown: %v", err)
	}

	log.Println("👋 NeoBase has been shut down successfully")
}
