package gosyringe

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/victormf2/gosyringe/internal"
)

// Container is the main object to register and resolve dependencies.
//
// As you can see, the API is exposed as function calls instead of
// methods. This is because Go has a limitation with generics and
// does not allow methods with generic parameters.
//
// You can read more about it here:
// https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#No-parameterized-methods
type Container struct {
	// The container mutex is to control concurrency over registrations and resolutions.
	mu sync.Mutex
	// Here is where the constructors are registered.
	//
	// We allow multiple constructors per resolution type, because we want to be able
	// to resolve slices of dependencies of the same type, and to override registrations
	// as well.
	registry map[reflect.Type][]reflect.Value
	// Here is where the resolved instances are stored. You can think of it as a cache.
	// Singleton and Scoped resolutions rely on this to work.
	instances map[reflect.Type]reflect.Value
	// Resolution locks are necessary because nested calls to resolve results in a dead lock
	// if we use the container mutex only.
	resolutionLocks map[reflect.Type]*sync.Mutex
}

// Instantiates a new root Container.
func NewContainer() *Container {
	return &Container{
		registry:        make(map[reflect.Type][]reflect.Value),
		instances:       make(map[reflect.Type]reflect.Value),
		resolutionLocks: map[reflect.Type]*sync.Mutex{},
	}
}

// Global mutex to control concurrency over resolve functions registry.
var mu sync.Mutex

// Go does not provide a way to dynamically instantiate a generic function.
// All generic function calls must be done statically.
//
// But we can overcome this
// limitation by storing the references of the functions and retrieving them later
// by their reflection type. This is also why we need to provide a type
// parameter in the register functions.
var resolveFunctionsRegistry = map[reflect.Type]reflect.Value{}

// Registers a dependency for the type T as a Singleton, providing a constructor as resolution method.
//
// A Resolve call to a dependency registered as Singleton is guaranteed to return
// the same instance in the Container lifetime.
//
// constructor must be a function returning exactly one value or a (value, error) tuple.
// The type of the return value must be exactly equal to the type parameter T.
//
// If this is called multiple times, the last registration is considered for resolution.
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

	// Here we lock on the container to prevent multiple go routines to
	// write at the same time to the dependency registry
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.registry[dependencyType] == nil {
		c.registry[dependencyType] = []reflect.Value{}
	}
	c.registry[dependencyType] = append(c.registry[dependencyType], reflectedConstructor)

	// Here we lock globally, because the resolveFunctionsRegistry is global.
	// We use the resolveFunctionsRegistry to retrieve the generic function instance
	// at resolution time, in a context we cannot determine the type statically.
	// We assume only registered types will be resolved.
	mu.Lock()
	defer mu.Unlock()

	// We need to register both dependencyType and slice of dependencyType,
	// because we want to allow the user to call Resolve[T] an Resolve[[]T]
	resolveFunctionsRegistry[dependencyType] = reflect.ValueOf(resolve[T])
	resolveFunctionsRegistry[reflect.SliceOf(dependencyType)] = reflect.ValueOf(resolve[[]T])
}

// Calls the constructor registered for the type T. If T was registered as Singleton or Scoped,
// then retrieves instance from the cache.
//
// If T is a slice type []E, then resolves all registered dependencies for E.
func Resolve[T any](c *Container) (T, error) {
	resolutionContext := resolutionContext{
		stack: []reflect.Type{},
	}
	return resolve[T](c, resolutionContext)
}

// Tracks information about undergoing Resolve call and nested resolutions.
type resolutionContext struct {
	stack []reflect.Type
}

func (rc resolutionContext) dependencyGraphString(cyclic bool) string {
	dependencyGraphNodes := rc.stack
	if cyclic {
		// adding first element of the stack to the string to make the cyclic dependency graph clear
		dependencyGraphNodes = append(dependencyGraphNodes, rc.stack[0])
	}
	typesString := internal.Map(
		dependencyGraphNodes,
		func(t reflect.Type) string { return fmt.Sprintf("%v", t) },
	)
	return strings.Join(typesString, " -> ")
}

func (rc resolutionContext) push(resolutionType reflect.Type) resolutionContext {
	return resolutionContext{
		stack: append(rc.stack, resolutionType),
	}
}

