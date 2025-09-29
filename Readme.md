# AI CV Evaluator

Sebuah aplikasi backend untuk mengevaluasi CV dan laporan proyek secara otomatis menggunakan AI (Gemini / model generatif).  
Kandidat upload file (PDF atau DOCX), sistem mengekstrak informasi dari file tersebut, lalu mengevaluasi sesuai dengan job dan rubric yang sudah ditetapkan.

## Fitur Utama

- Upload 2 file (CV + Project) dalam satu permintaan HTTP  
- Ekstraksi otomatis informasi seperti pendidikan, pengalaman, keterampilan, sertifikasi  
- Evaluasi otomatis berdasarkan deskripsi pekerjaan dan rubric  
- Penyimpanan hasil evaluasi ke database  
- Seeding data job awal jika belum tersedia  

## Arsitektur

- **Gin** — HTTP framework  
- **GORM** — ORM untuk MySQL  
- **RabbitMQ** — antrean evaluasi (opsional, kalau worker digunakan)  
- **AI Client (Gemini / generative API)** — untuk ekstraksi & evaluasi  

## Persyaratan

- Go (versi stabil)  
- MySQL  
- RabbitMQ  
- API key Gemini / generative AI yang valid  

## Setup Proyek

1. Clone repositori  
   ```bash
   git clone https://github.com/ioarintoko/ai-cv-evaluator.git
   cd ai-cv-evaluator
   ```

2. Buat file `.env` di root, contoh:
   ```
   DB_DSN=user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
   GEMINI_API_KEY=your_api_key_here
   RABBITMQ_URL=amqp://guest:guest@localhost:5672/
   ```

3. Jalankan migrasi dan seeding otomatis (terjadi saat aplikasi mulai).  

4. Mulai aplikasi:
   ```bash
   go run cmd/main.go
   ```

## Endpoints

| Method | URL              | Deskripsi                                             |
|--------|------------------|-------------------------------------------------------|
| POST   | `/upload`         | Upload CV dan project (multipart/form-data)           |
| POST   | `/evaluate`       | Men-trigger evaluasi untuk upload yang sudah ada      |
| GET    | `/result/:id`     | Mengambil hasil evaluasi berdasarkan ID evaluasi      |

### Contoh upload di Postman

- Method: POST  
- URL: `http://localhost:8080/upload`  
- Body → form-data:
  - `candidate_name` (text)  
  - `candidate_email` (text)  
  - `cv_file` (file)  
  - `project_file` (file)  

## Struktur Direktori (Contoh)

```
cmd/
  main.go
domain/
  job.go
  upload.go
  evaluation.go
infrastructure/
  mysql.go
  gemini.go
  rabbitmq.go
interfaces/
  http_handler.go
.go.mod
.go.sum
README.md
```

## Penjelasan Seeding Job

Saat aplikasi pertama kali dijalankan:

- Akan menjalankan migrasi schema ke MySQL  
- Mengecek apakah tabel `jobs` kosong  
- Jika kosong, akan menambahkan 2 job default seperti contoh dalam tabel di issue  

Sehingga kamu tidak perlu input job secara manual pada awalnya.

## Catatan & Peringatan

- Model generatif (Gemini atau lainnya) memiliki batas token dan performa yang bervariasi saat file besar  
- Untuk file yang sangat besar, mungkin perlu dipecah atau dioptimasi  
- Pastikan API key dan koneksi RabbitMQ / MySQL berfungsi  
