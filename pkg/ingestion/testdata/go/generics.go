package sample

// Map applies a function to each element of a slice.
func Map[T, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

// Container holds a value of any type.
type Container[T any] struct {
	value T
}

// Get returns the contained value.
func (c *Container[T]) Get() T {
	return c.value
}
