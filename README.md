# GOWA Broadcast - Go WhatsApp Broadcast Application

Aplikasi Go yang ringan memori untuk broadcast pesan WhatsApp menggunakan library `whatsmeow`. Aplikasi ini dirancang untuk berjalan di Docker VPS dengan efisiensi memori yang optimal.

## Fitur Utama

### 🚀 Core Features
- **WhatsApp Multi-Device Support** - Menggunakan library `whatsmeow` untuk koneksi WhatsApp Web
- **Broadcast Management** - Kirim pesan ke multiple kontak/grup secara bersamaan
- **Scheduled Messages** - Jadwalkan pengiriman pesan untuk waktu tertentu
- **Rate Limiting** - Kontrol kecepatan pengiriman untuk menghindari spam detection
- **Memory Efficient** - Optimasi penggunaan memori untuk VPS dengan resource terbatas

### 📊 Management Features
- **Contact & Group Management** - Kelola kontak dan grup WhatsApp
- **Broadcast Lists** - Buat dan kelola daftar penerima broadcast
- **Message History** - Riwayat pesan yang dikirim dan diterima
- **Statistics Dashboard** - Statistik penggunaan dan performa
- **Webhook Support** - Integrasi dengan sistem eksternal

### 🔧 Technical Features
- **REST API** - API lengkap untuk semua operasi
- **Multi-User Support** - Sistem autentikasi JWT dengan role-based access
- **Database Support** - SQLite (default) dan PostgreSQL
- **Docker Ready** - Siap deploy dengan Docker
- **Environment Configuration** - Konfigurasi melalui environment variables
- **Logging** - Comprehensive logging untuk monitoring

### 👥 Multi-User Features
- **JWT Authentication** - Secure token-based authentication
- **Role-Based Access Control** - Admin dan User roles dengan permission berbeda
- **Data Isolation** - Setiap user hanya dapat mengakses data miliknya
- **User Management** - CRUD operations untuk manajemen user (admin only)
- **Profile Management** - User dapat mengelola profil dan password sendiri

## Quick Start

### Prerequisites
- Go 1.21+
- Docker (optional)
- PostgreSQL (optional, default menggunakan SQLite)

### Installation

1. **Clone dan Setup**
```bash
cd GOLANG\ GOWA
cp .env.example .env
```

2. **Edit Configuration**
Edit file `.env` sesuai kebutuhan:
```env
# Server Configuration
SERVER_PORT=8080
SERVER_DEBUG=true

# Database (SQLite default)
DB_TYPE=sqlite
DB_PATH=./data/gowa.db

# WhatsApp Configuration
WA_AUTO_REPLY_MESSAGE=Terima kasih atas pesan Anda!
WA_WEBHOOK_URL=http://localhost:8080/webhook
```

3. **Install Dependencies**
```bash
go mod download
```

4. **Run Application**
```bash
go run main.go
```

### Docker Deployment

#### Option 1: Docker Command Line

1. **Build Image**
```bash
docker build -t gowa-broadcast .
```

2. **Run Container**
```bash
docker run -d \
  --name gowa-broadcast \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/.env:/app/.env \
  gowa-broadcast
```

#### Option 2: Docker Compose

```bash
docker-compose up -d
```

#### Option 3: Docker Portainer

**Langkah 1: Setup GitHub Repository**
1. **Push project ke GitHub:**
   ```bash
   # Inisialisasi git repository (jika belum)
   git init
   git add .
   git commit -m "Initial commit: GOWA Broadcast application"
   
   # Tambahkan remote repository
   git remote add origin https://github.com/username/gowa-broadcast.git
   git branch -M main
   git push -u origin main
   ```

2. **Clone ke server VPS:**
   ```bash
   # SSH ke server VPS
   ssh user@your-server-ip
   
   # Clone repository
   git clone https://github.com/username/gowa-broadcast.git
   cd gowa-broadcast
   
   # Copy dan edit file environment
   cp .env.example .env
   nano .env  # atau vim .env
   ```

