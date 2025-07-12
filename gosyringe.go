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
	registry *internal.SyncMap[registrationKey, *dependencyRegistration]
	// Here is where the resolved instances are stored. You can think of it as a cache.
	// Singleton and Scoped resolutions rely on this to work.
	instances *internal.SyncMap[resolutionKey, any]
	// Resolution locks are used to guarantee single instances for Singleton and Scoped
	// registrations.
	resolutionLocks *internal.SyncMap[resolutionKey, *sync.Mutex]
	// If this Container was created with CreateChildContainer, then the parent will be
	// the Container which received the call. This is supposed to be used together with
	// RegisterScoped dependencies.
	parent *Container
	// The Container created with [NewContainer]. This is mainly used for stuff related to
	// Singleton, as I defined that Singletons can only be registered in root Container.
	root *Container
	// Dispose callbacks are registered with [OnDispose]. Every time an instance
	// is resolved, it is registered as disposable if there was an [OnDispose] for its type.
	disposeCallbacks *internal.SyncMap[reflect.Type, disposeCallback]
	// Disposables registered on resolveSingle
	disposables *internal.SyncSlice[*disposable]
	// Flag to control if container was disposed
	disposed *internal.RLockValue[bool]
	// Start callbacks are registered with [OnStart]. Container will automatically
	// resolve all dependencies registered here when calling [Start]. Available
	// only for Singleton registrations.
	startCallbacks *internal.SyncMap[resolutionKey, startCallback]
	// Flag to control if container was started
	started *internal.RLockValue[bool]
}

type registrationKey struct {
	Key  string
	Type reflect.Type
}

func (k registrationKey) String() string {
	if len(k.Key) == 0 {
		return k.Type.String()
	}

	return fmt.Sprintf("%v(%v)", k.Type, k.Key)
}

type resolutionKey struct {
	Key        string
	Type       reflect.Type
	ResolveAll bool
}

func (k resolutionKey) String() string {
	typeStr := k.Type.String()
	if k.ResolveAll {
		typeStr = "[]" + typeStr
	}
	if len(k.Key) == 0 {
		return typeStr
	}

	return fmt.Sprintf("%v(%v)", typeStr, k.Key)
}

func (k resolutionKey) GetRegistrationKey() registrationKey {
	registrationKey := registrationKey{
		Key:  k.Key,
		Type: k.Type,
	}
	return registrationKey
}

type disposable struct {
	Value   any
	Dispose disposeCallback
}

type disposeCallback func(ctx context.Context, value any)

type startCallback func(value any)

func (c *Container) isRoot() bool {
	return c.parent == nil
}

func (c *Container) getDependencyRegistration(registrationKey registrationKey) (*dependencyRegistration, bool) {
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
	container.disposeCallbacks.Parent = c.disposeCallbacks
	return container
}

