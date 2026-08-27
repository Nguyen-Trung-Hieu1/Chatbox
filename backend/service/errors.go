package service

import "errors"

var (
	ErrInvalidCredentials = errors.New("tài khoản hoặc mật khẩu không chính xác")
	ErrUsernameExists     = errors.New("tên đăng nhập đã tồn tại")
	ErrUnauthorized       = errors.New("phiên đăng nhập không hợp lệ hoặc đã hết hạn")
	ErrForbidden          = errors.New("bạn không có quyền truy cập cuộc trò chuyện này")
	ErrInvalidID          = errors.New("conversation_id không hợp lệ")
	ErrInvalidTitle       = errors.New("tên cuộc trò chuyện phải có từ 1 đến 100 ký tự")
)
