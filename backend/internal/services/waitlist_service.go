package services

import (
	"context"
	"fmt"

	"neobase-ai/internal/repositories"
)

type WaitlistService struct {
	waitlistRepo *repositories.WaitlistRepository
	emailService EmailService
}

func NewWaitlistService(waitlistRepo *repositories.WaitlistRepository, emailService EmailService) *WaitlistService {
	return &WaitlistService{
		waitlistRepo: waitlistRepo,
		emailService: emailService,
	}
}

func (s *WaitlistService) AddToWaitlist(ctx context.Context, email string) error {
	// Check if email already exists on waitlist
	existing, err := s.waitlistRepo.GetByEmail(ctx, email)
	if err == nil && existing != nil {
		// User is already on the waitlist
		return fmt.Errorf("you're already on the waitlist")
	}

	// Add to waitlist database
	_, err = s.waitlistRepo.AddToWaitlist(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to add to waitlist: %w", err)
	}

	// Send enterprise waitlist email
	err = s.emailService.SendEnterpriseWaitlistEmail(email)
	if err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Failed to send waitlist email to %s: %v\n", email, err)
	}

	return nil
}
