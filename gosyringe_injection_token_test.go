package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInjectionTokens(t *testing.T) {
	t.Parallel()

	t.Run("should resolve different instances for different injection tokens", func(t *testing.T) {
		c := NewContainer()

		RegisterSingleton[TokenOne](c, NewTokenOne)
		RegisterSingleton[TokenTwo](c, NewTokenTwo)

		serviceOne, err := Resolve[TokenOne](c)
		require.NoError(t, err)

		serviceTwo, err := Resolve[TokenTwo](c)
		require.NoError(t, err)

		require.Equal(t, 1, serviceOne.GetValue())
		require.Equal(t, 2, serviceTwo.GetValue())
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
