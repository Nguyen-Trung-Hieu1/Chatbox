package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("tài khoản hoặc mật khẩu không chính xác")
	ErrUsernameExists     = errors.New("tên đăng nhập đã tồn tại")
	ErrUnauthorized       = errors.New("phiên đăng nhập không hợp lệ hoặc đã hết hạn")
	ErrForbidden          = errors.New("bạn không có quyền truy cập cuộc trò chuyện này")
	ErrInvalidID          = errors.New("conversation_id không hợp lệ")
	ErrInvalidTitle       = errors.New("tên cuộc trò chuyện phải có từ 1 đến 100 ký tự")
	ErrNotFound           = errors.New("không tìm thấy dữ liệu")
	ErrDuplicate          = errors.New("dữ liệu đã tồn tại")
	ErrModelForbidden     = errors.New("gói của bạn không được dùng model này")
	ErrQuotaExceeded      = errors.New("hạn mức còn lại không đủ cho yêu cầu này")
	ErrInvalidContent     = errors.New("tin nhắn quá dài")
	ErrInvalidAPIKeyName  = errors.New("tên API key phải có từ 1 đến 80 ký tự")
	ErrInvalidPlan        = errors.New("gói phải là free hoặc paid")
)
