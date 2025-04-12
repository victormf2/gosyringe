package handlers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/victormf2/gosyringe"
	"github.com/victormf2/gosyringe/internal/examples/gin/handlers"
	"github.com/victormf2/gosyringe/internal/examples/gin/mocks"
	"github.com/victormf2/gosyringe/internal/examples/gin/repositories"
	"go.uber.org/mock/gomock"
)

func TestCreateUser(t *testing.T) {
	t.Parallel()

	root := gosyringe.NewContainer()
	mocks.RegisterTestServices(root)

	ctrl := gomock.NewController(t)

	t.Run("Test user creation", func(t *testing.T) {
		t.Parallel()

		c := gosyringe.CreateChildContainer(root)

		// See how you don't need to provide every dependency for CreateUserHandler
		// as they are already provided by the Container.
		//
		// You just have to override those you want to control in your test.
		mockUserRepository := mocks.NewMockIUserRepository(ctrl)
		gosyringe.RegisterValue[repositories.IUserRepository](c, mockUserRepository)
		mockUserRepository.EXPECT().Create(&repositories.User{
			Name:  "John Doe",
			Email: "john.doe@example.com",
		}).DoAndReturn(func(user *repositories.User) error {
			user.ID = 1
			return nil
		})

		handler, err := gosyringe.Resolve[*handlers.CreateUserHandler](c)
		assert.NoError(t, err)

		input := &handlers.CreateUserInput{
			Name:  "John Doe",
			Email: "john.doe@example.com",
		}
		out, err := handler.Handle(input)
		assert.NoError(t, err)

		assert.Equal(t, &handlers.CreateUserOutput{ID: 1}, out)
	})
}
