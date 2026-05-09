package errno

type Error struct {
	Code    int
	Message string
	UserID  uint64
}

func (e Error) Error() string {
	return e.Message
}

func New(code int, message string) Error {
	return Error{
		Code:    code,
		Message: message,
	}
}

func NewWithUser(code int, message string, userID uint64) Error {
	return Error{
		Code:    code,
		Message: message,
		UserID:  userID,
	}
}
