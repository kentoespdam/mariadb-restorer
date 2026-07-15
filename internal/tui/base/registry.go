package base

// FactoryContext provides common context for creating screens at runtime.
type FactoryContext struct {
	DataDir string
	Demo    bool
}

// ScreenFactory creates a Screen from the given context.
type ScreenFactory func(ctx FactoryContext) Screen

var factories = map[ScreenID]ScreenFactory{}

// RegisterFactory registers a factory for the given ScreenID.
// Called during init() from screen packages or the central registration point.
func RegisterFactory(id ScreenID, fn ScreenFactory) {
	factories[id] = fn
}

// CreateScreen creates a screen by ID using the registered factory.
// Returns nil, false if no factory is registered for the given ID.
func CreateScreen(id ScreenID, ctx FactoryContext) (Screen, bool) {
	fn, ok := factories[id]
	if !ok {
		return nil, false
	}
	return fn(ctx), true
}
