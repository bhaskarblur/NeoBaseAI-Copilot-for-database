package services

import (
	"fmt"
	"io/ioutil"
	"log"
	"neobase-ai/config"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type EmailService interface {
	SendEmail(to, subject, body string) error
	SendPasswordResetOTP(email, username, otp string) error
	SendWelcomeEmail(email, username string) error
	SendEnterpriseWaitlistEmail(email string) error
	TestConnection() error
}

type emailService struct {
	smtpHost     string
	smtpPort     int
	smtpUser     string
	smtpPassword string
	fromName     string
	fromEmail    string
	smtpSecure   bool
}

func NewEmailService() EmailService {
	service := &emailService{
		smtpHost:     config.Env.SMTPHost,
		smtpPort:     config.Env.SMTPPort,
		smtpUser:     config.Env.SMTPUser,
		smtpPassword: config.Env.SMTPPassword,
		fromName:     config.Env.SMTPFromName,
		fromEmail:    config.Env.SMTPFromEmail,
		smtpSecure:   config.Env.SMTPSecure,
	}

	// Check if SMTP configuration is missing or contains default/placeholder values
	if service.isConfigurationMissing() {
		log.Println("⚠️  SMTP not configured properly. Email features will be disabled.")
		log.Println("   Configure the following environment variables to enable email functionality:")
		log.Println("   - SMTP_HOST (e.g., smtp.gmail.com)")
		log.Println("   - SMTP_USER (your email address)")
		log.Println("   - SMTP_PASSWORD (your app password)")
		log.Println("   - SMTP_FROM_NAME (sender display name)")
		log.Println("   - SMTP_FROM_EMAIL (sender email address)")
	} else {
		log.Println("✅ SMTP Client Setup Successfully")
		log.Printf("   📧 Email: %s", service.smtpUser)
		log.Printf("   👤 Name: %s", service.fromName)
		log.Printf("   🌐 Host: %s:%d", service.smtpHost, service.smtpPort)
		log.Printf("   🔒 Secure: %t", service.smtpSecure)
	}

	return service
}

// isConfigurationMissing checks if SMTP configuration is missing or contains default/placeholder values
func (s *emailService) isConfigurationMissing() bool {
	// Check for empty values
	if s.smtpHost == "" || s.smtpUser == "" || s.smtpPassword == "" {
		return true
	}

	// Check for common default/placeholder values
	defaultValues := []string{
		"your-email@gmail.com",
		"your-app-password",
		"your-gmail-app-password",
		"example@gmail.com",
		"placeholder",
		"change-me",
		"your-password",
		"your-smtp-password",
	}

	// Check SMTP user for default values
	for _, defaultValue := range defaultValues {
		if strings.ToLower(s.smtpUser) == strings.ToLower(defaultValue) {
			return true
		}
	}

	// Check SMTP password for default values
	for _, defaultValue := range defaultValues {
		if strings.ToLower(s.smtpPassword) == strings.ToLower(defaultValue) {
			return true
		}
	}

	// Check if from email is still a placeholder
	for _, defaultValue := range defaultValues {
		if strings.ToLower(s.fromEmail) == strings.ToLower(defaultValue) {
			return true
		}
	}

	return false
}

func (s *emailService) SendEmail(to, subject, body string) error {
	if s.isConfigurationMissing() {
		log.Printf("⚠️  SMTP configuration missing or contains default values. Email to %s not sent.", to)
		return nil // Return nil to not block the application flow
	}

	// Create properly formatted sender with display name
	from := fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)

	// Create message
	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, from, subject, body))

	// SMTP server configuration
	smtpAddr := fmt.Sprintf("%s:%d", s.smtpHost, s.smtpPort)
	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPassword, s.smtpHost)

	// Send email
	err := smtp.SendMail(smtpAddr, auth, s.fromEmail, []string{to}, msg)
	if err != nil {
		log.Printf("⚠️  Failed to send email to %s: %v", to, err)
		return nil // Return nil to not block the application flow
	}

	log.Printf("✅ Email sent successfully to %s", to)
	return nil
}

