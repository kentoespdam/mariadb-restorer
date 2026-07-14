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

// ShortcutInfo defines a global keyboard shortcut handled by the router.
type ShortcutInfo struct {
	Key      string   // keyboard key to match
	Desc     string   // human-readable description for help display
	TargetID ScreenID // screen to navigate to (0 = no navigation)
	OnlyOn   ScreenID // only works when this screen is active (0 = any screen)
	Quit     bool     // quit the application
	Back     bool     // navigate back (pop stack)
	Home     bool     // navigate to home (pop to first screen)
}

var shortcuts []ShortcutInfo

// RegisterShortcut registers a global keyboard shortcut.
// Called during init() from the central registration point.
func RegisterShortcut(info ShortcutInfo) {
	// Avoid duplicates if init() runs multiple times (e.g., in tests).
	for i, existing := range shortcuts {
		if existing.Key == info.Key {
			shortcuts[i] = info
			return
		}
	}
	shortcuts = append(shortcuts, info)
}

// Shortcuts returns all registered global shortcuts.
func Shortcuts() []ShortcutInfo {
	return shortcuts
}
