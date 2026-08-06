# Arsitektur Sistem PASTI V3

## 1. Topologi Infrastruktur

\```mermaid
graph TB
    User["👤 Pengguna<br/>Browser"]
    SSO["🏛️ SSO Kemenkeu<br/>sso.kemenkeu.go.id<br/>OpenID Connect Provider"]

    subgraph VPS["🖥️ VPS Production — Ubuntu Server 22.04 LTS"]
        subgraph DockerNet["🐳 Docker Network: pasti-network"]
            Nginx["🔀 Nginx Reverse Proxy<br/>Container: pasti-nginx<br/>Port 80/443<br/>SSL Termination"]
            Certbot["🔐 Certbot<br/>Container: pasti-certbot<br/>Auto-renew SSL"]
            Backend["⚙️ Backend Go + Gin<br/>Container: pasti-backend<br/>Port 8686 (internal)<br/>JWT · OIDC · Salt+Pepper"]
            Frontend["🎨 Frontend Next.js<br/>Container: pasti-frontend<br/>Port 3000 (internal)<br/>React 19 · TypeScript"]
        end
    end

    subgraph DBServer["🗄️ Database Server (Terpisah)"]
        DB[("SQL Server 2022<br/>pasti_v3_db<br/><br/>Tables:<br/>users · employees<br/>refresh_tokens · sso_states")]
    end

    User -- "HTTPS :443" --> Nginx
    Nginx -- "/api/*, /sso/*, /health" --> Backend
    Nginx -- "/ (semua route lain)" --> Frontend
    Backend -- "TCP :1433<br/>(firewall whitelist IP VPS)" --> DB
    Backend -- "OIDC Authorization Code + PKCE" --> SSO
    Certbot -. "renew cert tiap 12 jam" .-> Nginx

    style User fill:#e0f2fe,stroke:#0284c7
    style SSO fill:#fef3c7,stroke:#d97706
    style Nginx fill:#dcfce7,stroke:#16a34a
    style Backend fill:#dbeafe,stroke:#2563eb
    style Frontend fill:#f3e8ff,stroke:#9333ea
    style DB fill:#fee2e2,stroke:#dc2626
    style Certbot fill:#f1f5f9,stroke:#64748b
\```

## 2. Alur Login Manual

\```mermaid
sequenceDiagram
    participant B as Browser
    participant N as Nginx
    participant F as Frontend (Next.js)
    participant BE as Backend (Go)
    participant DB as SQL Server 2022

    B->>N: GET /login
    N->>F: proxy_pass
    F-->>B: Render halaman login

    B->>N: POST /api/v1/auth/login<br/>{username, password}
    N->>BE: proxy_pass

    BE->>DB: SELECT user WHERE username=?
    DB-->>BE: user data (hash, salt)

    Note over BE: Verify:<br/>SHA256(password+salt+pepper)<br/>→ bcrypt.Compare()

    alt Password Valid
        BE->>DB: UPDATE last_login, reset failed_attempts
        BE-->>N: JWT Access Token
        N-->>B: 200 OK + token
        B->>B: Simpan token di cookie
        B->>N: Redirect ke /dashboard
    else Password Invalid
        BE->>DB: UPDATE failed_login_attempts++
        BE-->>N: 401 Unauthorized
        N-->>B: Error message
    end
\```

## 3. Alur Login SSO Kemenkeu (OIDC + PKCE)

\```mermaid
sequenceDiagram
    participant B as Browser
    participant F as Frontend
    participant BE as Backend (Go)
    participant DB as SQL Server
    participant SSO as SSO Kemenkeu

    B->>F: Klik "Masuk dengan SSO Kemenkeu"
    F->>BE: GET /sso/login

    Note over BE: Generate state +<br/>PKCE code_verifier/challenge
    BE->>DB: INSERT INTO sso_states
    BE-->>B: 302 Redirect ke authorize_endpoint

    B->>SSO: GET /connect/authorize?client_id=pasti&...
    SSO-->>B: Halaman login SSO Kemenkeu
    B->>SSO: User login (username/password Kemenkeu)
    SSO-->>B: 302 Redirect ke redirect_uri?code=...&state=...

    B->>BE: GET /sso/callback/login?code=...&state=...
    BE->>DB: Validasi state, ambil code_verifier
    DB-->>BE: code_verifier valid

    BE->>SSO: POST /connect/token<br/>(code, code_verifier, client_secret)
    SSO-->>BE: access_token

    BE->>SSO: GET /connect/userinfo<br/>(Bearer access_token)
    SSO-->>BE: Claims: nip, email, jabatan, satker, dll

    Note over BE: Upsert data pegawai
    BE->>DB: INSERT/UPDATE employees
    Note over BE: Cek is_protected<br/>(email/NIP superadmin permanen)
    BE->>DB: INSERT/UPDATE users

    BE-->>B: 302 Redirect ke Frontend<br/>#token=JWT&expires_in=...

    B->>F: GET /sso/callback (baca URL fragment)
    F->>F: Simpan token ke cookie
    F-->>B: Redirect ke /dashboard
\```

## 4. Skema Database (ERD)

\```mermaid
erDiagram
    USERS ||--o| EMPLOYEES : "employee_id (FK)"
    USERS ||--o{ REFRESH_TOKENS : "user_id (FK)"

    USERS {
        uuid id PK
        string username
        string email
        string password_hash "nullable, untuk user SSO"
        string password_salt "nullable, untuk user SSO"
        string full_name
        string role "user/admin/superadmin"
        bool is_active
        string auth_provider "local/sso"
        bool is_protected "superadmin permanen"
        uuid employee_id FK
        int failed_login_attempts
        datetime locked_until
        datetime last_login
        datetime created_at
        datetime updated_at
    }

    EMPLOYEES {
        uuid id PK
        string sso_sub "unique, dari claim SSO"
        string nip
        string nip9
        string nik
        string name
        string email
        string jabatan
        string satker
        string organisasi
        string kode_kl
        string raw_claims "snapshot JSON userinfo"
        datetime created_at
        datetime updated_at
    }

    REFRESH_TOKENS {
        uuid id PK
        uuid user_id FK
        string token_hash
        datetime expires_at
        bool revoked
        datetime created_at
    }

    SSO_STATES {
        string state PK
        string code_verifier "PKCE"
        datetime expires_at
        datetime created_at
    }
\```

## 5. Zona Keamanan (Security Boundary)

\```mermaid
graph TB
    subgraph Public["🌐 ZONA PUBLIK — Internet"]
        User["Browser Pengguna"]
    end

    subgraph EdgeZone["🛡️ ZONA EDGE — Exposed ke Publik"]
        Nginx["Nginx :80 / :443<br/>SSL Termination"]
    end

    subgraph InternalZone["🔒 ZONA INTERNAL — Docker Network<br/>(tidak exposed ke publik)"]
        Backend["Backend :8686<br/>(expose, bukan ports)"]
        Frontend["Frontend :3000<br/>(expose, bukan ports)"]
    end

    subgraph DBZone["🗝️ ZONA DATABASE — Server Terpisah<br/>(firewall whitelist IP VPS)"]
        DB[("SQL Server 2022 :1433")]
    end

    User -->|"HANYA port 443/80<br/>yang bisa diakses"| Nginx
    Nginx --> Backend
    Nginx --> Frontend
    Backend -->|"koneksi terenkripsi<br/>hanya dari IP VPS ini"| DB

    style Public fill:#fef2f2,stroke:#dc2626,stroke-width:2px
    style EdgeZone fill:#fefce8,stroke:#ca8a04,stroke-width:2px
    style InternalZone fill:#f0fdf4,stroke:#16a34a,stroke-width:2px
    style DBZone fill:#eff6ff,stroke:#2563eb,stroke-width:2px
\```