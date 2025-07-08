package gosyringe

import (
	"context"
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
	registry *internal.SyncMap[dependencyKey, *dependencyRegistration]
	// Here is where the resolved instances are stored. You can think of it as a cache.
	// Singleton and Scoped resolutions rely on this to work.
	instances *internal.SyncMap[dependencyKey, any]
	// Resolution locks are used to guarantee single instances for Singleton and Scoped
	// registrations.
	resolutionLocks *internal.SyncMap[dependencyKey, *sync.Mutex]
	// If this Container was created with CreateChildContainer, then the parent will be
	// the Container which received the call. This is supposed to be used together with
	// RegisterScoped dependencies.
	parent *Container
	// The Container created with [NewContainer]. This is mainly used for stuff related to
	// Singleton, as I defined that Singletons can only be registered in root Container.
	root *Container
	// Dispose functions are registered with [OnDispose]. Every time an instance
	// is resolved, it is registered as disposable if there was an [OnDispose] for its type.
	disposeFunctions *internal.SyncMap[reflect.Type, disposeFunction]
	// Disposables registered on resolveSingle
	disposables *internal.SyncSlice[*disposable]
	// Flag to control if container was disposed
	disposed *internal.RLockValue[bool]
}

type dependencyKey struct {
	Key  string
	Type reflect.Type
}

func (k dependencyKey) String() string {
	if len(k.Key) == 0 {
		return k.Type.String()
	}

	return fmt.Sprintf("%v(%v)", k.Type, k.Key)
}

type disposable struct {
	Value           any
	DisposeFunction disposeFunction
}

type disposeFunction func(ctx context.Context, value any)

func (c *Container) isRoot() bool {
	return c.parent == nil
}

func (c *Container) getDependencyRegistration(registrationKey dependencyKey) (*dependencyRegistration, bool) {
	return c.registry.Load(registrationKey)
}

type dependencyRegistration struct {
	mu               sync.Mutex
	lifetime         dependencyLifetime
	constructorInfos []functionInfo
}

type functionInfo struct {
	Type          reflect.Type
	Value         reflect.Value
	ArgumentTypes []reflect.Type
}

func newFunctionInfo(functionValue reflect.Value) functionInfo {
	functionType := functionValue.Type()
	numberOfArguments := functionType.NumIn()
	arguments := make([]reflect.Type, numberOfArguments)
	for i := range numberOfArguments {
		arguments[i] = functionType.In(i)
	}

	functionInfo := functionInfo{
		Type:          functionType,
		Value:         functionValue,
		ArgumentTypes: arguments,
	}

	return functionInfo
}

func (dr *dependencyRegistration) AppendConstructor(constructorFunction reflect.Value) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	constructorInfo := newFunctionInfo(constructorFunction)

	dr.constructorInfos = append(dr.constructorInfos, constructorInfo)
}

type dependencyLifetime int

const (
	transient dependencyLifetime = iota + 1
	scoped
	singleton
)

func (s dependencyLifetime) String() string {
	switch s {
	case transient:
		return "Transient"
	case scoped:
		return "Scoped"
	case singleton:
		return "Singleton"
	default:
		return "Unknown"
	}
}

// Instantiates a new root Container.
func NewContainer() *Container {
	container := initContainer()
	container.root = container
	return container
}

// Instantiates a new child Container providing a Container as a parent.
func CreateChildContainer(c *Container) *Container {
	container := initContainer()
	container.parent = c
	container.root = c.root
	container.registry.Parent = c.registry
	container.disposeFunctions.Parent = c.disposeFunctions
	return container
}