func resolve[T any](c *Container, parentResolutionContext resolutionContext) (T, error) {
	var zero T // small trick since x := T{} is not possible
	resolutionType := reflect.TypeFor[T]()
	isCyclic := slices.Contains(parentResolutionContext.stack, resolutionType)
	if isCyclic {
		// We know it's a circular dependency because resolve was already called for type T
		// up in the stack. We decided to not handle it yet and just return an error.
		//
		// Circular dependencies can be resolved by providing some kind of lazy evaluation,
		// but it's too complex, and it's not in our plan right now.
		return zero, fmt.Errorf("circular dependency detected: %s", parentResolutionContext.dependencyGraphString(true))
	}

	currentResolutionContext := parentResolutionContext.push(resolutionType)
	isSlice := resolutionType.Kind() == reflect.Slice
	elementType := resolutionType
	if isSlice {
		elementType = resolutionType.Elem()
	}

	// Locking on container mutex to prevent simultaneous access to
	// the resolutionLocks list.
	c.mu.Lock()
	resolutionLock, found := c.resolutionLocks[resolutionType]
	if !found {
		resolutionLock = &sync.Mutex{}
		c.resolutionLocks[resolutionType] = resolutionLock
	}
	c.mu.Unlock()

	// By acquiring a lock per resolution type we prevent dead locks
	// on 2+ level deep dependency resolution
	resolutionLock.Lock()
	defer resolutionLock.Unlock()

	// Checking if an instance is already in the cache
	instance, foundInstance := c.instances[resolutionType]
	if foundInstance {
		return instance.Interface().(T), nil
	}

	// Checking if there is a registration for elementType.
	// Here elementType is used instead of resolutionType, because we
	// don't expect calls to RegisterXxx[[]T].
	constructors, foundConstructors := c.registry[elementType]
	if !foundConstructors || len(constructors) == 0 {
		return zero, fmt.Errorf("no constructor registered for type %v", elementType)
	}

	if isSlice {
		// When resolutionType is a slice, we resolve all registered dependencies for it,
		// then return the result for it only if all are successful. Failing fast for simplicity.
		sliceValue := reflect.MakeSlice(resolutionType, len(constructors), len(constructors))
		for constructorIndex, constructor := range constructors {
			value, err := resolveSingle(c, constructor, elementType, currentResolutionContext)
			if err != nil {
				return zero, err
			}

			sliceValue.Index(constructorIndex).Set(value)
		}

		c.instances[resolutionType] = sliceValue

		return sliceValue.Interface().(T), nil
	} else {
		// When resolutionType is not a slice, then resolve using the last registration.
		constructor := constructors[len(constructors)-1]

		value, err := resolveSingle(c, constructor, elementType, currentResolutionContext)
		if err != nil {
			return zero, err
		}

		c.instances[resolutionType] = value

		return value.Interface().(T), nil
	}
}

// Here is where the reflection dark magic happens.
//
// Recursively calls resolve on all parameters of constructor. Then calls the constructor
// with the resolved dependencies.
//
// It also handles constructors that return a (value, error) tuple.
func resolveSingle(c *Container, constructor reflect.Value, elementType reflect.Type, resolutionContext resolutionContext) (reflect.Value, error) {
	var zero reflect.Value // reflect.Value{} is possible, but it turns out declaring a zero value is a good idea regardless
	constructorType := constructor.Type()
	if constructorType.Kind() != reflect.Func {
		panic("constructor should be a function")
	}
	numberOfArguments := constructorType.NumIn()
	callArguments := make([]reflect.Value, numberOfArguments)
	for argumentIndex := range numberOfArguments {
		argumentType := constructorType.In(argumentIndex)
		reflectedResolve, found := resolveFunctionsRegistry[argumentType]
		if !found {
			return zero, fmt.Errorf("failed to resolve argument %v of constructor of type %v: no constructor registered for type %v", argumentIndex, elementType, argumentType)
		}
		argumentResolutionResult := reflectedResolve.Call([]reflect.Value{reflect.ValueOf(c), reflect.ValueOf(resolutionContext)})
		argumentValue := argumentResolutionResult[0]
		if len(argumentResolutionResult) == 2 {
			err := argumentResolutionResult[1].Interface()
			if err != nil {
				// Helping with more concise error message in case of circular dependency detection
				if strings.Contains(err.(error).Error(), "circular dependency detected") {
					return zero, err.(error)
				}
				return zero, fmt.Errorf("failed to resolve argument %v of constructor for %v: %w", argumentIndex, elementType, err.(error))
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
