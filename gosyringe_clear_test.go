package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClear(t *testing.T) {
	t.Parallel()

	t.Run("should clear all registrations of a type", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[IService](c, NewService)
		RegisterSingleton[IService](c, NewService)
		RegisterSingleton[IService](c, NewService)

		ClearRegistrations[IService](c)

		_, err := Resolve[IService](c)
		require.EqualError(t, err, "no constructor registered for type gosyringe.IService")
	})

	t.Run("should not clear other registrations", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[IServiceOne](c, NewServiceOne)
		RegisterSingleton[IServiceTwo](c, NewServiceTwo)

		ClearRegistrations[IServiceOne](c)

		_, err := Resolve[IServiceTwo](c)
		require.NoError(t, err)
	})

	t.Run("should clear all registrations of a key", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingletonWithKey[IService](c, "k0", NewService)
		RegisterSingletonWithKey[IService](c, "k0", NewService)
		RegisterSingletonWithKey[IService](c, "k0", NewService)

		ClearRegistrationsWithKey[IService](c, "k0")

		_, err := ResolveWithKey[IService](c, "k0")
		require.EqualError(t, err, "no constructor registered for type gosyringe.IService(k0)")
	})

	t.Run("should not clear other registrations with key", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingletonWithKey[IServiceOne](c, "k0", NewServiceOne)
		RegisterSingletonWithKey[IServiceOne](c, "k1", NewServiceOne)

		ClearRegistrationsWithKey[IServiceOne](c, "k0")

		_, err := ResolveWithKey[IServiceOne](c, "k1")
		require.NoError(t, err)
	})

	t.Run("should clear all resolved instances", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[IServiceOne](c, NewServiceOne)
		RegisterSingleton[IServiceTwo](c, NewServiceTwo)

		serviceOne, err := Resolve[IServiceOne](c)
		require.NoError(t, err)
		serviceTwo, err := Resolve[IServiceTwo](c)
		require.NoError(t, err)

		ClearInstances(c)

		serviceOneOther, err := Resolve[IServiceOne](c)
		require.NoError(t, err)
		serviceTwoOther, err := Resolve[IServiceTwo](c)
		require.NoError(t, err)

		require.NotSame(t, serviceOne, serviceOneOther)
		require.NotSame(t, serviceTwo, serviceTwoOther)
	})
}