**Langkah 2: Persiapan File di Server**
1. Pastikan file `.env` sudah dikonfigurasi dengan benar
2. Buat folder `data` untuk menyimpan database SQLite:
   ```bash
   mkdir -p data
   chmod 755 data
   ```
3. Pastikan Docker dan Portainer sudah terinstall di server

**Langkah 3: Deploy via Portainer**
1. Login ke Portainer dashboard
2. Pilih environment/endpoint yang akan digunakan
3. Masuk ke menu **Stacks** → **Add stack**
4. Berikan nama stack: `gowa-broadcast`
5. Pilih **Upload** dan upload file `docker-compose.yml`, atau copy-paste konfigurasi berikut:

```yaml
version: '3.8'
services:
  gowa-broadcast:
    build: .
    container_name: gowa-broadcast
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./.env:/app/.env
    restart: unless-stopped
    environment:
      - TZ=Asia/Jakarta
```

**Langkah 4: Konfigurasi Environment Variables (Optional)**
Jika ingin mengatur environment variables langsung di Portainer:
1. Scroll ke bagian **Environment variables**
2. Tambahkan variabel berikut:
   - `SERVER_PORT=8080`
   - `DB_TYPE=sqlite`
   - `DB_PATH=/app/data/gowa.db`
   - `AUTH_USERNAME=admin`
   - `AUTH_PASSWORD=admin123`
   - `JWT_SECRET=your-secret-key-here`

**Langkah 5: Deploy**
1. Klik **Deploy the stack**
2. Tunggu proses build dan deployment selesai
3. Cek status container di menu **Containers**

**Langkah 6: Akses Aplikasi**
1. Buka browser dan akses: `http://your-server-ip:8080`
2. Login dengan kredensial default:
   - Username: `admin`
   - Password: `admin123`
3. Scan QR code untuk menghubungkan WhatsApp

**Langkah 7: Monitoring**
- Monitor logs container melalui Portainer
- Cek resource usage (CPU, Memory) di dashboard
- Setup auto-restart policy jika diperlukan

**Tips untuk Portainer Deployment:**

1. **Volume Mapping**: Pastikan folder `data` sudah ada di host sebelum deploy
2. **Environment Variables**: Gunakan file `.env` atau set langsung di Portainer
3. **Network**: Jika menggunakan reverse proxy, pastikan container dalam network yang sama
4. **Resource Limits**: Set memory limit sesuai kebutuhan (minimal 512MB)
5. **Backup**: Backup folder `data` secara berkala untuk menjaga database

**Troubleshooting:**

- **Container tidak start**: Cek logs untuk error message
- **Database error**: Pastikan folder `data` memiliki permission yang benar
- **WhatsApp tidak connect**: Pastikan port 8080 accessible dan scan QR code
- **JWT error**: Pastikan `JWT_SECRET` sudah di-set dengan benar
- **Memory issues**: Monitor usage dan adjust container limits jika perlu

#### Option 4: GitHub Actions + Portainer (Advanced)

Untuk deployment otomatis menggunakan CI/CD:

**1. Setup GitHub Actions Workflow**
Buat file `.github/workflows/deploy.yml`:

```yaml
name: Deploy to Portainer

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  deploy:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Deploy to Portainer
      uses: carlrygart/portainer-stack-deploy@v1
      with:
        portainer-host: ${{ secrets.PORTAINER_HOST }}
        username: ${{ secrets.PORTAINER_USERNAME }}
        password: ${{ secrets.PORTAINER_PASSWORD }}
        stack-name: 'gowa-broadcast'
        stack-definition: 'docker-compose.yml'
        template-variables: |
          JWT_SECRET=${{ secrets.JWT_SECRET }}
          SERVER_PORT=8080
```

