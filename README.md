# CASE STUDY 5

[Gitstar ranking](https://gitstar-ranking.com/repositories) là một trang web thú vị để thống kê các trang được đánh giá sao nhiều nhất trên Github. Nhiệm vụ trong bài này là dựng một crawler có thể thu thập được thông tin các bản release của 5000 repository nhiều sao nhất Github.

## Gợi ý triển khai

Ngoài cách crawl trên trang chủ, có thể sử dụng [API này](https://docs.github.com/en/rest) để thu thập dữ liệu cần sử dụng. Các bạn có thể dùng các công cụ như [scrapy](https://scrapy.org/) (Python), [cheerio](https://github.com/cheeriojs/cheerio) (NodeJS), [Selenium](https://www.selenium.dev/), v.v.

Các trang web trên có thể chặn lưu lượng truy cập bất thường dù dùng thông qua API chính chủ, với vấn đề này có thể sử dụng proxy, VPN hoặc Tor, v.v.

## Dữ liệu

Các thông tin cần thu thập bao gồm tên bản release, nội dung release và các commit thay đổi trong bản release đó. Schema của cơ sở dữ liệu mẫu nằm trong file `db.sql`.

## Yêu cầu triển khai

| Mức độ | Mô tả |
|--|--|
| ![Static Badge](https://img.shields.io/badge/REQUIRED-easy-green) | Triển khai được crawler cơ bản, thu thập tự động (có thể bị chặn) |
| ![Static Badge](https://img.shields.io/badge/REQUIRED-easy-green) | Đánh giá và nêu nguyên nhân của các vấn đề gặp phải |
| ![Static Badge](https://img.shields.io/badge/REQUIRED-hard-red) | Cải tiến và so sánh hiệu năng với phiên bản ban đầu |
| ![Static Badge](https://img.shields.io/badge/OPTIONAL-easy-green) | Tối ưu quá trình đọc ghi database |
| ![Static Badge](https://img.shields.io/badge/OPTIONAL-medium-yellow) | Song song hoá (đa luồng) quá trình crawl |
| ![Static Badge](https://img.shields.io/badge/OPTIONAL-medium-yellow) | Giải quyết vấn đề crawler bị trang web chặn khi truy cập quá nhiều bằng một số kỹ thuật hoặc design pattern tương ứng |
| ![Static Badge](https://img.shields.io/badge/OPTIONAL-medium-yellow) | Đánh giá các giải pháp tối ưu khác nhau |
  
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
      <td>1 200</td>
      <td>0%</td>
      <td>100%</td>
      <td>6</td>
      <td>0%</td>
      <td>100%</td>
      <td>7</td>
      <td>0%</td>
    </tr>
    <tr>
      <td>Exp 1</td>
      <td>100%</td>
      <td>5.3</td>
      <td>0%</td>
      <td>100%</td>
      <td>6.6</td>
      <td>0%</td>
      <td>100%</td>
      <td>7</td>
      <td>0%</td>
    </tr>
    <tr>
      <td>Exp 2</td>
      <td>100%</td>
      <td>4.3</td>
      <td>0%</td>
      <td>100%</td>
      <td>6</td></td>
      <td>0%</td>
      <td>100%</td>
      <td>7</td>
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
