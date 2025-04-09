package internal

func Map[T any, U any](values []T, mapFn func(value T) U) []U {
	result := make([]U, len(values))
	for i, value := range values {
		result[i] = mapFn(value)
	}
	return result
}

func Flat[T any](values [][]T) []T {
	flatten := []T{}
	for _, row := range values {
		flatten = append(flatten, row...)
	}
	return flatten
}