**2. Setup GitHub Secrets**
Di repository GitHub, tambahkan secrets:
- `PORTAINER_HOST`: URL Portainer (https://portainer.yourdomain.com)
- `PORTAINER_USERNAME`: Username Portainer
- `PORTAINER_PASSWORD`: Password Portainer
- `JWT_SECRET`: Secret key untuk JWT

**3. Auto Deploy**
Setiap push ke branch `main` akan otomatis deploy ke Portainer.

## API Documentation

### Authentication

Aplikasi menggunakan **JWT (JSON Web Token)** untuk autentikasi. Berikut cara menggunakan API:

#### 1. Login untuk mendapatkan JWT Token
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2024-01-01T12:00:00Z",
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@gowa.local",
    "full_name": "System Administrator",
    "role": "admin",
    "active": true
  }
}
```

#### 2. Menggunakan Token untuk API Calls
Sertakan token di header `Authorization` dengan format `Bearer <token>`:

```bash
curl -X GET http://localhost:8080/api/whatsapp/status \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

#### 3. Authentication Endpoints
```http
POST   /api/auth/login              # Login user
GET    /api/auth/profile            # Get user profile
PUT    /api/auth/profile            # Update user profile
POST   /api/auth/change-password    # Change password
POST   /api/auth/validate-token     # Validate JWT token

# Admin only endpoints
POST   /api/auth/users              # Create new user
GET    /api/auth/users              # Get all users
GET    /api/auth/users/:id          # Get user by ID
PUT    /api/auth/users/:id          # Update user
DELETE /api/auth/users/:id          # Delete user
POST   /api/auth/users/:id/change-password  # Change user password
```

### Core Endpoints

#### WhatsApp Management
```http
GET    /api/whatsapp/qr          # Get QR code untuk login
GET    /api/whatsapp/status      # Status koneksi WhatsApp
POST   /api/whatsapp/logout      # Logout dari WhatsApp
GET    /api/whatsapp/contacts    # Daftar kontak
GET    /api/whatsapp/groups      # Daftar grup
```

#### Message Operations
```http
POST   /api/send/text           # Kirim pesan teks
POST   /api/send/image          # Kirim gambar
POST   /api/send/document       # Kirim dokumen
POST   /api/send/audio          # Kirim audio
POST   /api/send/video          # Kirim video
POST   /api/send/location       # Kirim lokasi
POST   /api/send/contact        # Kirim kontak
```

#### Broadcast Management
```http
POST   /api/broadcast-lists     # Buat broadcast list
GET    /api/broadcast-lists     # Daftar broadcast lists
PUT    /api/broadcast-lists/:id # Update broadcast list
DELETE /api/broadcast-lists/:id # Hapus broadcast list

POST   /api/broadcasts          # Buat broadcast
GET    /api/broadcasts/:id      # Status broadcast
DELETE /api/broadcasts/:id      # Cancel broadcast
GET    /api/broadcasts          # Riwayat broadcasts
```

#### Scheduled Messages
```http
POST   /api/scheduled           # Buat pesan terjadwal
GET    /api/scheduled           # Daftar pesan terjadwal
DELETE /api/scheduled/:id       # Hapus pesan terjadwal
```

#### Statistics
```http
GET    /api/stats/dashboard     # Dashboard statistics
GET    /api/stats/messages      # Message statistics
GET    /api/stats/broadcasts    # Broadcast statistics
```

#### Webhooks
```http
POST   /api/webhooks            # Buat webhook
GET    /api/webhooks            # Daftar webhooks
PUT    /api/webhooks/:id        # Update webhook
DELETE /api/webhooks/:id        # Hapus webhook
GET    /api/webhooks/:id/logs   # Log webhook
```

### Example Usage

#### Send Text Message
```bash
# Dapatkan token terlebih dahulu
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# Kirim pesan menggunakan token
curl -X POST http://localhost:8080/api/send/text \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "6281234567890@s.whatsapp.net",
    "message": "Hello from GOWA Broadcast!"
  }'
```

#### Create Broadcast
```bash
curl -X POST http://localhost:8080/api/broadcasts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "broadcast_list_id": 1,
    "message": {
      "type": "text",
      "content": "Broadcast message to all recipients"
    }
  }'
```