func initContainer() *Container {
	container := &Container{
		registry:         internal.NewSyncMap[dependencyKey, *dependencyRegistration](),
		instances:        internal.NewSyncMap[dependencyKey, any](),
		resolutionLocks:  internal.NewSyncMap[dependencyKey, *sync.Mutex](),
		disposeFunctions: internal.NewSyncMap[reflect.Type, disposeFunction](),
		disposables:      internal.NewSyncSlice[*disposable](),
		disposed:         internal.NewRLockValue(false),
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
	registrationKey := dependencyKey{
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, transient)
}

// Same as [RegisterTransient], but with a registration key.
//
// Dependencies registered with key can be resolved with [ResolveWithKey]
func RegisterTransientWithKey[T any](c *Container, key string, constructor any) {
	if len(key) == 0 {
		panic(fmt.Sprintf("registration key for type %v cannot be empty", reflect.TypeFor[T]()))
	}
	registrationKey := dependencyKey{
		Key:  key,
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, transient)
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
	registrationKey := dependencyKey{
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, scoped)
}

// Same as [RegisterScoped], but with a registration key.
//
// Dependencies registered with key can be resolved with [ResolveWithKey]
func RegisterScopedWithKey[T any](c *Container, key string, constructor any) {
	if len(key) == 0 {
		panic(fmt.Sprintf("registration key for type %v cannot be empty", reflect.TypeFor[T]()))
	}
	registrationKey := dependencyKey{
		Key:  key,
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, scoped)
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
	registrationKey := dependencyKey{
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, singleton)
}

// Same as [RegisterSingleton], but with a registration key.
//
// Dependencies registered with key can be resolved with [ResolveWithKey]
func RegisterSingletonWithKey[T any](c *Container, key string, constructor any) {
	if len(key) == 0 {
		panic(fmt.Sprintf("registration key for type %v cannot be empty", reflect.TypeFor[T]()))
	}
	registrationKey := dependencyKey{
		Key:  key,
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, singleton)
}

// Registers a dependency for the type T as Singleton if called on a root Container and
// as Scoped if called on a child Container, providing a value as resolution method.
//
// The registered value will be cached in the Container.
//
// This is useful when you already have an instance and just want it to be injectable.
func RegisterValue[T any](c *Container, value T) {
	if c.isRoot() {
		RegisterSingleton[T](c, func() T { return value })
	} else {
		RegisterScoped[T](c, func() T { return value })
	}
}

func registerConstructor(c *Container, registrationKey dependencyKey, constructorFunctionInstance any, lifetime dependencyLifetime) {
	isDisposed, unlock := c.disposed.Load()
	defer unlock()

	if isDisposed {
		panic("cannot register on a disposed container")
	}
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
	registerType := registrationKey.Type

	if dependencyType != registerType {
		panic(fmt.Sprintf("the type parameter %v must be equal to the return type of the constructor %v", registerType, dependencyType))
	}

	if lifetime == singleton && !c.isRoot() {
		panic(fmt.Sprintf("Singletons can only be registered at a root container: %v", registerType))
	}

	depReg, _ := c.registry.LoadOrStore(registrationKey, &dependencyRegistration{
		lifetime:         lifetime,
		constructorInfos: []functionInfo{},
	})

	if depReg.lifetime != lifetime {
		panic(fmt.Sprintf("cannot register type %v as %v, because it was already registered as %v", dependencyType, lifetime, depReg.lifetime))
	}
	depReg.AppendConstructor(constructorFunction)
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
//
// If the Container don't have a registration for the type T, then it will be resolved
// from the parent Container up to the root. If none of the Containers up the hierarchy
// has a registration, then this will return an error.
//
// If the registered constructor for T returns an error, then this will return an error.
// In the case of slice type, the resolution will stop at the first element instantiation
// error.
func Resolve[T any](c *Container) (T, error) {
	resolutionContext := resolutionContext{
		container: c,
		stack:     []dependencyKey{},
	}
	resolutionKey := dependencyKey{
		Type: reflect.TypeFor[T](),
	}
	var zero T
	value, err := resolve(resolutionContext, resolutionKey)
	if err != nil {
		return zero, err
	}
	return value.(T), nil
}

// Multiple dependencies can be registered for the same type with
// different keys. This is useful for strategy pattern implementations.
//
// Works the same as [Resolve] but use a key and a type for resolution.
func ResolveWithKey[T any](c *Container, key string) (T, error) {
	resolutionContext := resolutionContext{
		container: c,
		stack:     []dependencyKey{},
	}
	resolutionKey := dependencyKey{
		Key:  key,
		Type: reflect.TypeFor[T](),
	}
	var zero T
	value, err := resolve(resolutionContext, resolutionKey)
	if err != nil {
		return zero, err
	}
	return value.(T), nil
}

// Tracks information about undergoing Resolve call.
type resolutionContext struct {
	// The container with which [Resolve] was called
	container *Container
	// This is to apply the injection lifetime rules.
	mainLifetime dependencyLifetime
	// This is to track nested resolutions to identify cyclic dependencies.
	stack []dependencyKey
	// This is to add disposables
	dependencyRegistration *dependencyRegistration
}

func (rc resolutionContext) push(resolutionKey dependencyKey) resolutionContext {
	return resolutionContext{
		container:    rc.container,
		mainLifetime: rc.mainLifetime,
		stack:        append(rc.stack, resolutionKey),
	}
}

func (rc resolutionContext) isRoot() bool {
	return len(rc.stack) == 0
}

func (rc resolutionContext) resolutionKey() dependencyKey {
	return rc.stack[len(rc.stack)-1]
}

func (rc resolutionContext) dependencyGraphString() string {
	typesString := internal.Map(
		rc.stack,
		func(t dependencyKey) string { return fmt.Sprintf("%v", t) },
	)
	return strings.Join(typesString, " -> ")
}

func resolve(parentResolutionContext resolutionContext, resolutionKey dependencyKey) (any, error) {
	isDisposed, unlock := parentResolutionContext.container.disposed.Load()
	defer unlock()

	if isDisposed {
		return nil, fmt.Errorf("cannot resolve on a disposed container")
	}
	currentResolutionContext := parentResolutionContext.push(resolutionKey)

	container := currentResolutionContext.container

	requestedResolutionKey := resolutionKey
	actualResolutionKey := resolutionKey
	isSlice := resolutionKey.Type.Kind() == reflect.Slice
	if isSlice {
		actualResolutionKey = dependencyKey{
			Key:  resolutionKey.Key,
			Type: resolutionKey.Type.Elem(),
		}
	}

	// Checking if there is a registration.
	// Here actualResolutionKey is used instead of requestedResolutionKey, because we
	// don't expect calls to register[[]T].
	dependencyRegistration, found := container.getDependencyRegistration(actualResolutionKey)
	if !found {
		return nil, fmt.Errorf("no constructor registered for type %v", actualResolutionKey)
	}
	currentResolutionContext.dependencyRegistration = dependencyRegistration

	var cacheContainer *Container
	if dependencyRegistration.lifetime == singleton {
		cacheContainer = container.root
	} else if dependencyRegistration.lifetime == scoped {
		cacheContainer = container
	}

	if cacheContainer != nil {
		// Checking if an instance is already in the cache
		cachedInstance, found := cacheContainer.instances.Load(resolutionKey)
		if found {
			return cachedInstance, nil
		}
	}

	isCyclic := slices.Contains(parentResolutionContext.stack, resolutionKey)
	if isCyclic {
		// We know it's a circular dependency because resolve was already called for type T
		// up in the stack. We decided to not handle it yet and just return an error.
		//
		// Circular dependencies can be resolved by providing some kind of lazy evaluation,
		// but it's too complex, and it's not in our plan right now.
		return nil, fmt.Errorf("circular dependency detected: %s", currentResolutionContext.dependencyGraphString())
	}

	// Promoting resolutionContext mainLifetime, to prevent indirect forbidden injections
	if parentResolutionContext.isRoot() {
		currentResolutionContext.mainLifetime = dependencyRegistration.lifetime
	} else {
		if dependencyRegistration.lifetime > currentResolutionContext.mainLifetime {
			currentResolutionContext.mainLifetime = dependencyRegistration.lifetime
		}
	}

	if cacheContainer != nil {
		resolutionLock, _ := cacheContainer.resolutionLocks.LoadOrStore(requestedResolutionKey, &sync.Mutex{})
		// By acquiring a lock per resolution type we prevent multiple Singleton or Scoped instances
		resolutionLock.Lock()
		defer resolutionLock.Unlock()

		// Checking again for a cached instance because it could be created by other concurrent resolve calls
		cachedInstance, found := cacheContainer.instances.Load(requestedResolutionKey)
		if found {
			return cachedInstance, nil
		}
	}

	// Resolution lifetime rules
	if dependencyRegistration.lifetime == scoped {
		if currentResolutionContext.mainLifetime == singleton {
			// cannot inject Scoped into Singleton
			return nil, fmt.Errorf("cannot inject Scoped (%v) dependencies in Singletons (%v)", requestedResolutionKey, parentResolutionContext.resolutionKey())
		}
	}

	if dependencyRegistration.lifetime == scoped {
		if container.isRoot() {
			return nil, fmt.Errorf("cannot resolve Scoped dependencies from the root Container: %v", requestedResolutionKey)
		}
	}

	constructors := dependencyRegistration.constructorInfos

	var value reflect.Value

	if isSlice {
		// When resolutionType is a slice, we resolve all registered dependencies for it,
		// then return the result for it only if all are successful. Failing fast for simplicity.
		sliceValue := reflect.MakeSlice(requestedResolutionKey.Type, len(constructors), len(constructors))
		for constructorIndex, constructor := range constructors {
			value, err := resolveSingle(currentResolutionContext, constructor, actualResolutionKey)
			if err != nil {
				return nil, err
			}

			sliceValue.Index(constructorIndex).Set(value)
		}
		value = sliceValue

	} else {
		// When resolutionType is not a slice, then resolve using the last registration.
		constructor := constructors[len(constructors)-1]

		singleValue, err := resolveSingle(currentResolutionContext, constructor, actualResolutionKey)
		if err != nil {
			return nil, err
		}
		value = singleValue
	}

	if cacheContainer != nil {
		cacheContainer.instances.Store(requestedResolutionKey, value.Interface())
	}

	return value.Interface(), nil
}

// Here is where the reflection dark magic happens.
//
// Recursively calls resolve on all parameters of constructor. Then calls the constructor
// with the resolved dependencies.
//
// It also handles constructors that return a (value, error) tuple.
//
//	.																																						 .resolutionKey is just for error messages
func resolveSingle(resolutionContext resolutionContext, constructor functionInfo, resolutionKey dependencyKey) (reflect.Value, error) {
	var zero reflect.Value

	resolutionResult, err := invoke(constructor, resolutionContext, invokeMessageOptions{
		InvokeTargetType:     "constructor",
		InvokeTargetFullName: fmt.Sprintf("constructor for %v", resolutionKey),
	})

	if err != nil {
		return zero, err
	}

	value := resolutionResult[0]

	if len(resolutionResult) == 2 {
		err := resolutionResult[1].Interface()
		if err != nil {
			return zero, fmt.Errorf("failed to resolve a value for type %v: %w", resolutionKey, err.(error))
		}
	}

	disposeContainer := resolutionContext.container
	if resolutionContext.dependencyRegistration.lifetime == singleton {
		disposeContainer = disposeContainer.root
	}
	disposeFunction, found := disposeContainer.disposeFunctions.Load(resolutionKey.Type)
	if found {
		disposeContainer.disposables.Append(&disposable{
			Value:           value.Interface(),
			DisposeFunction: disposeFunction,
		})
	}

	return value, nil
}

// Registers a callback to be called on [Dispose]. The callback will be called for
// any instance resolved with the type [T], even those registered with key.
func OnDispose[T any](c *Container, disposeFunction func(ctx context.Context, value T)) {
	c.disposeFunctions.Store(reflect.TypeFor[T](), func(ctx context.Context, value any) {
		disposeFunction(ctx, value.(T))
	})
}

// Calls the callbacks registered with [OnDispose]. Callbacks are done in parallel.
// It waits until all dispose callbacks are done or the context timeout, whichever
// comes first.
//
// Returns true if all callbacks finished, false if reached timeout before.
func Dispose(ctx context.Context, c *Container) bool {
	c.disposed.Store(true)

	var wg sync.WaitGroup
	disposables := c.disposables.Snapshot()
	for _, d := range disposables {
		wg.Add(1)
		go func(disposable *disposable) {
			defer wg.Done()
			disposable.DisposeFunction(ctx, disposable.Value)
		}(d)
	}

	waitGroupDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitGroupDone)
	}()

	select {
	case <-waitGroupDone:
		return true
	case <-ctx.Done():
		return false
	}
}

type invokeMessageOptions struct {
	InvokeTargetType     string
	InvokeTargetFullName string
}

func invoke(function functionInfo, resolutionContext resolutionContext, messageOptions invokeMessageOptions) ([]reflect.Value, error) {
	functionType := function.Type
	if functionType.Kind() != reflect.Func {
		panic(fmt.Sprintf("%s should be a function", messageOptions.InvokeTargetType))
	}
	callArguments := make([]reflect.Value, len(function.ArgumentTypes))
	for argumentIndex, argumentType := range function.ArgumentTypes {
		argumentResolutionKey := dependencyKey{
			Type: argumentType,
		}

		argumentValue, err := resolve(resolutionContext, argumentResolutionKey)

		if err != nil {
			// Helping with more concise error message in case of circular dependency detection
			if strings.Contains(err.Error(), "circular dependency detected") {
				return nil, err
			}

			return nil, fmt.Errorf("failed to resolve argument %v of %v: %w", argumentIndex, messageOptions.InvokeTargetFullName, err)
		}
		callArguments[argumentIndex] = reflect.ValueOf(argumentValue)
	}

	resolutionResult := function.Value.Call(callArguments)
	return resolutionResult, nil
}
