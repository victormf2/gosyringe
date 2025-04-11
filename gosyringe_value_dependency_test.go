package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValueDependency(t *testing.T) {
	t.Parallel()

	t.Run("should resolve same value in root container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		injectedValue := NewService()
		RegisterValue(c, injectedValue)

		value, err := Resolve[IService](c)
		assert.NoError(t, err)

		assert.True(t, assert.ObjectsAreEqual(injectedValue, value))
	})

	t.Run("should resolve same value in child container", func(t *testing.T) {
		t.Parallel()

		cRoot := NewContainer()
		RegisterValue(cRoot, NewService())

		injectedValue := NewService()
		c := CreateChildContainer(cRoot)
		RegisterValue(c, injectedValue)

		value, err := Resolve[IService](c)
		assert.NoError(t, err)

		assert.True(t, assert.ObjectsAreEqual(injectedValue, value))
	})
}
