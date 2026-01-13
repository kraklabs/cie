package sample

// ProcessData processes data with a callback.
func ProcessData(data []int) []int {
	transform := func(x int) int {
		return x * 2
	}

	result := make([]int, len(data))
	for i, v := range data {
		result[i] = transform(v)
	}
	return result
}

// Filter filters a slice.
func Filter(data []int, predicate func(int) bool) []int {
	var result []int
	for _, v := range data {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}
