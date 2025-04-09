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
	// The Container mutex is to control concurrency over registrations and resolutions.
	mu sync.Mutex
	// Here is where the constructors are registered.
	//
	// We allow multiple constructors per resolution type, because we want to be able
	// to resolve slices of dependencies of the same type, and to override registrations
	// as well.
	registry map[reflect.Type]*dependencyRegistration
	// Here is where the resolved instances are stored. You can think of it as a cache.
	// Singleton and Scoped resolutions rely on this to work.
	instances map[reflect.Type]reflect.Value
	// Resolution locks are necessary because nested calls to resolve results in a dead lock
	// if we use the Container mutex only.
	resolutionLocks map[reflect.Type]*sync.Mutex
	// If this Container was created with CreateChildContainer, then the parent will be
	// the Container which received the call. This is supposed to be used together with
	// RegisterScoped dependencies.
	parent *Container
}

func (c *Container) isRoot() bool {
	return c.parent == nil
}

type dependencyRegistration struct {
	lifetime     dependencyLifetime
	constructors []reflect.Value
}

type dependencyLifetime int

const (
	Transient dependencyLifetime = iota + 1
	Scoped                       = 2
	Singleton                    = 3
)

func (s dependencyLifetime) String() string {
	switch s {
	case Transient:
		return "Transient"
	case Scoped:
		return "Scoped"
	case Singleton:
		return "Singleton"
	default:
		return "Unknown"
	}
}

// Instantiates a new root Container.
func NewContainer() *Container {
	return &Container{
		registry:        make(map[reflect.Type]*dependencyRegistration),
		instances:       make(map[reflect.Type]reflect.Value),
		resolutionLocks: map[reflect.Type]*sync.Mutex{},
	}
}

func CreateChildContainer(c *Container) *Container {
	return &Container{
		registry:        make(map[reflect.Type]*dependencyRegistration),
		instances:       make(map[reflect.Type]reflect.Value),
		resolutionLocks: map[reflect.Type]*sync.Mutex{},
		parent:          c,
	}
}

// Registers a dependency for the type T as Transient, providing a constructor as resolution method.
//
// Multiple calls to Resolve on a dependency registered as Transient always return a new instance.
//
// constructor must be a function returning exactly one value or a (value, error) tuple.
// The type of the return value must be exactly equal to the type parameter T.
//
// If RegisterTransient is called multiple times, the last registration is considered for resolution.
func RegisterTransient[T any](c *Container, constructor any) {
	register[T](c, constructor, Transient)
}

// Registers a dependency for the type T as Scoped, providing a constructor as resolution method.
//
// Multiple calls to Resolve on a dependency registered as Scoped will return the same instance
// when they are done on the same Container.
//
// This is specially useful with CreateChildContainer. With Scoped dependencies you can implement
// things like a RequestContext, a UserSession, or any kind of service that should have its lifetime
// and data scoped for a single operation or a set of related operations that should not span
// the whole application. Think like multiple executions with isolation among them.
//
// constructor must be a function returning exactly one value or a (value, error) tuple.
// The type of the return value must be exactly equal to the type parameter T.
//
// If RegisterScoped is called multiple times, the last registration is considered for resolution.
func RegisterScoped[T any](c *Container, constructor any) {
	register[T](c, constructor, Scoped)
}

