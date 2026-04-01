package tools

import (
	"fmt"
	"sync"
)

// ToolRegistry provides a thread-safe registry for MCP tools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolRegistry creates a new empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
// Returns an error if a tool with the same name is already registered.
func (r *ToolRegistry) Register(name string, tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}

	r.tools[name] = tool
	return nil
}

// Has returns true if a tool with the given name is registered.
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// Get returns the tool with the given name, or nil if not found.
func (r *ToolRegistry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.tools[name]
}

// List returns the names of all registered tools.
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}

// Remove removes a tool from the registry.
// Returns true if the tool was removed, false if it wasn't found.
func (r *ToolRegistry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return false
	}

	delete(r.tools, name)
	return true
}

// Clear removes all tools from the registry.
func (r *ToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools = make(map[string]Tool)
}

// ForEach iterates over all registered tools and calls the provided function.
// The function receives the tool name and the tool itself.
// Iteration stops if the function returns false.
func (r *ToolRegistry) ForEach(fn func(name string, tool Tool) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, tool := range r.tools {
		if !fn(name, tool) {
			break
		}
	}
}
