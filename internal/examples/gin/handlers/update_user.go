package handlers

import (
	"github.com/sirupsen/logrus"
	"github.com/victormf2/gosyringe/internal/examples/gin/repositories"
)

type UpdateUserHandler struct {
	logger         *logrus.Entry
	userRepository repositories.IUserRepository
}

func NewUpdateUserHandler(logger *logrus.Entry, userRepository repositories.IUserRepository) *UpdateUserHandler {
	return &UpdateUserHandler{
		logger:         logger,
		userRepository: userRepository,
	}
}

type UpdateUserInput struct {
	ID    int64  `json:"id" binding:"required"`
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

type UpdateUserOutput struct{}

func (h *UpdateUserHandler) Handle(input *UpdateUserInput) (*UpdateUserOutput, error) {
	h.logger.Infof("Updating user")
	user := &repositories.User{
		ID:    input.ID,
		Name:  input.Name,
		Email: input.Email,
	}
	err := h.userRepository.Update(user)
	if err != nil {
		h.logger.Errorf("Failed to update user")
		return nil, err
	}

	h.logger.Errorf("User updated")
	output := &UpdateUserOutput{}

	return output, nil
}
