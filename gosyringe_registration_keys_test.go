package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistrationKeys(t *testing.T) {
	t.Parallel()

	t.Run("singleton", func(t *testing.T) {
		t.Parallel()

		t.Run("should resolve same instance for same key", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingletonWithKey[IService](c, "service1", NewService)

			service1_1, err := ResolveWithKey[IService](c, "service1")
			require.NoError(t, err)
			service1_2, err := ResolveWithKey[IService](c, "service1")
			require.NoError(t, err)

			require.Same(t, service1_1, service1_2)
		})

		t.Run("should resolve different instances for different keys", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingletonWithKey[IService](c, "service1", NewService)
			RegisterSingletonWithKey[IService](c, "service2", NewService)

			service1, err := ResolveWithKey[IService](c, "service1")
			require.NoError(t, err)
			service2, err := ResolveWithKey[IService](c, "service2")
			require.NoError(t, err)

			require.NotSame(t, service1, service2)
		})
	})

	t.Run("scoped", func(t *testing.T) {
		t.Parallel()

		t.Run("should resolve same instance for same key", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterScopedWithKey[IService](c, "service1", NewService)

			c = CreateChildContainer(c)

			service1_1, err := ResolveWithKey[IService](c, "service1")
			require.NoError(t, err)
			service1_2, err := ResolveWithKey[IService](c, "service1")
			require.NoError(t, err)

			require.Same(t, service1_1, service1_2)
		})

		t.Run("should resolve different instances for different keys", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterScopedWithKey[IService](c, "service1", NewService)
			RegisterScopedWithKey[IService](c, "service2", NewService)

			c = CreateChildContainer(c)

			service1, err := ResolveWithKey[IService](c, "service1")
			require.NoError(t, err)
			service2, err := ResolveWithKey[IService](c, "service2")
			require.NoError(t, err)

			require.NotSame(t, service1, service2)
		})
	})
}
