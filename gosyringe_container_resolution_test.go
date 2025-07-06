package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainerResolution(t *testing.T) {
	t.Parallel()

	t.Run("should resolve root container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[ServiceWithContainer](c, NewServiceWithContainer)

		service, err := Resolve[ServiceWithContainer](c)
		require.NoError(t, err)

		container, err := Resolve[*Container](c)
		require.NoError(t, err)

		require.Same(t, c, service.container)
		require.Same(t, c, container)
	})

	t.Run("should resolve child container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterScoped[ServiceWithContainer](c, NewServiceWithContainer)

		childContainer := CreateChildContainer(c)

		service, err := Resolve[ServiceWithContainer](childContainer)
		require.NoError(t, err)

		container, err := Resolve[*Container](childContainer)
		require.NoError(t, err)

		require.Same(t, childContainer, service.container)
		require.Same(t, childContainer, container)
		require.NotSame(t, childContainer, c)
	})
}

type ServiceWithContainer struct {
	container *Container
}

func NewServiceWithContainer(c *Container) ServiceWithContainer {
	return ServiceWithContainer{
		container: c,
	}
}