func (s *emailService) SendPasswordResetOTP(email, username, otp string) error {
	subject := "Reset Your NeoBase Password"

	// Load and process template
	body, err := s.loadTemplate("password_reset", map[string]string{
		"username": username,
		"otp":      otp,
	})
	if err != nil {
		log.Printf("⚠️  Failed to load password reset template: %v", err)
		return nil // Return nil to not block the application flow
	}

	return s.SendEmail(email, subject, body)
}

func (s *emailService) SendWelcomeEmail(email, username string) error {
	subject := "Welcome to NeoBase - Your AI Database Copilot!"

	// Load and process template
	body, err := s.loadTemplate("welcome", map[string]string{
		"username": username,
	})
	if err != nil {
		log.Printf("⚠️  Failed to load welcome template: %v", err)
		return nil // Return nil to not block the application flow
	}

	return s.SendEmail(email, subject, body)
}

func (s *emailService) SendEnterpriseWaitlistEmail(email string) error {
	subject := "You're on the NeoBase Enterprise Waitlist! 🚀"

	// Load and process template
	body, err := s.loadTemplate("enterprise_waitlist", map[string]string{})
	if err != nil {
		log.Printf("⚠️  Failed to load enterprise waitlist template: %v", err)
		return nil // Return nil to not block the application flow
	}

	return s.SendEmail(email, subject, body)
}

// loadTemplate loads an HTML template file and replaces placeholders with actual values
func (s *emailService) loadTemplate(templateName string, placeholders map[string]string) (string, error) {
	// Get current working directory for debugging
	cwd, _ := os.Getwd()
	log.Printf("📁 Current working directory: %s", cwd)

	// Check if we're running in Docker (working directory is /app)
	isDocker := cwd == "/app"

	// Build possible paths based on environment
	var possiblePaths []string

	if isDocker {
		// Docker environment paths
		possiblePaths = []string{
			filepath.Join("/app", "internal", "email_templates", templateName+".html"),
			filepath.Join("internal", "email_templates", templateName+".html"),
		}
	} else {
		// Local development paths
		possiblePaths = []string{
			filepath.Join("internal", "email_templates", templateName+".html"),
			filepath.Join(cwd, "internal", "email_templates", templateName+".html"),
			// If running from project root instead of backend directory
			filepath.Join("backend", "internal", "email_templates", templateName+".html"),
			filepath.Join(cwd, "backend", "internal", "email_templates", templateName+".html"),
		}
	}

	var templateBytes []byte
	var err error
	var templatePath string

	// Try each possible path
	for _, path := range possiblePaths {
		templateBytes, err = ioutil.ReadFile(path)
		if err == nil {
			templatePath = path
			log.Printf("✅ Successfully loaded template from: %s", templatePath)
			break
		}
	}

	// If all paths failed, use fallback
	if err != nil {
		log.Printf("⚠️  Failed to read template file from any of the following paths:")
		for _, path := range possiblePaths {
			log.Printf("   - %s", path)
		}
		log.Printf("⚠️  Using fallback template for %s", templateName)
		// Return a simple fallback template
		return s.createFallbackTemplate(templateName, placeholders), nil
	}

	// Convert to string
	template := string(templateBytes)

	// Replace placeholders with actual values
	for placeholder, value := range placeholders {
		template = strings.ReplaceAll(template, "{{"+placeholder+"}}", value)
	}

	return template, nil
}

