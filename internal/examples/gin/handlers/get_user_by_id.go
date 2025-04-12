package handlers

import (
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/victormf2/gosyringe/internal/examples/gin/repositories"
)

type GetUserByIDHandler struct {
	logger         *logrus.Entry
	userRepository repositories.IUserRepository
}

func NewGetUserByIDHandler(logger *logrus.Entry, userRepository repositories.IUserRepository) *GetUserByIDHandler {
	return &GetUserByIDHandler{
		logger:         logger,
		userRepository: userRepository,
	}
}

type GetUserByIDInput struct {
	ID int64 `uri:"id" binding:"required"`
}

type GetUserByIDOutput struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

var ErrUserNotFound = errors.New("user not found")

func (h *GetUserByIDHandler) Handle(input *GetUserByIDInput) (*GetUserByIDOutput, error) {
	h.logger.Infof("Finding user")

	user, err := h.userRepository.GetByID(input.ID)
	if err != nil {
		h.logger.Errorf("Failed to get user")
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	h.logger.Errorf("User found")
	output := &GetUserByIDOutput{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}

	return output, nil
}
