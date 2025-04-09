package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainerResolution(t *testing.T) {
	t.Parallel()

	t.Run("should resolve root container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[ServiceWithContainer](c, NewServiceWithContainer)

		service, err := Resolve[ServiceWithContainer](c)
		assert.NoError(t, err)

		container, err := Resolve[*Container](c)
		assert.NoError(t, err)

		assert.True(t, assert.ObjectsAreEqual(c, service.container))
		assert.True(t, assert.ObjectsAreEqual(c, container))
	})

	t.Run("should resolve child container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterScoped[ServiceWithContainer](c, NewServiceWithContainer)

		childContainer := CreateChildContainer(c)

		service, err := Resolve[ServiceWithContainer](childContainer)
		assert.NoError(t, err)

		container, err := Resolve[*Container](childContainer)
		assert.NoError(t, err)

		assert.True(t, assert.ObjectsAreEqual(childContainer, service.container))
		assert.True(t, assert.ObjectsAreEqual(childContainer, container))
		assert.False(t, assert.ObjectsAreEqual(childContainer, c))
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