#### Create New User (Admin Only)
```bash
curl -X POST http://localhost:8080/api/auth/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user1",
    "email": "user1@example.com",
    "password": "password123",
    "full_name": "User One",
    "role": "user"
  }'
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | Port server HTTP |
| `SERVER_DEBUG` | `false` | Mode debug |
| `DB_TYPE` | `sqlite` | Tipe database (sqlite/postgres) |
| `DB_PATH` | `./data/gowa.db` | Path database SQLite |
| `JWT_SECRET` | `your-secret-key` | Secret key untuk JWT token |
| `AUTH_USERNAME` | `admin` | Username basic auth (legacy) |
| `AUTH_PASSWORD` | `admin123` | Password basic auth (legacy) |
| `BROADCAST_RATE_LIMIT` | `10` | Rate limit broadcast (msg/min) |
| `BROADCAST_DELAY_MS` | `1000` | Delay antar pesan (ms) |
| `BROADCAST_MAX_RECIPIENTS` | `100` | Max penerima per broadcast |

### Database Configuration

#### SQLite (Default)
```env
DB_TYPE=sqlite
DB_PATH=./data/gowa.db
```

#### PostgreSQL
```env
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
DB_NAME=gowa_broadcast
DB_USER=postgres
DB_PASSWORD=password
DB_SSLMODE=disable
```

## Memory Optimization

Aplikasi ini dioptimasi untuk penggunaan memori yang efisien:

1. **Database Connection Pooling** - Menggunakan connection pool yang terbatas
2. **Message Batching** - Memproses pesan dalam batch untuk mengurangi memory footprint
3. **Rate Limiting** - Mencegah overload memory dengan membatasi concurrent operations
4. **Garbage Collection** - Optimasi GC untuk mengurangi memory pressure
5. **Streaming Processing** - Memproses data besar secara streaming

## Deployment di VPS

### Minimum Requirements
- **RAM**: 512MB (recommended 1GB)
- **CPU**: 1 vCPU
- **Storage**: 2GB
- **OS**: Linux (Ubuntu 20.04+ recommended)

### Docker Compose Example
```yaml
version: '3.8'
services:
  gowa-broadcast:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./.env:/app/.env
    restart: unless-stopped
    environment:
      - SERVER_PORT=8080
      - DB_TYPE=sqlite
      - DB_PATH=/app/data/gowa.db
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M
```

### Nginx Reverse Proxy
```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Monitoring

### Health Check
```bash
curl http://localhost:8080/api/whatsapp/status
```

### Logs
Aplikasi menggunakan structured logging. Log dapat dilihat dengan:
```bash
docker logs gowa-broadcast
```

### Metrics
Statistik dapat diakses melalui API:
```bash
curl -u admin:password http://localhost:8080/api/stats/dashboard
```

## Troubleshooting

### Common Issues

1. **QR Code tidak muncul**
   - Pastikan WhatsApp Web tidak sedang login di device lain
   - Restart aplikasi dan coba lagi

2. **Memory usage tinggi**
   - Kurangi `BROADCAST_RATE_LIMIT`
   - Tingkatkan `BROADCAST_DELAY_MS`
   - Gunakan SQLite untuk database yang lebih ringan

3. **Database connection error**
   - Periksa konfigurasi database di `.env`
   - Pastikan database server berjalan (untuk PostgreSQL)

4. **Webhook tidak bekerja**
   - Periksa URL webhook dapat diakses
   - Cek log webhook di `/api/webhooks/:id/logs`

### Debug Mode
Aktifkan debug mode untuk logging yang lebih detail:
```env
SERVER_DEBUG=true
```

## Contributing

1. Fork repository
2. Buat feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License

MIT License - lihat file [LICENSE](LICENSE) untuk detail.

## Support

Untuk pertanyaan dan dukungan:
- Create issue di GitHub repository
- Email: support@example.com

---

**GOWA Broadcast** - Efficient WhatsApp Broadcasting Solution