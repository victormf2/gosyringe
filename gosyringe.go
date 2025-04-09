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

	if constructorType.Kind() != reflect.Func || constructorType.NumOut() != 1 {
		panic("constructor must be a function returning exactly one value")
	}

	dependencyType := constructorType.Out(0)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.registry[dependencyType] = reflectedConstructor
}

func Resolve[T any](c *Container) (T, error) {
	typeOfT := reflect.TypeOf((*T)(nil)).Elem()

	c.mu.Lock()
	defer c.mu.Unlock()

	instance, found := c.instances[typeOfT]
	if found {
		return instance.Interface().(T), nil
	}

	constructor, ok := c.registry[typeOfT]
	if !ok {
		var zero T
		return zero, fmt.Errorf("no constructor registered for type %v", typeOfT)
	}

	result := constructor.Call(nil)[0]
	c.instances[typeOfT] = result

	return result.Interface().(T), nil
}
