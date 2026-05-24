package utils

func SliceToAny[T any](src []T) []any {
	result := make([]any, len(src))
	for i := range src {
		result[i] = (any)(src[i])
	}

	return result
}
