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
	c.registry[dependencyType] = reflectedConstructor
	resolveRegistry[dependencyType] = reflect.ValueOf(Resolve[T])
}

var resolveRegistry = map[reflect.Type]reflect.Value{}
var resolutionLocks = map[reflect.Type]*sync.Mutex{}

func Resolve[T any](c *Container) (T, error) {
	var zero T
	typeOfT := reflect.TypeFor[T]()

	c.mu.Lock()
	resolutionLock, found := resolutionLocks[typeOfT]
	if !found {
		resolutionLock = &sync.Mutex{}
		resolutionLocks[typeOfT] = resolutionLock
	}
	c.mu.Unlock()

	resolutionLock.Lock()
	defer resolutionLock.Unlock()

	instance, foundInstance := c.instances[typeOfT]
	if foundInstance {
		return instance.Interface().(T), nil
	}

	constructor, foundConstructor := c.registry[typeOfT]
	if !foundConstructor {
		return zero, fmt.Errorf("no constructor registered for type %v", typeOfT)
	}

	constructorType := constructor.Type()
	numberOfArguments := constructorType.NumIn()
	callArguments := make([]reflect.Value, numberOfArguments)
	for argumentIndex := range numberOfArguments {
		argumentType := constructorType.In(argumentIndex)
		reflectedResolve, found := resolveRegistry[argumentType]
		if !found {
			return zero, fmt.Errorf("failed to resolve argument %v of constructor of type %v: no constructor registered for type %v", argumentIndex, typeOfT, argumentType)
		}
		argumentResolutionResult := reflectedResolve.Call([]reflect.Value{reflect.ValueOf(c)})
		argumentValue := argumentResolutionResult[0]
		if len(argumentResolutionResult) == 2 {
			err := argumentResolutionResult[1].Interface()
			if err != nil {
				return zero, fmt.Errorf("failed to resolve argument %v of constructor of type %v: %w", argumentIndex, typeOfT, err.(error))
			}
		}
		callArguments[argumentIndex] = argumentValue
	}

	resolutionResult := constructor.Call(callArguments)

	value := resolutionResult[0]

	if len(resolutionResult) == 2 {
		err := resolutionResult[1].Interface()
		if err != nil {
			return zero, fmt.Errorf("failed to resolve a value for type %v: %w", typeOfT, err.(error))
		}
	}
	c.instances[typeOfT] = value

	return value.Interface().(T), nil
}
