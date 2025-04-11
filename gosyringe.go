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
	// Here is where the constructors are registered.
	//
	// We allow multiple constructors per resolution type, because we want to be able
	// to resolve slices of dependencies of the same type, and to override registrations
	// as well.
	registry *internal.SyncMap[reflect.Type, *dependencyRegistration]
	// Here is where the resolved instances are stored. You can think of it as a cache.
	// Singleton and Scoped resolutions rely on this to work.
	instances *internal.SyncMap[reflect.Type, any]
	// Resolution locks are necessary because nested calls to resolve results in multiple
	// instances resolved for a Singleton.
	resolutionLocks *internal.SyncMap[reflect.Type, *sync.Mutex]
	// If this Container was created with CreateChildContainer, then the parent will be
	// the Container which received the call. This is supposed to be used together with
	// RegisterScoped dependencies.
	parent *Container
}

func (c *Container) isRoot() bool {
	return c.parent == nil
}

func (c *Container) getDependencyRegistration(registrationType reflect.Type) (*dependencyRegistration, bool) {
	dependencyRegistration, found := c.registry.Load(registrationType)
	if found {
		return dependencyRegistration, true
	}
	if c.parent != nil {
		dependencyRegistration, found := c.parent.registry.Load(registrationType)
		return dependencyRegistration, found
	}

	return nil, false
}

type dependencyRegistration struct {
	mu           sync.Mutex
	lifetime     dependencyLifetime
	constructors []constructor
}

type constructor struct {
	cType     reflect.Type
	function  reflect.Value
	arguments []reflect.Type
}

func (dr *dependencyRegistration) AppendConstructor(constructorFunction reflect.Value) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	constructorType := constructorFunction.Type()
	numberOfArguments := constructorType.NumIn()
	arguments := make([]reflect.Type, numberOfArguments)
	for i := range numberOfArguments {
		arguments[i] = constructorType.In(i)
	}

	constructor := constructor{
		cType:     constructorType,
		function:  constructorFunction,
		arguments: arguments,
	}
	dr.constructors = append(dr.constructors, constructor)
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
	container := initContainer()
	return container
}

func CreateChildContainer(c *Container) *Container {
	container := initContainer()
	container.parent = c
	return container
}

func initContainer() *Container {
	container := &Container{
		registry:        internal.NewSyncMap[reflect.Type, *dependencyRegistration](),
		instances:       internal.NewSyncMap[reflect.Type, any](),
		resolutionLocks: internal.NewSyncMap[reflect.Type, *sync.Mutex](),
	}

	RegisterValue(container, container)

	return container
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
	registerConstructor[T](c, constructor, Transient)
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
	registerConstructor[T](c, constructor, Scoped)
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
	registerConstructor[T](c, constructor, Singleton)
}

func RegisterValue[T any](c *Container, value T) {
	if c.isRoot() {
		RegisterSingleton[T](c, func() T { return value })
	} else {
		RegisterScoped[T](c, func() T { return value })
	}
}

// Go does not provide a way to dynamically instantiate a generic function.
// All generic function calls must be done statically.
//
// But we can overcome this
// limitation by storing the references of the functions and retrieving them later
// by their reflection type. This is also why we need to provide a type
// parameter in the register functions.
var resolveFunctionsRegistry = internal.NewSyncMap[reflect.Type, reflect.Value]()

func registerConstructor[T any](c *Container, constructorFunctionInstance any, lifetime dependencyLifetime) {
	constructorFunction := reflect.ValueOf(constructorFunctionInstance)
	constructorType := constructorFunction.Type()

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

	depReg, _ := c.registry.LoadOrStore(dependencyType, &dependencyRegistration{
		lifetime:     lifetime,
		constructors: []constructor{},
	})

	if depReg.lifetime != lifetime {
		panic(fmt.Sprintf("cannot register type %v as %v, because it was already registered as %v", dependencyType, lifetime, depReg.lifetime))
	}
	depReg.AppendConstructor(constructorFunction)

	// We need to register both dependencyType and slice of dependencyType,
	// because we want to allow the user to call Resolve[T] an Resolve[[]T]
	resolveFunctionsRegistry.Store(dependencyType, reflect.ValueOf(resolve[T]))
	resolveFunctionsRegistry.Store(reflect.SliceOf(dependencyType), reflect.ValueOf(resolve[[]T]))
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
		container: c,
		stack:     []reflect.Type{},
	}
	return resolve[T](resolutionContext)
}

// Tracks information about undergoing Resolve call.
type resolutionContext struct {
	// This is to know if the cached instances will be stored in the child container or root.
	container *Container
	// This is to know current dependency resolution lifetime
	lifetime dependencyLifetime
	// This is to apply the injection lifetime rules.
	mainLifetime dependencyLifetime
	// This is to track nested resolutions to identify cyclic dependencies.
	stack []reflect.Type
}

func (rc resolutionContext) push(resolutionType reflect.Type) resolutionContext {
	return resolutionContext{
		container: rc.container,
		lifetime:  rc.lifetime,
		stack:     append(rc.stack, resolutionType),
	}
}

func (rc resolutionContext) isRoot() bool {
	return len(rc.stack) == 0
}

func (rc resolutionContext) resolutionType() reflect.Type {
	return rc.stack[len(rc.stack)-1]
}

func (rc resolutionContext) dependencyGraphString() string {
	typesString := internal.Map(
		rc.stack,
		func(t reflect.Type) string { return fmt.Sprintf("%v", t) },
	)
	return strings.Join(typesString, " -> ")
}

