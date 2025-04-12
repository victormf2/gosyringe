package handlers

import (
	"github.com/sirupsen/logrus"
	"github.com/victormf2/gosyringe/internal/examples/gin/repositories"
)

type CreateUserHandler struct {
	logger         *logrus.Entry
	userRepository repositories.IUserRepository
}

func NewCreateUserHandler(logger *logrus.Entry, userRepository repositories.IUserRepository) *CreateUserHandler {
	return &CreateUserHandler{
		logger:         logger,
		userRepository: userRepository,
	}
}

type CreateUserInput struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

type CreateUserOutput struct {
	ID int64 `json:"id"`
}

func (h *CreateUserHandler) Handle(input *CreateUserInput) (*CreateUserOutput, error) {
	h.logger.Infof("Creating user")
	user := &repositories.User{
		Name:  input.Name,
		Email: input.Email,
	}
	err := h.userRepository.Create(user)
	if err != nil {
		h.logger.Errorf("Failed to create user")
		return nil, err
	}

	h.logger.Errorf("User created")
	output := &CreateUserOutput{
		ID: user.ID,
	}

	return output, nil
}