// createFallbackTemplate creates a simple HTML template when the main template fails to load
func (s *emailService) createFallbackTemplate(templateName string, placeholders map[string]string) string {
	// Base styles matching the design system
	baseStyles := `
		body { 
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; 
			max-width: 600px; 
			margin: 0 auto; 
			padding: 20px; 
			background-color: #fef3c7; 
			color: #374151; 
		}
		.container { 
			background-color: #ffffff; 
			border: 3px solid #000000; 
			border-radius: 12px; 
			box-shadow: 6px 6px 0px #000000; 
			padding: 30px; 
		}
		.logo { 
			font-size: 28px; 
			font-weight: bold; 
			color: #000000; 
			text-align: center; 
			margin-bottom: 20px; 
		}
		.otp-code { 
			font-size: 32px; 
			font-weight: bold; 
			color: #10b981; 
			text-align: center; 
			background-color: #f8fafc; 
			border: 3px dashed #10b981; 
			border-radius: 12px; 
			padding: 20px; 
			margin: 20px 0; 
			letter-spacing: 4px; 
		}
	`

	switch templateName {
	case "password_reset":
		username := placeholders["username"]
		otp := placeholders["otp"]
		return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>Reset Your NeoBase Password</title>
	<style>%s</style>
</head>
<body>
	<div class="container">
		<div class="logo">NeoBase</div>
		<h2>Password Reset Request</h2>
		<p>Hello <strong>%s</strong>,</p>
		<p>We received a request to reset your NeoBase password. Use the OTP code below:</p>
		<div class="otp-code">%s</div>
		<p><strong>⚠️ Security Notice:</strong></p>
		<ul>
			<li>This OTP is valid for 10 minutes only</li>
			<li>Don't share this code with anyone</li>
			<li>If you didn't request this reset, please ignore this email</li>
		</ul>
		<p>Best regards,<br><strong>The NeoBase Team</strong></p>
	</div>
</body>
</html>`, baseStyles, username, otp)
	case "welcome":
		username := placeholders["username"]
		return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>Welcome to NeoBase</title>
	<style>%s</style>
</head>
<body>
	<div class="container">
		<div class="logo">NeoBase</div>
		<h2>Welcome to NeoBase! 🎉</h2>
		<p>Hello <strong>%s</strong>,</p>
		<p>Welcome to NeoBase, your AI Database Copilot! We're thrilled to have you join our community.</p>
		<p>NeoBase transforms complex database operations into simple, natural language conversations.</p>
		<p><strong>Get started by exploring:</strong></p>
		<ul>
			<li>Natural Language Queries: Ask questions in plain English</li>
			<li>Multi-Database Support: Connect to PostgreSQL, MySQL, MongoDB, and more</li>
			<li>AI-Powered Analysis: Get insights from your data</li>
		</ul>
		<p>Best regards,<br><strong>The NeoBase Team</strong></p>
	</div>
</body>
</html>`, baseStyles, username)
	default:
		return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>NeoBase Notification</title>
	<style>%s</style>
</head>
<body>
	<div class="container">
		<div class="logo">NeoBase</div>
		<p>You have received a notification from NeoBase.</p>
		<p>Best regards,<br><strong>The NeoBase Team</strong></p>
	</div>
</body>
</html>`, baseStyles)
	}
}

// applyTemplate applies template substitution with enhanced support for complex patterns
func (s *emailService) applyTemplate(template string, data map[string]string) string {
	result := template

	// Helper function to safely convert values to strings
	safeString := func(value interface{}) string {
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	}

	// Replace simple variable substitutions {{variableName}}
	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, safeString(value))
	}

	return result
}

func (s *emailService) TestConnection() error {
	if s.isConfigurationMissing() {
		log.Println("⚠️  SMTP configuration missing or contains default values. Cannot test connection.")
		return nil // Return nil to not block the application flow
	}

	// Test SMTP connection
	smtpAddr := fmt.Sprintf("%s:%s", s.smtpHost, strconv.Itoa(s.smtpPort))
	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPassword, s.smtpHost)

	// Try to connect and authenticate
	client, err := smtp.Dial(smtpAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		log.Printf("⚠️  SMTP authentication failed: %v", err)
		return nil // Return nil to not block the application flow
	}

	log.Println("✅ SMTP connection test successful")
	return nil
}