func resolve[T any](parentResolutionContext resolutionContext) (T, error) {

	var zero T // small trick since x := T{} is not possible
	requestedResolutionType := reflect.TypeFor[T]()
	currentResolutionContext := parentResolutionContext.push(requestedResolutionType)

	c := currentResolutionContext.container

	// Checking if an instance is already in the cache
	cachedInstance, found := currentResolutionContext.container.instances.Load(requestedResolutionType)
	if found {
		return cachedInstance.(T), nil
	}

	isCyclic := slices.Contains(parentResolutionContext.stack, requestedResolutionType)
	if isCyclic {
		// We know it's a circular dependency because resolve was already called for type T
		// up in the stack. We decided to not handle it yet and just return an error.
		//
		// Circular dependencies can be resolved by providing some kind of lazy evaluation,
		// but it's too complex, and it's not in our plan right now.
		return zero, fmt.Errorf("circular dependency detected: %s", currentResolutionContext.dependencyGraphString())
	}

	isSlice := requestedResolutionType.Kind() == reflect.Slice
	actualResolutionType := requestedResolutionType
	if isSlice {
		actualResolutionType = requestedResolutionType.Elem()
	}

	// Checking if there is a registration for elementType.
	// Here actualResolutionType is used instead of requestedResolutionType, because we
	// don't expect calls to register[[]T].
	dependencyRegistration, found := c.getDependencyRegistration(actualResolutionType)
	if !found {
		return zero, fmt.Errorf("no constructor registered for type %v", actualResolutionType)
	}

	currentResolutionContext.lifetime = dependencyRegistration.lifetime

	// Promoting resolutionContext mainLifetime, to prevent indirect forbidden injections
	if parentResolutionContext.isRoot() {
		currentResolutionContext.mainLifetime = dependencyRegistration.lifetime
	} else {
		if dependencyRegistration.lifetime > currentResolutionContext.mainLifetime {
			currentResolutionContext.mainLifetime = dependencyRegistration.lifetime
		}
	}

	if currentResolutionContext.lifetime == Singleton || currentResolutionContext.lifetime == Scoped {
		resolutionLock, _ := c.resolutionLocks.LoadOrStore(requestedResolutionType, &sync.Mutex{})

		// By acquiring a lock per resolution type we prevent multiple Singleton or Scoped instances
		resolutionLock.Lock()
		defer resolutionLock.Unlock()

		// Checking again for a cached instance because it could be created by other concurrent resolve calls
		cachedInstance, found = currentResolutionContext.container.instances.Load(requestedResolutionType)
		if found {
			return cachedInstance.(T), nil
		}
	}

	// Resolution lifetime rules
	if dependencyRegistration.lifetime == Scoped {
		if currentResolutionContext.mainLifetime == Singleton {
			// cannot inject Scoped into Singleton
			return zero, fmt.Errorf("cannot inject Scoped (%v) dependencies in Singletons (%v)", requestedResolutionType, parentResolutionContext.resolutionType())
		}
	}

	if dependencyRegistration.lifetime == Scoped {
		if c.isRoot() {
			return zero, fmt.Errorf("cannot resolve Scoped dependencies from the root Container: %v", requestedResolutionType)
		}
	}

	constructors := dependencyRegistration.constructors

	if isSlice {
		// When resolutionType is a slice, we resolve all registered dependencies for it,
		// then return the result for it only if all are successful. Failing fast for simplicity.
		sliceValue := reflect.MakeSlice(requestedResolutionType, len(constructors), len(constructors))
		for constructorIndex, constructor := range constructors {
			value, err := resolveSingle(currentResolutionContext, constructor, actualResolutionType)
			if err != nil {
				return zero, err
			}

			sliceValue.Index(constructorIndex).Set(value)
		}

		if dependencyRegistration.lifetime == Singleton || dependencyRegistration.lifetime == Scoped {
			c.instances.Store(requestedResolutionType, sliceValue.Interface())
		}

		return sliceValue.Interface().(T), nil
	} else {
		// When resolutionType is not a slice, then resolve using the last registration.
		constructor := constructors[len(constructors)-1]

		value, err := resolveSingle(currentResolutionContext, constructor, actualResolutionType)
		if err != nil {
			return zero, err
		}

		if dependencyRegistration.lifetime == Singleton || dependencyRegistration.lifetime == Scoped {
			c.instances.Store(requestedResolutionType, value.Interface())
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
func resolveSingle(resolutionContext resolutionContext, constructor constructor, elementType reflect.Type) (reflect.Value, error) {
	var zero reflect.Value // reflect.Value{} is possible, but it turns out declaring a zero value is a good idea regardless
	constructorType := constructor.cType
	if constructorType.Kind() != reflect.Func {
		panic("constructor should be a function")
	}
	callArguments := make([]reflect.Value, len(constructor.arguments))
	for argumentIndex, argumentType := range constructor.arguments {
		reflectedResolve, found := resolveFunctionsRegistry.Load(argumentType)
		if !found {
			return zero, fmt.Errorf("failed to resolve argument %v of constructor of type %v: no constructor registered for type %v", argumentIndex, elementType, argumentType)
		}
		argumentResolutionResult := reflectedResolve.Call([]reflect.Value{reflect.ValueOf(resolutionContext)})
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

	resolutionResult := constructor.function.Call(callArguments)

	value := resolutionResult[0]

	if len(resolutionResult) == 2 {
		err := resolutionResult[1].Interface()
		if err != nil {
			return zero, fmt.Errorf("failed to resolve a value for type %v: %w", elementType, err.(error))
		}
	}

	return value, nil
}
