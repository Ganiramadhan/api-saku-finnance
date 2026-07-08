# SAKU load testing dengan k6

Paket ini menguji kapasitas HTTP aplikasi SAKU secara bertahap dan aman. Fokus utamanya adalah halaman publik dan request baca yang dipakai dashboard user. Secara default script tidak membuat transaksi, tidak mengirim email atau Telegram, tidak memanggil AI, dan tidak membuat checkout Midtrans.

## 1. Instalasi

macOS:

```bash
brew install k6
k6 version
```

Alternatif Docker:

```bash
docker run --rm -i grafana/k6 run - < api/load-tests/saku.js
```

Untuk konfigurasi dengan token file, instalasi native lebih praktis karena file dapat dibaca langsung.

## 2. Konsep pengujian

- **VU (virtual user)** adalah satu pengguna simulasi yang terus mengulang user journey selama tes.
- **Iteration** adalah satu putaran journey. Journey authenticated membuka tujuh endpoint dashboard secara paralel, membuka satu halaman tambahan, lalu menunggu 1–3 detik.
- **RPS** adalah jumlah request per detik yang benar-benar diterima server. Satu VU tidak sama dengan satu RPS.
- **p95** berarti 95% request selesai di bawah durasi tersebut.
- Kapasitas aman adalah level tertinggi yang masih memenuhi threshold, tidak membuat resource server jenuh, dan tetap stabil selama periode hold.

## 3. Profil yang tersedia

### Smoke

Tes paling ringan untuk memastikan script, DNS, TLS, reverse proxy, frontend, health endpoint, dan katalog paket dapat diakses.

```bash
k6 run \
  -e SAKU_BASE_URL=https://staging.example.com \
  api/load-tests/saku.js
```

Default: 2 VU selama 30 detik, tanpa login.

### Load

Mensimulasikan traffic dashboard normal. Gunakan akun khusus load test, bukan akun pribadi.

```bash
k6 run \
  -e SAKU_BASE_URL=https://staging.example.com \
  -e SAKU_PROFILE=load \
  -e SAKU_AUTHENTICATED=true \
  -e SAKU_TOKEN='JWT_AKUN_LOAD_TEST' \
  -e SAKU_VUS=25 \
  api/load-tests/saku.js
```

Default: naik ke 25 VU selama 1 menit, bertahan 5 menit, lalu turun selama 1 menit.

### Stress

Menaikkan beban bertahap untuk menemukan titik degradasi. Profil ini harus diaktifkan secara eksplisit.

```bash
k6 run \
  -e SAKU_BASE_URL=https://staging.example.com \
  -e SAKU_PROFILE=stress \
  -e SAKU_AUTHENTICATED=true \
  -e SAKU_TOKEN_FILE=./api/load-tests/secrets/tokens.json \
  -e SAKU_VUS=100 \
  -e SAKU_ALLOW_STRESS=true \
  api/load-tests/saku.js
```

Dengan `SAKU_VUS=100`, beban ditahan pada sekitar 25, 50, lalu 100 VU. Jangan langsung memulai dari angka besar.

## 4. Token dan akun pengujian

Metode yang disarankan adalah membuat beberapa akun khusus, login secara normal, lalu menyimpan JWT dalam:

```text
api/load-tests/secrets/tokens.json
```

Format file mengikuti `tokens.example.json`. Folder `secrets/` sudah diabaikan Git.

Script membagi token secara round-robin berdasarkan nomor VU. Lima puluh VU dengan sepuluh token berarti setiap akun digunakan oleh sekitar lima VU. Untuk hasil tenant/database yang lebih realistis, gunakan lebih banyak akun dan beri masing-masing data dummy yang wajar.

Untuk staging tanpa Turnstile, script juga dapat login sekali saat setup:

```bash
k6 run \
  -e SAKU_EMAIL=loadtest@example.com \
  -e SAKU_PASSWORD='password-khusus-test' \
  -e SAKU_PROFILE=load \
  -e SAKU_AUTHENTICATED=true \
  -e SAKU_BASE_URL=https://staging.example.com \
  api/load-tests/saku.js
```

Login tidak dilakukan oleh setiap VU karena endpoint login memiliki rate limiter. Di production, token file lebih aman dan lebih akurat untuk mengukur endpoint aplikasi.

## 5. Menguji production

Target `saku.ganipedia.com` ditolak kecuali izin production diberikan:

```bash
k6 run \
  -e SAKU_BASE_URL=https://saku.ganipedia.com \
  -e SAKU_ALLOW_PRODUCTION=true \
  -e SAKU_VUS=2 \
  -e SAKU_DURATION=30s \
  api/load-tests/saku.js
```

Mulai dari smoke test 1–2 VU. Pastikan:

1. Tidak ada deployment atau migrasi database yang sedang berjalan.
2. Monitoring CPU, RAM, disk I/O, database connection, Redis, dan error log terbuka.
3. Provider VPS/CDN mengizinkan traffic pengujian.
4. Ada waktu sepi dan prosedur stop jika error meningkat.
5. Endpoint AI, checkout, webhook, email, dan mutasi data tetap di luar skenario.

Untuk authenticated production smoke test:

