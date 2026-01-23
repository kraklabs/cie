package sample

// helper is a helper function.
func helper(x int) int {
	return x * 2
}

// Process processes a value.
func Process(x int) int {
	return helper(x) + 1
}

// Chain calls multiple functions.
func Chain(x int) int {
	a := Process(x)
	b := helper(a)
	return b + Process(b)
}
