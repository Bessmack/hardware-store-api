package payments

import "fmt"

// Registry holds all registered payment providers.
// Registered once at startup in main.go; injected into the PaymentService.
type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds a provider. Panics on duplicate name — startup misconfiguration
// should be caught immediately, not silently ignored at runtime.
func (r *Registry) Register(p Provider) {
	if _, exists := r.providers[p.Name()]; exists {
		panic(fmt.Sprintf("payments: provider %q is already registered", p.Name()))
	}
	r.providers[p.Name()] = p
}

// Get returns a provider by name. Returns an error if not registered.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("payments: provider %q not found — is it registered in main.go?", name)
	}
	return p, nil
}

// Names returns all registered provider names.
// Used by the checkout endpoint to list available payment methods.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}