```bash
k6 run \
  -e SAKU_BASE_URL=https://saku.ganipedia.com \
  -e SAKU_ALLOW_PRODUCTION=true \
  -e SAKU_AUTHENTICATED=true \
  -e SAKU_TOKEN='JWT_AKUN_TEST' \
  -e SAKU_VUS=1 \
  -e SAKU_DURATION=30s \
  api/load-tests/saku.js
```

## 6. Variabel konfigurasi

| Variabel | Default | Fungsi |
| --- | --- | --- |
| `SAKU_BASE_URL` | `http://localhost:8080` | Origin utama API/website |
| `SAKU_WEB_URL` | `SAKU_BASE_URL` | Origin frontend jika berbeda |
| `SAKU_API_URL` | `<base>/api/v1` | Base URL API jika berbeda |
| `SAKU_PROFILE` | `smoke` | `smoke`, `load`, atau `stress` |
| `SAKU_AUTHENTICATED` | aktif selain smoke | Menjalankan workload dashboard |
| `SAKU_VUS` | 2/25/100 | Target VU sesuai profil |
| `SAKU_DURATION` | `30s` | Durasi profil smoke |
| `SAKU_RAMP_UP` | `1m` | Ramp-up profil load |
| `SAKU_HOLD` | `5m` | Durasi stabil profil load |
| `SAKU_RAMP_DOWN` | `1m` | Ramp-down |
| `SAKU_THINK_TIME_MIN` | `1` | Jeda minimum antarsiklus |
| `SAKU_THINK_TIME_MAX` | `3` | Jeda maksimum antarsiklus |
| `SAKU_TOKEN` | kosong | Satu JWT |
| `SAKU_TOKENS` | kosong | Daftar JWT dipisahkan koma |
| `SAKU_TOKEN_FILE` | kosong | JSON array berisi JWT |
| `SAKU_ALLOW_PRODUCTION` | `false` | Izin eksplisit menguji production |
| `SAKU_ALLOW_STRESS` | `false` | Izin eksplisit menjalankan stress |

Jika frontend dan API berada di origin berbeda:

```bash
k6 run \
  -e SAKU_WEB_URL=https://staging.example.com \
  -e SAKU_BASE_URL=https://api-staging.example.com \
  -e SAKU_API_URL=https://api-staging.example.com/api/v1 \
  api/load-tests/saku.js
```

Gunakan flag `-e` untuk setiap variabel script. Ini memastikan nilainya tersedia melalui `__ENV` pada semua versi k6 yang didukung.

## 7. Threshold bawaan

Tes dianggap gagal jika salah satu kondisi berikut terjadi:

- error HTTP atau check aplikasi mencapai 1%;
- p95 seluruh request mencapai 800 ms;
- p99 seluruh request mencapai 1.500 ms;
- p95 endpoint transaksi mencapai 1.000 ms;
- p95 satu batch dashboard mencapai 2.500 ms.

Threshold adalah titik awal. Sesuaikan dengan SLO produk setelah memperoleh baseline nyata.

## 8. Cara mencari kapasitas aman

Jalankan load test berulang dengan urutan, misalnya:

```text
5 VU → 10 VU → 25 VU → 50 VU → 100 VU
```

Tahan setiap level minimal 5–10 menit. Catat:

- `http_reqs` dan RPS;
- p95/p99 latency;
- `http_req_failed` dan `saku_request_errors`;
- CPU dan RAM container API;
- jumlah koneksi aktif/menunggu pada PostgreSQL;
- Redis latency dan hit/miss bila tersedia;
- response 429, 500, 502, 503, dan timeout;
- query lambat serta lock database.

Contoh interpretasi:

```text
25 VU: p95 320 ms, error 0%, CPU 45%
50 VU: p95 610 ms, error 0.2%, CPU 72%
75 VU: p95 1.4 s, error 3%, CPU 96%
```

Dalam contoh tersebut breakpoint berada di antara 50–75 VU. Kapasitas operasional sebaiknya diberi headroom sekitar 30–50%, sehingga target aman mungkin sekitar 35–40 VU sampai bottleneck diperbaiki.

## 9. Menyimpan hasil

```bash
mkdir -p api/load-tests/results

k6 run \
  -e SAKU_BASE_URL=https://staging.example.com \
  -e SAKU_PROFILE=load \
  -e SAKU_AUTHENTICATED=true \
  -e SAKU_TOKEN_FILE=./api/load-tests/secrets/tokens.json \
  --summary-export=api/load-tests/results/load-25vus.json \
  api/load-tests/saku.js
```

Jangan commit hasil besar atau token. Kedua folder sudah masuk `.gitignore`.

## 10. Batasan

k6 script ini mengukur HTTP/API, bukan rendering React, Web Vitals, atau performa browser nyata. Gunakan Lighthouse atau browser performance testing secara terpisah untuk LCP, CLS, INP, dan ukuran bundle.

Hasil dari laptop juga dibatasi bandwidth dan CPU laptop. Untuk beban besar, jalankan k6 dari mesin terpisah yang dekat dengan region server atau gunakan distributed load generation. Jangan menjalankan generator beban di VPS yang sama dengan aplikasi karena hasilnya akan bias.