func initContainer() *Container {
	container := &Container{
		registry:         internal.NewSyncMap[registrationKey, *dependencyRegistration](),
		instances:        internal.NewSyncMap[resolutionKey, any](),
		resolutionLocks:  internal.NewSyncMap[resolutionKey, *sync.Mutex](),
		disposeCallbacks: internal.NewSyncMap[reflect.Type, disposeCallback](),
		disposables:      internal.NewSyncSlice[*disposable](),
		disposed:         internal.NewRLockValue(false),
		startCallbacks:   internal.NewSyncMap[resolutionKey, startCallback](),
		started:          internal.NewRLockValue(false),
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
	registrationKey := registrationKey{
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, transient)
}

// Same as [RegisterTransient], but with a registration key.
//
// Dependencies registered with key can be resolved with [ResolveWithKey]
func RegisterTransientWithKey[T any](c *Container, key string, constructor any) {
	if len(key) == 0 {
		panic(fmt.Errorf("registration key for type %v cannot be empty", reflect.TypeFor[T]()))
	}
	registrationKey := registrationKey{
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
	registrationKey := registrationKey{
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, scoped)
}

// Same as [RegisterScoped], but with a registration key.
//
// Dependencies registered with key can be resolved with [ResolveWithKey]
func RegisterScopedWithKey[T any](c *Container, key string, constructor any) {
	if len(key) == 0 {
		panic(fmt.Errorf("registration key for type %v cannot be empty", reflect.TypeFor[T]()))
	}
	registrationKey := registrationKey{
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
	registrationKey := registrationKey{
		Type: reflect.TypeFor[T](),
	}
	registerConstructor(c, registrationKey, constructor, singleton)
}

// Same as [RegisterSingleton], but with a registration key.
//
// Dependencies registered with key can be resolved with [ResolveWithKey]
func RegisterSingletonWithKey[T any](c *Container, key string, constructor any) {
	if len(key) == 0 {
		panic(fmt.Errorf("registration key for type %v cannot be empty", reflect.TypeFor[T]()))
	}
	registrationKey := registrationKey{
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

func registerConstructor(c *Container, registrationKey registrationKey, constructorFunctionInstance any, lifetime dependencyLifetime) {
	isDisposed, unlock := c.disposed.Load()
	defer unlock()

	if isDisposed {
		panic(fmt.Errorf("cannot register on a disposed container"))
	}
	constructorFunction := reflect.ValueOf(constructorFunctionInstance)
	constructorType := constructorFunction.Type()

	if constructorType.Kind() != reflect.Func || constructorType.NumOut() < 1 || constructorType.NumOut() > 2 {
		panic(fmt.Errorf("constructor must be a function returning exactly one value, or a value and an error"))
	}

	if constructorType.NumOut() == 2 {
		errorType := constructorType.Out(1)
		if !errorType.AssignableTo(reflect.TypeFor[error]()) {
			panic(fmt.Errorf("constructor must be a function returning exactly one value, or a value and an error"))
		}
	}

	dependencyType := constructorType.Out(0)
	registerType := registrationKey.Type

	if dependencyType != registerType {
		panic(fmt.Errorf("the type parameter %v must be equal to the return type of the constructor %v", registerType, dependencyType))
	}

	if lifetime == singleton && !c.isRoot() {
		panic(fmt.Errorf("%v can only be registered at a root container: %v", singleton, registerType))
	}

	depReg, _ := c.registry.LoadOrStore(registrationKey, &dependencyRegistration{
		lifetime:         lifetime,
		constructorInfos: []functionInfo{},
	})

	if depReg.lifetime != lifetime {
		panic(fmt.Errorf("cannot register type %v as %v, because it was already registered as %v", dependencyType, lifetime, depReg.lifetime))
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
	resolutionContext := newResolutionContext(c)
	resolutionKey := resolutionKey{
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
	resolutionContext := newResolutionContext(c)
	resolutionKey := resolutionKey{
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

// Resolves all instances of [T].
//
// See [Resolve].
func ResolveAll[T any](c *Container) ([]T, error) {
	resolutionContext := newResolutionContext(c)
	resolutionKey := resolutionKey{
		Type:       reflect.TypeFor[T](),
		ResolveAll: true,
	}
	value, err := resolve(resolutionContext, resolutionKey)
	if err != nil {
		return nil, err
	}
	return value.([]T), nil
}

// Resolves all instances of [T] registered with key.
//
// See [ResolveWithKey].
func ResolveAllWithKey[T any](c *Container, key string) ([]T, error) {
	resolutionContext := newResolutionContext(c)
	resolutionKey := resolutionKey{
		Key:        key,
		Type:       reflect.TypeFor[T](),
		ResolveAll: true,
	}
	value, err := resolve(resolutionContext, resolutionKey)
	if err != nil {
		return nil, err
	}
	return value.([]T), nil
}

// Tracks information about undergoing Resolve call.
type resolutionContext struct {
	// The container with which [Resolve] was called
	container *Container
	// This is to apply the injection lifetime rules.
	mainLifetime dependencyLifetime
	// This is to track nested resolutions to identify cyclic dependencies.
	stack []resolutionKey
	// This is to determine container to add disposables to.
	dependencyRegistration *dependencyRegistration
}

func newResolutionContext(c *Container) resolutionContext {
	resolutionContext := resolutionContext{
		container: c,
		stack:     []resolutionKey{},
	}
	return resolutionContext
}

func (rc resolutionContext) push(resolutionKey resolutionKey) resolutionContext {
	return resolutionContext{
		container:    rc.container,
		mainLifetime: rc.mainLifetime,
		stack:        append(rc.stack, resolutionKey),
	}
}

func (rc resolutionContext) isRoot() bool {
	return len(rc.stack) == 0
}

func (rc resolutionContext) resolutionKey() resolutionKey {
	return rc.stack[len(rc.stack)-1]
}

func (rc resolutionContext) dependencyGraphString() string {
	typesString := internal.Map(
		rc.stack,
		func(t resolutionKey) string { return fmt.Sprintf("%v", t) },
	)
	return strings.Join(typesString, " -> ")
}

func resolve(parentResolutionContext resolutionContext, resolutionKey resolutionKey) (any, error) {
	isDisposed, unlock := parentResolutionContext.container.disposed.Load()
	defer unlock()

	if isDisposed {
		return nil, fmt.Errorf("cannot resolve on a disposed container")
	}
	currentResolutionContext := parentResolutionContext.push(resolutionKey)

	container := currentResolutionContext.container

	// Checking if there is a registration.
	dependencyRegistration, found := container.getDependencyRegistration(resolutionKey.GetRegistrationKey())
	if !found {
		return nil, fmt.Errorf("no constructor registered for type %v", resolutionKey)
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
		resolutionLock, _ := cacheContainer.resolutionLocks.LoadOrStore(resolutionKey, &sync.Mutex{})
		// By acquiring a lock per resolution type we prevent multiple Singleton or Scoped instances
		resolutionLock.Lock()
		defer resolutionLock.Unlock()

		// Checking again for a cached instance because it could be created by other concurrent resolve calls
		cachedInstance, found := cacheContainer.instances.Load(resolutionKey)
		if found {
			return cachedInstance, nil
		}
	}

	// Resolution lifetime rules
	if dependencyRegistration.lifetime == scoped {
		if currentResolutionContext.mainLifetime == singleton {
			// cannot inject Scoped into Singleton
			return nil, fmt.Errorf("cannot inject Scoped (%v) dependencies in Singletons (%v)", resolutionKey, parentResolutionContext.resolutionKey())
		}
	}

	if dependencyRegistration.lifetime == scoped {
		if container.isRoot() {
			return nil, fmt.Errorf("cannot resolve Scoped dependencies from the root Container: %v", resolutionKey)
		}
	}

	constructors := dependencyRegistration.constructorInfos

	var value reflect.Value

	if resolutionKey.ResolveAll {
		// When resolutionType is a slice, we resolve all registered dependencies for it,
		// then return the result for it only if all are successful. Failing fast for simplicity.
		sliceType := reflect.SliceOf(resolutionKey.Type)
		sliceValue := reflect.MakeSlice(sliceType, len(constructors), len(constructors))
		for constructorIndex, constructor := range constructors {
			value, err := resolveSingle(currentResolutionContext, constructor, resolutionKey)
			if err != nil {
				return nil, err
			}

			sliceValue.Index(constructorIndex).Set(value)
		}
		value = sliceValue

	} else {
		// When resolutionType is not a slice, then resolve using the last registration.
		constructor := constructors[len(constructors)-1]

		singleValue, err := resolveSingle(currentResolutionContext, constructor, resolutionKey)
		if err != nil {
			return nil, err
		}
		value = singleValue
	}

	if cacheContainer != nil {
		cacheContainer.instances.Store(resolutionKey, value.Interface())
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
func resolveSingle(resolutionContext resolutionContext, constructor functionInfo, resolutionKey resolutionKey) (reflect.Value, error) {
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
	disposeCallback, found := disposeContainer.disposeCallbacks.Load(resolutionKey.Type)
	if found {
		disposeContainer.disposables.Append(&disposable{
			Value:   value.Interface(),
			Dispose: disposeCallback,
		})
	}

	return value, nil
}

type invokeMessageOptions struct {
	InvokeTargetType     string
	InvokeTargetFullName string
}

func invoke(function functionInfo, resolutionContext resolutionContext, messageOptions invokeMessageOptions) ([]reflect.Value, error) {
	functionType := function.Type
	if functionType.Kind() != reflect.Func {
		panic(fmt.Errorf("%s should be a function", messageOptions.InvokeTargetType))
	}
	callArguments := make([]reflect.Value, len(function.ArgumentTypes))
	for argumentIndex, argumentType := range function.ArgumentTypes {
		argumentResolutionKey := resolutionKey{
			Type: argumentType,
		}
		if argumentType.Kind() == reflect.Slice {
			argumentResolutionKey = resolutionKey{
				Type:       argumentType.Elem(),
				ResolveAll: true,
			}
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

// Registers a callback to be called on [Dispose]. The callback will be called for
// any instance resolved with the type [T], even those registered with key.
func OnDispose[T any](c *Container, callback func(ctx context.Context, value T)) {
	isDisposed, unlock := c.disposed.Load()
	defer unlock()

	if isDisposed {
		panic(fmt.Errorf("cannot register dispose callback on a disposed container"))
	}

	c.disposeCallbacks.Store(reflect.TypeFor[T](), func(ctx context.Context, value any) {
		callback(ctx, value.(T))
	})
}

type DisposeResult int

const (
	DisposeResultAlreadyDisposed DisposeResult = 0
	DisposeResultDone            DisposeResult = 1
	DisposeResultCtxDone         DisposeResult = 2
)

// Calls the callbacks registered with [OnDispose]. Callbacks are executed in parallel,
// each in a separate goroutine.
//
// It waits until all dispose callbacks are done or the context is done, whichever
// comes first.
//
// If container was already disposed, it's a noop returning [DisposeResultAlreadyDisposed].
//
// If all dispose callbacks finished executing before context is done, returns [DisposeResultDone].
//
// If context is done before all callbacks, return [DisposeResultCtxDone]
func Dispose(ctx context.Context, c *Container) DisposeResult {
	wasDisposed := c.disposed.Store(true)
	if wasDisposed {
		return DisposeResultAlreadyDisposed
	}

	var wg sync.WaitGroup
	disposables := c.disposables.Snapshot()
	for _, d := range disposables {
		wg.Add(1)
		go func(disposable *disposable) {
			defer wg.Done()
			disposable.Dispose(ctx, disposable.Value)
		}(d)
	}

	waitGroupDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitGroupDone)
	}()

	select {
	case <-waitGroupDone:
		return DisposeResultDone
	case <-ctx.Done():
		return DisposeResultCtxDone
	}
}

// Registers a callback to be called on [Start] for a Singleton dependency
// registered for the type [T].
//
// Calling [OnStart] on a non registered type [T] will panic.
//
// Start callback will be called in a goroutine, so it can be long running.
func OnStart[T any](c *Container, callback func(value T)) {
	resolutionKey := resolutionKey{
		Type: reflect.TypeFor[T](),
	}
	onStart(c, resolutionKey, func(value any) {
		callback(value.(T))
	})
}

// Same as [OnStart], but with a registration key.
func OnStartWithKey[T any](c *Container, key string, callback func(value T)) {
	resolutionKey := resolutionKey{
		Type: reflect.TypeFor[T](),
		Key:  key,
	}
	onStart(c, resolutionKey, func(value any) {
		callback(value.(T))
	})
}

// Same as [OnStart] but [Start] will be called for all registrations of [T]
func OnStartAll[T any](c *Container, callback func(value T)) {
	resolutionKey := resolutionKey{
		Type:       reflect.TypeFor[T](),
		ResolveAll: true,
	}
	onStart(c, resolutionKey, func(value any) {
		callback(value.(T))
	})
}

// Same as [OnStartWithKey] but [Start] will be called for all registrations of [T] with key
func OnStartAllWithKey[T any](c *Container, key string, callback func(value T)) {
	resolutionKey := resolutionKey{
		Type:       reflect.TypeFor[T](),
		Key:        key,
		ResolveAll: true,
	}
	onStart(c, resolutionKey, func(value any) {
		callback(value.(T))
	})
}

func onStart(c *Container, resolutionKey resolutionKey, callback startCallback) {
	isDisposed, unlock := c.disposed.Load()
	defer unlock()

	if isDisposed {
		panic(fmt.Errorf("cannot register start callback on a disposed container"))
	}

	isStarted, unlock := c.started.Load()
	defer unlock()

	if isStarted {
		panic(fmt.Errorf("cannot register start callback on a started container"))
	}

	if !c.isRoot() {
		panic(fmt.Errorf("cannot register start callback on a child container, only root allowed"))
	}

	registration, found := c.registry.Load(resolutionKey.GetRegistrationKey())
	if !found {
		panic(fmt.Errorf("you must register a dependency for %v before a start callback", resolutionKey))
	}
	if registration.lifetime != singleton {
		panic(fmt.Errorf("cannot register callback on a %v dependency, only %v allowed", registration.lifetime, singleton))
	}

	c.startCallbacks.Store(resolutionKey, callback)
}

// Calls the callbacks registered with [OnStart] and [OnStartWithKey].
// Callbacks are executed in parallel, each in a separate goroutine.
//
// Dependencies will be resolved automatically.
//
// Any error on resolutions will panic, as [Start] is intended to be
// called during application startup. It's assumed that the program
// should exit if the startup dependencies cannot be instantiated.
func Start(c *Container) {
	isDisposed, unlock := c.disposed.Load()
	defer unlock()

	if isDisposed {
		panic(fmt.Errorf("cannot start a disposed container"))
	}

	if !c.isRoot() {
		panic(fmt.Errorf("cannot start a child container, only root allowed"))
	}

	wasStarted := c.started.Store(true)
	if wasStarted {
		return
	}

	startCallbacks := c.startCallbacks.Snapshot()
	for resolutionKey, start := range startCallbacks {
		resolutionContext := newResolutionContext(c)

		resolvedValue, err := resolve(resolutionContext, resolutionKey)
		if err != nil {
			panic(fmt.Errorf("failed to start dependency %v: %w", resolutionKey, err))
		}

		if !resolutionKey.ResolveAll {
			go start(resolvedValue)
		} else {
			sliceValue := reflect.ValueOf(resolvedValue)
			if sliceValue.Kind() != reflect.Slice {
				panic(fmt.Errorf("failed to start dependency %v: expected resolved value to be a slice", resolutionKey))
			}

			for i := range sliceValue.Len() {
				value := sliceValue.Index(i)
				go start(value.Interface())
			}
		}

	}
}

// Clear all constructors registered with type [T] on this [Container]
//
// This can be used for overrides when you want to [ResolveAll]
func ClearRegistrations[T any](c *Container) {
	registrationKey := registrationKey{
		Type: reflect.TypeFor[T](),
	}
	c.registry.Remove(registrationKey)
}

// Clear all constructors registered with type [T] and key on this [Container]
//
// This can be used for overrides when you want to [ResolveAllWithKey]
func ClearRegistrationsWithKey[T any](c *Container, key string) {
	registrationKey := registrationKey{
		Type: reflect.TypeFor[T](),
		Key:  key,
	}
	c.registry.Remove(registrationKey)
}

// Clear all instances resolved with this [Container]
//
// This can be useful for tests
func ClearInstances(c *Container) {
	c.instances.Clear()
}
