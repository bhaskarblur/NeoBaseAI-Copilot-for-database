package utils

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func GenerateSecret() string {
	return uuid.New().String()
}

func GenerateOTP() string {
	rand.Seed(time.Now().UnixNano())
	otp := rand.Intn(900000) + 100000 // Generate 6-digit number between 100000-999999
	return strconv.Itoa(otp)
}
