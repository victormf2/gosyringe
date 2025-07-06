package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueDependency(t *testing.T) {
	t.Parallel()

	t.Run("should resolve same value in root container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		injectedValue := NewService()
		RegisterValue(c, injectedValue)

		value, err := Resolve[IService](c)
		require.NoError(t, err)

		require.Same(t, injectedValue, value)
	})

	t.Run("should resolve same value in child container", func(t *testing.T) {
		t.Parallel()

		cRoot := NewContainer()
		RegisterValue(cRoot, NewService())

		injectedValue := NewService()
		c := CreateChildContainer(cRoot)
		RegisterValue(c, injectedValue)

		value, err := Resolve[IService](c)
		require.NoError(t, err)

		require.Same(t, injectedValue, value)
	})
}
