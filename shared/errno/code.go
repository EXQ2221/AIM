package errno

const (
	Success          = 0
	ErrBadRequest    = 40000
	ErrUnauthorized  = 40100
	ErrForbidden     = 40300
	ErrInternalError = 50000

	ErrUserNotFound     = 40101
	ErrPasswordWrong    = 40102
	ErrUserNotAvailable = 40103
	ErrDuplicateUser    = 40901
)
