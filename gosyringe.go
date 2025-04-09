package gosyringe

import (
	"fmt"
	"reflect"
	"sync"
)

type Container struct {
	mu              sync.Mutex
	registry        map[reflect.Type][]reflect.Value
	instances       map[reflect.Type]reflect.Value
	resolutionLocks map[reflect.Type]*sync.Mutex
}

func NewContainer() *Container {
	return &Container{
		registry:        make(map[reflect.Type][]reflect.Value),
		instances:       make(map[reflect.Type]reflect.Value),
		resolutionLocks: map[reflect.Type]*sync.Mutex{},
	}
}

var mu sync.Mutex
var resolveRegistry = map[reflect.Type]reflect.Value{}

func RegisterSingleton[T any](c *Container, constructor any) {
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
	registerType := reflect.TypeFor[T]()

	if dependencyType != registerType {
		panic(fmt.Sprintf("the type parameter %v is not the same as the return type of the constructor %v", registerType, dependencyType))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.registry[dependencyType] == nil {
		c.registry[dependencyType] = []reflect.Value{}
	}
	c.registry[dependencyType] = append(c.registry[dependencyType], reflectedConstructor)

	mu.Lock()
	defer mu.Unlock()

	resolveRegistry[dependencyType] = reflect.ValueOf(Resolve[T])
	resolveRegistry[reflect.SliceOf(dependencyType)] = reflect.ValueOf(Resolve[[]T])
}

func Resolve[T any](c *Container) (T, error) {
	var zero T
	resolutionType := reflect.TypeFor[T]()
	isSlice := resolutionType.Kind() == reflect.Slice
	elementType := resolutionType
	if isSlice {
		elementType = resolutionType.Elem()
	}

	c.mu.Lock()
	resolutionLock, found := c.resolutionLocks[resolutionType]
	if !found {
		resolutionLock = &sync.Mutex{}
		c.resolutionLocks[resolutionType] = resolutionLock
	}
	c.mu.Unlock()

	resolutionLock.Lock()
	defer resolutionLock.Unlock()

	instance, foundInstance := c.instances[resolutionType]
	if foundInstance {
		return instance.Interface().(T), nil
	}

	constructors, foundConstructors := c.registry[elementType]
	if !foundConstructors || len(constructors) == 0 {
		return zero, fmt.Errorf("no constructor registered for type %v", elementType)
	}

	if isSlice {
		sliceValue := reflect.MakeSlice(resolutionType, len(constructors), len(constructors))
		for constructorIndex, constructor := range constructors {
			value, err := resolveSingle(c, constructor, elementType)
			if err != nil {
				return zero, err
			}

			sliceValue.Index(constructorIndex).Set(value)
		}

		c.instances[resolutionType] = sliceValue

		return sliceValue.Interface().(T), nil
	} else {
		constructor := constructors[len(constructors)-1]

		value, err := resolveSingle(c, constructor, elementType)
		if err != nil {
			return zero, err
		}

		c.instances[resolutionType] = value

		return value.Interface().(T), nil
	}
}

func resolveSingle(c *Container, constructor reflect.Value, elementType reflect.Type) (reflect.Value, error) {
	var zero reflect.Value
	constructorType := constructor.Type()
	if constructorType.Kind() != reflect.Func {
		panic("constructor should be a function")
	}
	numberOfArguments := constructorType.NumIn()
	callArguments := make([]reflect.Value, numberOfArguments)
	for argumentIndex := range numberOfArguments {
		argumentType := constructorType.In(argumentIndex)
		reflectedResolve, found := resolveRegistry[argumentType]
		if !found {
			return zero, fmt.Errorf("failed to resolve argument %v of constructor of type %v: no constructor registered for type %v", argumentIndex, elementType, argumentType)
		}
		argumentResolutionResult := reflectedResolve.Call([]reflect.Value{reflect.ValueOf(c)})
		argumentValue := argumentResolutionResult[0]
		if len(argumentResolutionResult) == 2 {
			err := argumentResolutionResult[1].Interface()
			if err != nil {
				return zero, fmt.Errorf("failed to resolve argument %v of constructor of type %v: %w", argumentIndex, elementType, err.(error))
			}
		}
		callArguments[argumentIndex] = argumentValue
	}

	resolutionResult := constructor.Call(callArguments)

	value := resolutionResult[0]

	if len(resolutionResult) == 2 {
		err := resolutionResult[1].Interface()
		if err != nil {
			return zero, fmt.Errorf("failed to resolve a value for type %v: %w", elementType, err.(error))
		}
	}

	return value, nil
}
