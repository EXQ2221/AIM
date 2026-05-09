package response

type BaseResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func Success(data any) BaseResponse {
	return BaseResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	}
}

func Error(code int, message string) BaseResponse {
	return BaseResponse{
		Code:    code,
		Message: message,
	}
}
