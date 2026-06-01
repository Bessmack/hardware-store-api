package notifications

import "fmt"

// Registry holds all registered notification providers.
// Providers are registered once at startup in main.go;
// the registry is then injected into the NotificationService.
type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds a provider to the registry.
// Panics on duplicate name — misconfiguration should be caught at startup.
func (r *Registry) Register(p Provider) {
	if _, exists := r.providers[p.Name()]; exists {
		panic(fmt.Sprintf("notifications: provider %q is already registered", p.Name()))
	}
	r.providers[p.Name()] = p
}

// Get returns a provider by name. Returns an error if not found.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("notifications: provider %q not found", name)
	}
	return p, nil
}

// All returns every registered provider.
// Used by the service to fan out to all channels simultaneously.
func (r *Registry) All() []Provider {
	result := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}