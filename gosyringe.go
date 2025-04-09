package gosyringe

import (
	"fmt"
	"reflect"
	"sync"
)

type Container struct {
	mu        sync.Mutex
	registry  map[reflect.Type]reflect.Value
	instances map[reflect.Type]reflect.Value
}

func NewContainer() *Container {
	return &Container{
		registry:  make(map[reflect.Type]reflect.Value),
		instances: make(map[reflect.Type]reflect.Value),
	}
}

func RegisterSingleton(c *Container, constructor any) {
	reflectedConstructor := reflect.ValueOf(constructor)
	constructorType := reflectedConstructor.Type()

	if constructorType.Kind() != reflect.Func || constructorType.NumOut() < 1 || constructorType.NumOut() > 2 {
		panic("constructor must be a function returning exactly one value, or a value and an error")
	}

	if constructorType.NumOut() == 2 {
		errorType := constructorType.Out(1)
		if !errorType.AssignableTo(reflect.TypeFor[error]()) {
			panic("constructor must be a function returning exactly one value, or a value and an error")
		}
	}

	dependencyType := constructorType.Out(0)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.registry[dependencyType] = reflectedConstructor
}

func Resolve[T any](c *Container) (T, error) {
	typeOfT := reflect.TypeFor[T]()

	c.mu.Lock()
	defer c.mu.Unlock()

	instance, foundInstance := c.instances[typeOfT]
	if foundInstance {
		return instance.Interface().(T), nil
	}

	constructor, foundConstructor := c.registry[typeOfT]
	if !foundConstructor {
		var zero T
		return zero, fmt.Errorf("no constructor registered for type %v", typeOfT)
	}

	result := constructor.Call(nil)

	value := result[0]

	if len(result) == 1 {
		c.instances[typeOfT] = value
	} else {
		err := result[1].Interface().(error)
		if err != nil {
			var zero T
			return zero, fmt.Errorf("failed to resolve a value for type %v: %w", typeOfT, err)
		}
	}

	return value.Interface().(T), nil
}
