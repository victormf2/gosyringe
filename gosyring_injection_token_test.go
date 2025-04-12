package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInjectionTokens(t *testing.T) {
	t.Parallel()

	t.Run("should resolve different instances for different injection tokens", func(t *testing.T) {
		c := NewContainer()

		RegisterSingleton[TokenOne](c, NewTokenOne)
		RegisterSingleton[TokenTwo](c, NewTokenTwo)

		serviceOne, err := Resolve[TokenOne](c)
		assert.NoError(t, err)

		serviceTwo, err := Resolve[TokenTwo](c)
		assert.NoError(t, err)

		assert.Equal(t, 1, serviceOne.GetValue())
		assert.Equal(t, 2, serviceTwo.GetValue())
	})
}

type (
	TokenOne        IService
	TokenOneService struct{}
)

func (s *TokenOneService) GetValue() int {
	return 1
}

func NewTokenOne() TokenOne {
	return &TokenOneService{}
}

type (
	TokenTwo        IService
	TokenTwoService struct{}
)

func (s *TokenTwoService) GetValue() int {
	return 2
}

func NewTokenTwo() TokenTwo {
	return &TokenTwoService{}
}
