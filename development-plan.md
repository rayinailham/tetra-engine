# Tetra Recommendation Engine - Development Plan

Berdasarkan **Tetra Architecture Design (v1.1)** dan standar dari **cc-skills-golang**, berikut adalah *Development Plan* komprehensif untuk membangun Tetra Engine. Rencana ini memastikan *vibe-coding* yang lancar, maintainable, performan, dan sesuai dengan best practice Golang terkini.

---

## 🏗️ Phase 1: Project Setup & Core Infrastructure (Foundation)
**Tujuan:** Membangun fondasi aplikasi yang solid dengan Dependency Injection, Configuration, dan Logging yang terstruktur.

1. **Project Layout Initialization**
   - Menggunakan standar Go project layout (`cmd/`, `internal/`, `pkg/`).
   - *Skill Ref:* `golang-project-layout`
2. **CLI & Configuration Layer**
   - Menggunakan `spf13/cobra` untuk manajemen command line (misal: `tetra server`, `tetra migrate`, `tetra worker`).
   - Menggunakan `spf13/viper` untuk membaca konfigurasi dari environment variables (`.env`) atau file config.
   - *Skill Ref:* `golang-spf13-cobra`, `golang-spf13-viper`
3. **Dependency Injection & Lifecycle Management**
   - Menggunakan `uber-go/fx` untuk wiring dependency secara otomatis (database, HTTP server, client, logger) dan mengatur *graceful shutdown* untuk scheduler.
   - *Skill Ref:* `golang-uber-fx`
4. **Structured Logging & Error Handling**
   - Implementasi `log/slog` bawaan Go dikombinasikan dengan package dari `samber/slog-multi` atau formatter terkait untuk logging terstruktur (JSON di production, text di lokal).
   - Menggunakan `samber/oops` untuk error wrapping, stack traces, dan error context (sangat berguna untuk debugging scheduler).
   - *Skill Ref:* `golang-samber-slog`, `golang-samber-oops`, `golang-error-handling`

---

## 🗄️ Phase 2: Database Layer & Persistence
**Tujuan:** Menyiapkan struktur database lokal dan layer akses data.

1. **Database Connection & Migration**
   - Memilih SQL driver (PostgreSQL/MySQL).
   - Menggunakan `database/sql` dengan ekstensi `sqlx` atau driver native (`pgx` untuk Postgres) untuk *struct scanning*.
   - Setup `golang-migrate/migrate` untuk manajemen skema database (ERD `orders`, `order_items`, `cartons`, `scheduler_logs`).
   - *Skill Ref:* `golang-database`
2. **Repository Pattern Implementation**
   - Membuat interface dan implementasi repository untuk entitas:
     - `OrderRepository` (CRUD, filtering by status `SYNCED`, `PENDING`, dll.)
     - `CartonRepository` (Sync data, active cartons)
     - `LogRepository` (Mencatat history eksekusi scheduler)
   - Memastikan koneksi aman, menggunakan parameterized query, dan *Context Propagation* untuk timeout.
   - *Skill Ref:* `golang-context`, `golang-structs-interfaces`

---

## 🌐 Phase 3: External API Integration (Flux WMS Client)
**Tujuan:** Membuat HTTP Client yang tangguh untuk berkomunikasi dengan Flux WMS (`mock-api-anteraja.vercel.app`).

1. **HTTP Client Setup**
   - Membuat base HTTP Client dengan konfigurasi *timeout* yang proper.
   - *Skill Ref:* `golang-safety`, `golang-context`
2. **Implementasi API Endpoints**
   - `GetOrders()` - GET `/orders`
   - `GetOrderDetail(orderID)` - GET `/orders/{id}`
   - `GetCartons()` - GET `/cartons`
   - `AssignCarton(payload)` - POST `/orders/carton`
3. **Resilience & Error Handling**
   - Menambahkan retry mechanism untuk request network (bisa menggunakan package retry sederhana atau custom roundtripper).
   - Memastikan unit konversi terhandle (menerima data string decimal dan mapping ke struct Go).

