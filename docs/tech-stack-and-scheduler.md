# Tech Stack & Scheduler Design Decision

Dokumen ini menjelaskan alasan teknis dibalik pemilihan *technology stack* yang digunakan dalam Tetra Engine, serta logika pembagian *scheduler* menjadi 4 job yang berbeda.

## 1. Pemilihan Tech Stack

Tetra Engine dibangun dengan fokus pada **stabilitas proses latar belakang** dan **akurasi data**.

| Komponen | Teknologi | Alasan Utama |
|---|---|---|
| **Bahasa Pemrograman** | **Golang** | Performa tinggi, manajemen memori efisien, dan dukungan *concurrency* (goroutine) yang sangat stabil untuk menjalankan banyak *scheduler* secara paralel. |
| **Dependency Injection** | **Uber-go/fx** | Mengatur siklus hidup aplikasi (*lifecycle*). Memastikan saat aplikasi dimatikan, *scheduler* berhenti secara bersih (*graceful shutdown*) tanpa memutus proses yang sedang berjalan di tengah jalan. |
| **Database Access** | **Sqlx** | *Lightweight* dan cepat. Kita memerlukan kontrol penuh atas *query* SQL untuk optimasi, namun tetap ingin kemudahan pemetaan data ke *struct* Go. |
| **Error Handling** | **Samber/oops** | Memberikan konteks dan *stack trace* pada error. Sangat penting untuk *scheduler* karena saat terjadi error di proses otomatis, kita perlu tahu persis di baris mana dan pada data mana error itu terjadi. |
| **Structured Logging** | **Slog** | Standar modern Go yang memungkinkan log disimpan dalam format JSON. Memudahkan pencarian log jika diintegrasikan dengan alat monitoring (ELK/Datadog). |

---

## 2. Desain Scheduler (4 Jobs)

Integrasi dibagi menjadi 4 tahap terpisah untuk menjaga **ketahanan sistem** (*resilience*). Jika satu tahap gagal (misal: API Flux sedang down), tahap lain tetap bisa berjalan dengan data yang sudah ada di database lokal.

### Job 1: Order Retrieval Sync (15 Menit)
*   **Alasan:** Menghindari beban berlebih pada API Flux. Siklus pesanan gudang biasanya tidak berubah dalam hitungan detik. Interval 15 menit adalah keseimbangan antara data yang *up-to-date* dan efisiensi jaringan.

### Job 2: Carton Master Sync (30 Menit)
*   **Alasan:** Katalog karton (*master data*) sangat jarang berubah. Ukuran kardus baru tidak ditambahkan setiap menit. Sinkronisasi setiap 30 menit atau lebih sudah sangat cukup.

### Job 3: Carton Recommendation (5 Menit)
*   **Alasan:** Ini adalah proses internal Tetra. Begitu order sudah ditarik (Job 1), kita ingin rekomendasi kardus segera dihasilkan secepat mungkin agar staff gudang tidak menunggu lama.

### Job 4: Carton Push (5 Menit)
*   **Alasan:** Begitu rekomendasi selesai (Job 3), hasilnya harus segera dikirim kembali ke Flux WMS agar staff di gudang bisa melihat kardus mana yang harus diambil saat proses *packing*.

---

## 3. Strategi Akurasi Data (Integer Math)

Program ini secara sadar menghindari tipe data `float` untuk dimensi dan berat:
*   **Panjang/Lebar/Tinggi:** Disimpan dalam **milimeter (mm)**.
*   **Berat:** Disimpan dalam **gram (g)**.

**Mengapa?**
Komputer seringkali memiliki masalah presisi saat menghitung angka desimal (misal: `0.1 + 0.2` bisa jadi `0.30000000004`). Dengan menggunakan angka bulat (*integer*), perhitungan volume dan berat di Tetra Engine menjadi **100% akurat** dan jauh lebih cepat diproses oleh CPU.

---

## 4. Keamanan & Resiliensi

*   **Idempotency:** Menggunakan status `SYNCED` -> `PENDING` -> `RECOMMENDED` -> `PUSHED`. Jika koneksi internet putus saat pengiriman, sistem hanya akan mengirim ulang data yang berstatus `RECOMMENDED` dan belum `PUSHED`. Tidak akan ada data ganda.
*   **Retry Mechanism:** HTTP Client dilengkapi dengan otomatisasi *retry* (3x percobaan) jika Flux API memberikan respon error sementara.
