package models

import (
	"time"
)

type PasswordResetOTP struct {
	Email     string    `bson:"email" json:"email"`
	OTP       string    `bson:"otp" json:"otp"`
	ExpiresAt time.Time `bson:"expires_at" json:"expires_at"`
	Used      bool      `bson:"used" json:"used"`
	Base      `bson:",inline"`
}

func NewPasswordResetOTP(email, otp string) *PasswordResetOTP {
	return &PasswordResetOTP{
		Email:     email,
		OTP:       otp,
		ExpiresAt: time.Now().Add(10 * time.Minute), // OTP expires in 10 minutes
		Used:      false,
		Base:      NewBase(),
	}
}