---

## 🧠 Phase 4: Core Recommendation Engine
**Tujuan:** Mengimplementasikan *business logic* utama untuk menentukan karton.

1. **Recommendation Service**
   - Mengambil data order dengan status `PENDING`.
   - Mengambil seluruh *active cartons*.
2. **Kalkulasi & Validasi (Sesuai Spec)**
   - *Volume Calculation*: `Panjang × Lebar × Tinggi × Qty`.
   - *Weight Calculation*: `Berat × Qty` (Konversi berat *gram* ke *kilogram* untuk dibandingkan dengan carton `max_weight`).
   - *Algorithm*: Filter karton yang cukup untuk menampung seluruh volume & berat, lalu urutkan berdasarkan volume terkecil (efisiensi).
3. **Unit Testing**
   - Menggunakan `stretchr/testify` untuk *Table-Driven Tests*. Uji edge cases (barang terlalu besar, pas-pasan, melebihi max weight, dll).
   - *Skill Ref:* `golang-testing`, `golang-stretchr-testify`

---

## ⚙️ Phase 5: Schedulers & Background Workers
**Tujuan:** Menjalankan alur integrasi secara otomatis sesuai jadwal tanpa campur tangan user.

1. **Scheduler Setup**
   - Menggunakan `go-co-op/gocron` (atau standard `time.Ticker` di dalam Goroutine) yang di-register ke `fx.Lifecycle`.
2. **Implementasi 4 Job Utama**
   - **Job 1: Order Retrieval Sync** (Setiap 15 Menit) -> Sync orders, ubah jadi `SYNCED` lalu `PENDING`.
   - **Job 2: Carton Master Sync** (Periodik) -> Update/Insert tabel `cartons`.
   - **Job 3: Carton Recommendation** (Periodik) -> Proses orders `PENDING`, ubah status ke `RECOMMENDED`.
   - **Job 4: Carton Push** (Periodik) -> Hit API POST `/orders/carton`, update status jadi `PUSHED`.
3. **Observability**
   - Menulis eksekusi setiap job ke tabel `scheduler_logs`.
   - Tangkap panic di dalam goroutine menggunakan defensive programming (`samber/oops` atau `recover()`).
   - *Skill Ref:* `golang-concurrency`, `golang-safety`

---

## 📡 Phase 6: HTTP Server (Monitoring & Webhooks - Jika Diperlukan)
**Tujuan:** Menyediakan interface HTTP (Sesuai spesifikasi yang menyebutkan Gin/Echo).

1. **Web Framework Setup**
   - Menggunakan `gin-gonic/gin` atau `labstack/echo`.
2. **Endpoints**
   - `GET /health` : Liveness/Readiness probe (cek koneksi DB).
   - `POST /api/internal/trigger/{job_name}` : Endpoint opsional untuk men-trigger scheduler secara manual via admin (sangat mempermudah testing & operation).

---

## 🛡️ Phase 7: Code Quality & CI/CD Pipeline
**Tujuan:** Memastikan standar kode terjaga.

1. **Linting**
   - Setup `.golangci.yml` dengan linter yang disarankan (`golangci-lint`).
   - *Skill Ref:* `golang-lint`
2. **GitHub Actions (Opsional jika pakai repo Github)**
   - Setup pipeline untuk Run Tests, Linter, & Build Check.
   - *Skill Ref:* `golang-continuous-integration`

---

### Saran Urutan Eksekusi (*Vibe-Coding Path*)
Untuk memulai development dengan enak, saya sarankan kita kerjakan dengan urutan iteratif berikut:
1. **Init repo & setup folder structure + FX + Logger**.
2. **Setup Database + migrations** (Buat ERD jadi tabel nyata).
3. **Buat Model & Repository** (CRUD logic).
4. **Buat Flux API Client** (Test hit API mock-nya).
5. **Buat Core Logic Recommendation** + Unit test-nya.
6. **Bungkus semuanya ke Schedulers**.

