package handlers

import (
	"net/http"
	"strings"

	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"

	"github.com/gin-gonic/gin"
)

type WaitlistHandler struct {
	waitlistService *services.WaitlistService
}

func NewWaitlistHandler(waitlistService *services.WaitlistService) *WaitlistHandler {
	return &WaitlistHandler{
		waitlistService: waitlistService,
	}
}

type WaitlistRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *WaitlistHandler) AddToWaitlist(c *gin.Context) {
	var req WaitlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Add to waitlist
	err := h.waitlistService.AddToWaitlist(c.Request.Context(), email)
	if err != nil {
		// Check if it's a duplicate email error
		if strings.Contains(err.Error(), "already on the waitlist") {
			errorMsg := "You're already on the NeoBase Enterprise waitlist! We'll notify you when it's available."
			c.JSON(http.StatusConflict, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			return
		}

		// Generic error for other cases
		errorMsg := "Failed to add to waitlist. Please try again."
		c.JSON(http.StatusInternalServerError, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data: gin.H{
			"message": "Successfully added to waitlist",
			"email":   email,
		},
	})
}
