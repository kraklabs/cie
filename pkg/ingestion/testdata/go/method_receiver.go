package sample

// Handler handles HTTP requests.
type Handler struct {
	name string
}

// HandleRequest processes an HTTP request.
func (h *Handler) HandleRequest(path string) error {
	return nil
}

// GetName returns the handler name.
func (h Handler) GetName() string {
	return h.name
}
