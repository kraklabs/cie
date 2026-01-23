package sample

// Base provides common functionality.
type Base struct {
	ID   int
	Name string
}

// GetID returns the ID.
func (b *Base) GetID() int {
	return b.ID
}

// Extended embeds Base and adds more fields.
type Extended struct {
	Base
	Extra string
}

// GetName returns the name.
func (e *Extended) GetName() string {
	return e.Name
}
