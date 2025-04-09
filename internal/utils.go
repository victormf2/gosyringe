package internal

func Map[T any, U any](values []T, mapFn func(value T) U) []U {
	result := make([]U, len(values))
	for i, value := range values {
		result[i] = mapFn(value)
	}
	return result
}