// Registers a dependency for the type T as Singleton, providing a constructor as resolution method.
//
// Multiple calls to Resolve on a dependency registered as Singleton are guaranteed to return
// the same instance in the Container lifetime.
//
// constructor must be a function returning exactly one value or a (value, error) tuple.
// The type of the return value must be exactly equal to the type parameter T.
//
// If RegisterSingleton is called multiple times, the last registration is considered for resolution.
func RegisterSingleton[T any](c *Container, constructor any) {
	register[T](c, constructor, Singleton)
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

func register[T any](c *Container, constructor any, lifetime dependencyLifetime) {
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

	if lifetime == Singleton && !c.isRoot() {
		panic(fmt.Sprintf("Singletons can only be registered at a root container: %v", registerType))
	}

	// Here we lock on the container to prevent multiple go routines to
	// write at the same time to the dependency registry
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.registry[dependencyType] == nil {
		c.registry[dependencyType] = &dependencyRegistration{
			lifetime:     lifetime,
			constructors: []reflect.Value{},
		}
	}

	if c.registry[dependencyType].lifetime != lifetime {
		panic(fmt.Sprintf("cannot register type %v as %v, because it was already registered as %v", dependencyType, lifetime, c.registry[dependencyType].lifetime))
	}
	c.registry[dependencyType].constructors = append(c.registry[dependencyType].constructors, reflectedConstructor)

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

// Calls the constructor registered for the type T.
//
// If T was registered as Singleton, then the instance will be retrieved
// from the root Container cache.
//
// If T was registered as Scoped, then the instance will be retrieved from
// the child Container cache.
//
// If T is a slice type []E, then resolves all registered dependencies for E.
func Resolve[T any](c *Container) (T, error) {
	resolutionContext := resolutionContext{
		cacheContainer: c,
		stack:          []reflect.Type{},
	}
	return resolve[T](c, resolutionContext)
}

// Tracks information about undergoing Resolve call.
type resolutionContext struct {
	// This is to know if the cached instances will be stored in the child container or root.
	cacheContainer *Container
	// This is to apply the injection lifetime rules.
	lifetime dependencyLifetime
	// This is to track nested resolutions to identify cyclic dependencies.
	stack []reflect.Type
}

func (rc resolutionContext) push(resolutionType reflect.Type) resolutionContext {
	return resolutionContext{
		cacheContainer: rc.cacheContainer,
		lifetime:       rc.lifetime,
		stack:          append(rc.stack, resolutionType),
	}
}

func (rc resolutionContext) isRoot() bool {
	return len(rc.stack) == 0
}

func (rc resolutionContext) resolutionType() reflect.Type {
	return rc.stack[len(rc.stack)-1]
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

func resolve[T any](c *Container, parentResolutionContext resolutionContext) (T, error) {
	var zero T // small trick since x := T{} is not possible
	requestedResolutionType := reflect.TypeFor[T]()
	isCyclic := slices.Contains(parentResolutionContext.stack, requestedResolutionType)
	if isCyclic {
		// We know it's a circular dependency because resolve was already called for type T
		// up in the stack. We decided to not handle it yet and just return an error.
		//
		// Circular dependencies can be resolved by providing some kind of lazy evaluation,
		// but it's too complex, and it's not in our plan right now.
		return zero, fmt.Errorf("circular dependency detected: %s", parentResolutionContext.dependencyGraphString(true))
	}

	currentResolutionContext := parentResolutionContext.push(requestedResolutionType)
	isSlice := requestedResolutionType.Kind() == reflect.Slice
	actualResolutionType := requestedResolutionType
	if isSlice {
		actualResolutionType = requestedResolutionType.Elem()
	}

	// Locking on container mutex to prevent simultaneous access to
	// the resolutionLocks list.
	c.mu.Lock()
	resolutionLock, found := c.resolutionLocks[requestedResolutionType]
	if !found {
		resolutionLock = &sync.Mutex{}
		c.resolutionLocks[requestedResolutionType] = resolutionLock
	}
	c.mu.Unlock()

	// By acquiring a lock per resolution type we prevent dead locks
	// on 2+ level deep dependency resolution
	resolutionLock.Lock()
	defer resolutionLock.Unlock()

	// Checking if there is a registration for elementType.
	// Here actualResolutionType is used instead of requestedResolutionType, because we
	// don't expect calls to register[[]T].
	dependencyRegistration, foundDependencyRegistration := c.registry[actualResolutionType]
	if !foundDependencyRegistration || len(dependencyRegistration.constructors) == 0 {
		if c.parent != nil {
			return resolve[T](c.parent, parentResolutionContext)
		}
		return zero, fmt.Errorf("no constructor registered for type %v", actualResolutionType)
	}

	if parentResolutionContext.isRoot() {
		currentResolutionContext.lifetime = dependencyRegistration.lifetime
	}

	// Resolution lifetime rules
	if dependencyRegistration.lifetime == Scoped {
		if currentResolutionContext.lifetime == Transient {
			// cannot inject Scoped into Transient
			return zero, fmt.Errorf("cannot inject Scoped (%v) dependencies in Transients (%v)", requestedResolutionType, parentResolutionContext.resolutionType())
		}

		if currentResolutionContext.lifetime == Singleton {
			// cannot inject Scoped into Singleton
			return zero, fmt.Errorf("cannot inject Scoped (%v) dependencies in Singletons (%v)", requestedResolutionType, parentResolutionContext.resolutionType())
		}
	}

	if dependencyRegistration.lifetime == Transient {
		if currentResolutionContext.lifetime == Singleton {
			return zero, fmt.Errorf("cannot inject Transient (%v) dependencies in Singletons (%v)", requestedResolutionType, parentResolutionContext.resolutionType())
		}
	}

	cacheContainer := currentResolutionContext.cacheContainer

	// Checking if an instance is already in the cache
	if dependencyRegistration.lifetime == Singleton {
		instance, foundInstance := cacheContainer.instances[requestedResolutionType]
		if foundInstance {
			return instance.Interface().(T), nil
		}
	}

	if dependencyRegistration.lifetime == Scoped {
		if cacheContainer.isRoot() {
			return zero, fmt.Errorf("cannot resolve Scoped dependencies from the root Container: %v", requestedResolutionType)
		}
		instance, foundInstance := cacheContainer.instances[requestedResolutionType]
		if foundInstance {
			return instance.Interface().(T), nil
		}
	}

	constructors := dependencyRegistration.constructors

	if isSlice {
		// When resolutionType is a slice, we resolve all registered dependencies for it,
		// then return the result for it only if all are successful. Failing fast for simplicity.
		sliceValue := reflect.MakeSlice(requestedResolutionType, len(constructors), len(constructors))
		for constructorIndex, constructor := range constructors {
			value, err := resolveSingle(c, constructor, actualResolutionType, currentResolutionContext)
			if err != nil {
				return zero, err
			}

			sliceValue.Index(constructorIndex).Set(value)
		}

		if dependencyRegistration.lifetime == Singleton || dependencyRegistration.lifetime == Scoped {
			cacheContainer.instances[requestedResolutionType] = sliceValue
		}

		return sliceValue.Interface().(T), nil
	} else {
		// When resolutionType is not a slice, then resolve using the last registration.
		constructor := constructors[len(constructors)-1]

		value, err := resolveSingle(c, constructor, actualResolutionType, currentResolutionContext)
		if err != nil {
			return zero, err
		}

		if dependencyRegistration.lifetime == Singleton || dependencyRegistration.lifetime == Scoped {
			cacheContainer.instances[requestedResolutionType] = value
		}

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
