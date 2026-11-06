package app

func publicValue[T any](value T, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, publicError(err)
	}
	return value, nil
}
