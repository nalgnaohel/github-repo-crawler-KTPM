# CASE STUDY 5

[Gitstar ranking](https://gitstar-ranking.com/repositories) là một trang web thú vị để thống kê các trang được đánh giá sao nhiều nhất trên Github. Nhiệm vụ trong bài này là dựng một crawler có thể thu thập được thông tin các bản release của 5000 repository nhiều sao nhất Github.

## 🚀 Hướng dẫn cài đặt

### 1. Clone repository

```bash
git clone <your-repo-url>
cd <ten-thu-muc-repo>
```

### 2. Di chuyển vào thư mục thực nghiệm muốn chạy

```bash
cd <1 trong 4 foler>
```

### 3. Khởi tạo dữ liệu

Trong thư mục thực nghiệm thường có một thư mục `setup-data` hoặc mục tên khác nhưng có docker-compose file là được

```bash
cd setup-data
docker-compose up
```

Sau khi xong, quay lại thư mục thực nghiệm:

```bash
cd ..
go run cmd/main.go
```

Lệnh trên sẽ khởi chạy server tại `localhost:<port>`.

---

## 📡 API có sẵn

Sau khi server khởi động, bạn có thể gọi các API như sau:

### Repositories
- `GET /api/repos/crawl`: crawl toàn bộ repositories
- `GET /api/repos/{repoID}`: lấy thông tin một repository

### Releases
- `GET /api/releases/crawl`: crawl toàn bộ releases
- `GET /api/releases/{releaseID}`: lấy thông tin một release
- `GET /api/releases/{releaseID}/commits`: crawl commit theo release

### Commits
- `GET /api/commits/crawl`: crawl toàn bộ commits
- `GET /api/commits/{commitID}`: lấy thông tin một commit

---

## 📝 Lưu ý

- Log hệ thống được lưu tại thư mục `logs` trong từng thực nghiệm.
  
## ⚙️ Công nghệ sử dụng

- **Go (Golang)**: ngôn ngữ lập trình chính để xây dựng server và các thành phần logic
- **[Colly](https://github.com/gocolly/colly)**: thư viện crawler mạnh mẽ cho Go
- **[Chi Router](https://github.com/go-chi/chi)**: router HTTP nhẹ và nhanh
- **[Logrus](https://github.com/sirupsen/logrus)**: logging framework
- **[Viper](https://github.com/spf13/viper)**: quản lý cấu hình ứng dụng
- **[GORM](https://gorm.io/)**: ORM tương tác với cơ sở dữ liệu
- **Docker Compose**: phục vụ việc khởi tạo cơ sở dữ liệu dễ dàng qua `setup-data`

## 🧱 Kiến trúc & thiết kế

- **Queue-Based Load Leveling**: dữ liệu được đưa vào hàng đợi (queue) thay vì ghi trực tiếp vào DB, giúp tăng tốc độ crawl và giảm tải cho DB
- **Circuit Breaker Pattern**

---
  
# Solution

## Kết quả thực nghiệm
<table>
  <thead>
    <tr>
      <th> </th>
      <th colspan="3">Repos </th>
      <th colspan="3">Releases </th>
      <th colspan="3">Commits</th>
    </tr>
    <tr>
      <!-- Dòng header thứ hai để đánh tên hai cột con của Col B -->
      <th></th>
      <th>crawled</th>
      <th>time (s)</th>
      <th>%error</th>
      <th>crawled</th>
      <th>time (s) </th>
      <th>%error</th>
      <th>crawled</th>
      <th>time (s) </th>
      <th>%error</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Baseline</td>
      <td>5000</td>
      <td>17.783</td>
      <td>0%</td>
      <td>1890</td>
      <td>1h53</td>
      <td>0%</td>
      <td>_</td>
      <td>_</td>
      <td>0%</td>
    </tr>
    <tr>
      <td>Exp 1</td>
      <td>5000</td>
      <td>5.3</td>
      <td>0%</td>
      <td>45837</td>
      <td>15p34</td>
      <td>0%</td>
      <td>36822</td>
      <td>13p56</td>
      <td>0%</td>
    </tr>
    <tr>
      <td>Exp 2</td>
      <td>5000</td>
      <td>4.3</td>
      <td>0%</td>
      <td>25690</td>
      <td>8p28</td>
      <td>0%</td>
      <td>37682</td>
      <td>12p55</td>
      <td>0%</td>
    </tr>
    <tr>
      <td>Exp 3</td>
      <td>5000</td>
      <td>6.5</td>
      <td>0%</td>
      <td>9835</td>
      <td>6p28</td>
      <td>0%</td>
      <td>6570</td>
      <td>4p14</td>
      <td>0%</td>
    </tr>
  </tbody>
</table>

# Mô tả từng thử nghiệm
## Baseline
Baseline là một crawler siêu đơn giản, chỉ có thể cào dữ liệu đơn thuần tự động, mà chưa có bất kỳ xử lý giúp tối ưu về mặt thời gian và lượng dữ liệu crawled được. 
Các vấn đề baseline này gặp phải:
- 

## Exp 1
Crawl đa luồng (thực nghiệm 4 - 10 luồng), đồng thời sử dụng batch để cho phép ghi batch 100 records cùng 1 lúc.
=> Các cải tiến:
1. **Tận dụng đỗ trễ mạng**  
   - Tạo nhiều đồng thời, tận dụng tối đa độ trễ mạng từ đó rút ngắn thời gian crawl

2. **Ổn định hơn so với 1 luồng đơn**  
   - Nếu một luồng bị block (timeout, delay), các luồng khác vẫn tiếp tục hoạt động, ngăn tình trạng “điểm chết” toàn bộ quá trình crawl so với việc chỉ sử dụng mỗi 1 luồng như baseline.

4. **Giảm số lượng truy vấn DB nhờ batch insert**  
   - Gom 100 kết quả crawl vào một lô (batch) trước khi gọi `INSERT`/`COPY` một lần.  
   - Sử dụng transaction đảm bảo tính nhất quán của dữ liệu

5. **Tăng tốc độ ghi & giảm latency tail**  
   - Việc ghi 100 bản ghi cùng lúc tận dụng tốt I/O throughput, giảm IOPS so với ghi rải rác từng bản ghi.  
   - Giảm thời gian chờ đợi cho mỗi lô dữ liệu, giúp crawler không phải chờ quá lâu giữa các batch.

## Exp 2
Crawl dùng queue, các data crawl cào về được nhét vào queue để đợi khi nào database rảnh thì sẽ thực hiện ghi vào db.
=> Các cải tiến đạt được:
1. **Tăng throughput cho crawler**  
   - Crawler chỉ cần đẩy kết quả vào queue mà không phải chờ ghi xong vào DB => Giảm thời gian chờ, việc crawl được thực hiện liên tục từ đó giảm thời gian crawl xuống  

2. **Điều tiết tải (Back‑pressure)**  
   - Queue lưu trữ lượng data chờ ghi. Khi DB bận, consumer giảm tốc độ ghi tự động, crawler vẫn tiếp tục (đến ngưỡng queue).  
