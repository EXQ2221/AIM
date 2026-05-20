package errno

func BadRequest(message string) Error {
	return New(ErrBadRequest, "bad_request: "+message)
}

func Unauthorized(message string) Error {
	return New(ErrUnauthorized, "unauthorized: "+message)
}

func Forbidden(message string) Error {
	return New(ErrForbidden, "forbidden: "+message)
}

func Conflict(message string) Error {
	return New(ErrConflict, "conflict: "+message)
}

func NotFound(message string) Error {
	return New(ErrNotFound, "not_found: "+message)
}

func Internal(message string) Error {
	return New(ErrInternalError, "internal: "+message)
}

func Required(field string) Error {
	return BadRequest(field + " is required")
}
