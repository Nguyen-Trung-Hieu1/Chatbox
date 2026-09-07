# Chatbox1 trong K3s trên WSL

Mở http://localhost:8088 trên Windows, đăng nhập bằng tài khoản Chatbox1 cũ.
Đây là bản chạy cục bộ trên máy này, chưa xuất bản lên Internet.

## Khởi động và kiểm tra

Sau khi khởi động lại Windows, mở Ubuntu bằng PowerShell:

```powershell
wsl -d Ubuntu-24.04
```

K3s và dịch vụ truy cập web được bật tự động khi Ubuntu chạy. Kiểm tra trong Ubuntu:

```bash
sudo k3s kubectl get pods -n chatbox1
sudo systemctl status chatbox1-web --no-pager
```

Luồng yêu cầu: trình duyệt → localhost:8088 → giao diện Nginx → backend →
MongoDB hoặc 9router → dịch vụ AI. Các dịch vụ dữ liệu chỉ có ClusterIP nội bộ.
Không cần bật bản backend hay 9router trên Windows để bản K3s gọi AI.

## Cấu hình và dữ liệu

- `local-data.yaml`: MongoDB và 9router, dùng thư mục dữ liệu cố định trên một máy WSL.
- `local-app.yaml`: backend và giao diện, dùng ba image `chatbox1-*:local` đã nhập vào K3s.
- `chatbox1-web.service`: chuyển tiếp cổng 8088 trên localhost tới giao diện, tự kết nối lại khi pod thay đổi.
- Dữ liệu đang chạy: `/var/lib/chatbox1-data/` trong Ubuntu.
- Bản sao lưu và cấu hình riêng: `/var/lib/chatbox1-migration/20260906/` trong Ubuntu, cần sudo.
- Các Secret `mongodb-auth` và `chatbox-backend-config` chứa thông tin kết nối riêng; không xuất chúng vào Git.

Bản dữ liệu Windows được giữ nguyên. Hai bản không tự đồng bộ: tin nhắn mới
trong bản K3s chỉ lưu vào MongoDB của K3s. Không chạy lại các script sao lưu/phục
hồi để cập nhật ứng dụng. Những script đó phục vụ lần chuyển dữ liệu ban đầu.

## Kiểm tra đã thực hiện

- HTTP giao diện trả 200 từ Windows.
- Đăng ký tài khoản thử, đăng nhập bằng cookie và kiểm tra phiên đăng nhập.
- Gửi một tin nhắn qua backend → 9router → AI; nhận sự kiện delta và done.
- Đọc lại lịch sử có câu trả lời AI; dọn riêng tài khoản và dữ liệu thử.

Argo CD chưa được cài. Cấu hình này dành cho thực hành trên một máy, chưa phải
cấu hình nhiều máy hay triển khai công khai. Các image local cần được build và
nhập lại vào K3s khi mã ứng dụng thay đổi.

## Sau khi Windows ngủ hoặc WSL bị dừng

K3s trong WSL có thể giữ trạng thái container cũ sau khi máy ngủ. Nếu trang mở
được nhưng chat báo `Failed to fetch`, hoặc localhost:8088 không mở được, chạy:

```powershell
powershell -ExecutionPolicy Bypass -File E:\Chatbox1\scripts\restart-chatbox.ps1
```

Script chỉ khởi động lại WSL khi HTTP 8088 đang lỗi, sau đó chờ tối đa 90 giây.
