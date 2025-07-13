package handlers

import (
	"github.com/sirupsen/logrus"
	"github.com/victormf2/gosyringe/internal/examples/gin/repositories"
)

type DeleteUserHandler struct {
	logger         *logrus.Entry
	userRepository repositories.IUserRepository
}

func NewDeleteUserHandler(logger *logrus.Entry, userRepository repositories.IUserRepository) *DeleteUserHandler {
	return &DeleteUserHandler{
		logger:         logger,
		userRepository: userRepository,
	}
}

type DeleteUserInput struct {
	ID int64 `uri:"id" binding:"required"`
}

type DeleteUserOutput struct{}

func (h *DeleteUserHandler) Handle(input *DeleteUserInput) (*DeleteUserOutput, error) {
	h.logger.Infof("Deleting user")

	err := h.userRepository.Delete(input.ID)
	if err != nil {
		h.logger.Errorf("Failed to delete user")
		return nil, err
	}

	h.logger.Infof("User deleted")
	output := &DeleteUserOutput{}

	return output, nil
}
