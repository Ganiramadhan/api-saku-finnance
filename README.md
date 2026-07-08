# SAKU Finance API

AI-powered personal finance API platform built with Go, designed for modern financial tracking, intelligent transaction management, and scalable SaaS architecture.

SAKU helps users manage personal finances through manual transactions, AI-powered financial insights, receipt scanning, subscription reminders, budgeting, split bills, and conversational finance management using NLP and AI assistants.

---

## Overview

SAKU is more than a simple expense tracker.

It combines:
- Personal finance management
- AI-powered transaction processing
- Conversational finance assistant
- OCR receipt scanning
- Subscription & billing reminders
- Goal-based budgeting
- Smart financial insights

into a single scalable backend platform.

Built with:
- Go (Golang)
- Fiber
- PostgreSQL
- Redis
- RabbitMQ
- MinIO
- Claude AI (Anthropic)
- Docker

---

# Core Features

## Smart Transaction Management

Track:
- Income
- Expenses
- Transfers
- Recurring payments

Transactions can be created through:
- Manual input
- AI/NLP chat commands
- OCR receipt scanning

Examples:
- "I spent 75k for coffee at Starbucks"
- "Add internet bill 350k for this month"

The AI automatically extracts:
- Amount
- Merchant
- Category
- Transaction type
- Date

Before saving, every AI-generated transaction goes through a preview & confirmation flow to avoid incorrect data.

---

## AI Financial Assistant (Claude AI)

Powered by Anthropic Claude.

Users can:
- Ask financial questions
- Generate spending summaries
- Analyze monthly expenses
- Receive budgeting insights
- Detect unusual spending habits
- Get financial recommendations

Examples:
- "How much did I spend on food this month?"
- "Summarize my expenses this week"
- "Why is my spending higher this month?"
- "Give me saving suggestions"

---

## NLP Transaction Creation

Users can create transactions naturally through chat.

Examples:
- "Paid Netflix subscription 169k"
- "Bought groceries 250k at Alfamart"
- "Salary received 8 million"

The AI converts natural language into structured transactions automatically.

---

## OCR Receipt Scanner

Upload or scan receipts and the system will:
- Extract text using OCR
- Detect merchant name
- Detect total amount
- Categorize transaction
- Generate structured transaction preview

Supported flow:
1. Upload receipt image
2. OCR + AI processing
3. Preview extracted data
4. User confirmation
5. Save transaction

This reduces manual finance tracking friction significantly.

---

## Wallet Management

Supports multiple wallets per user:
- Cash
- Bank accounts
- E-wallets
- Digital wallets
- Savings accounts

Each wallet includes:
- Balance tracking
- Transaction history
- Currency support
- Analytics integration

---

## Budget & Saving Goals

Users can create:
- Monthly budgets
- Spending limits
- Saving goals

Examples:
- House down payment
- Emergency fund
- Vacation target
- Vehicle purchase

Features:
- Budget progress tracking
- Spending alerts
- Goal analytics
- Financial milestone monitoring

---

## Upcoming Billing & Subscription Reminder

Track recurring bills and subscriptions:
- Netflix
- Spotify
- VPS hosting
- Internet bills
- Electricity
- SaaS subscriptions

Features:
- Upcoming payment reminders
- Due date tracking
- Recurring billing management
- Notification scheduling

---

## Split Bill Management

Supports:
- Manual split bill creation
- Receipt-based split bill generation

Features:
- Equal split
- Custom split
- Participant tracking
- Shared expense calculation

Perfect for:
- Trips
- Group dining
- Team expenses
- Shared subscriptions

---

# Tech Stack

## Backend
- Go (Golang)
- Fiber
- GORM
- PostgreSQL

## Infrastructure
- Redis
- RabbitMQ
- MinIO
- Docker
- Docker Compose

## AI & OCR
- Claude AI (Anthropic)
- OCR Processing Pipeline

## Payment Gateway
- Midtrans

## Storage
- S3-compatible object storage via MinIO

---

# Architecture

```bash
Client Apps
   │
   ├── Web App
   ├── Mobile App
   └── AI Chat Interface
   │
   ▼
SAKU API Gateway
   │
   ├── Authentication
   ├── Wallet Service
   ├── Transaction Service
   ├── Budget Service
   ├── Billing Service
   ├── AI Service
   ├── OCR Service
   └── Split Bill Service
   │
   ▼
Infrastructure Layer
   ├── PostgreSQL
   ├── Redis
   ├── RabbitMQ
   ├── MinIO
   └── Claude AI
```

---

# Project Structure

```bash
api/
├── cmd/api
├── docker
├── docs
├── envs
├── internal
│   ├── app
│   ├── config
│   ├── constants
│   ├── domain
│   ├── dto
│   ├── middleware
│   ├── modules
│   │   ├── ai
│   │   ├── ailog
│   │   ├── auth
│   │   ├── budget
│   │   ├── category
│   │   ├── savinggoal
│   │   ├── splitbill
│   │   ├── subscription
│   │   ├── transaction
│   │   ├── user
│   │   └── wallet
│   ├── platform
│   │   ├── ai
│   │   ├── cache
│   │   ├── database
│   │   ├── messaging
│   │   ├── payment
│   │   └── storage
│   └── routes
└── pkg
```

---

# Infrastructure Components

## PostgreSQL
Primary relational database used for:
- Transactions
- Wallets
- Budgets
- Users
- Billing
- Split bills

## Redis
Used for:
- Caching
- Session storage
- Rate limiting
- Queue buffering

## RabbitMQ
Used for:
- AI processing queue
- OCR jobs
- Notification jobs
- Background processing

## MinIO
Used for:
- Receipt storage
- User uploads
- AI processing assets
- Object storage

Compatible with Amazon S3 APIs.

---

# API Documentation

Swagger documentation available at:

```bash
/api/swagger/index.html
```

---

# Security

Features:
- JWT authentication
- Role-based authorization
- Secure file uploads
- Request validation
- Rate limiting
- Environment-based configuration
- AI processing isolation
- Secure object storage

---

# Docker Support

Run full infrastructure locally using Docker Compose.

Services included:
- API
- PostgreSQL
- Redis
- RabbitMQ
- MinIO

---

# CI/CD Deployment

Production deployment uses `docker-compose.prd.yaml` and Jenkins. The pipeline builds the Go API image, pushes it to the registry, uploads the production compose file and secret `.env` to the server deploy directory, then runs `docker compose up -d` with a health check and rollback image tracking.

Expected Jenkins credentials:

- `saku-finance-api-env` as secret file containing the production API `.env`
- `saku-finance-api-deploy-path` as secret text containing the absolute server deploy directory
- `docker-registry-host` as secret text, without protocol
- `docker-registry-username` as secret text
- `docker-registry-credentials` as secret text for the registry password or access token
- `ganipedia-host-ssh-server` as secret text
- `ganipedia-host-ssh-port` as secret text
- `ganipedia-host-ssh-user` as secret text
- `ganipedia-host-ssh-password` as secret text

The server deploy directory is expected to contain:

```text
docker-compose.prd.yaml
.env
.previous_image
```

The API is attached to the `saku-finance` Docker network with alias `api-saku-finance` and listens on internal port `4001`.

---

# Environment Variables

```env
APP_ENV=development
APP_PORT=4000

DB_HOST=postgres
DB_PORT=5432
DB_NAME=saku
DB_USER=postgres
DB_PASSWORD=postgres

REDIS_HOST=redis
REDIS_PORT=6379

RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/

MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=saku

ANTHROPIC_API_KEY=your_claude_api_key

MIDTRANS_SERVER_KEY=your_midtrans_server_key
MIDTRANS_CLIENT_KEY=your_midtrans_client_key

SENTRY_DSN=
SENTRY_ENVIRONMENT=production
SENTRY_TRACES_SAMPLE_RATE=0.05

JWT_SECRET=super-secret-key
```

---

# Development Philosophy

SAKU is designed with:
- Clean Architecture principles
- Vertical Slice Architecture
- Modular domain separation
- Scalable async processing
- Production-ready infrastructure
- AI-first financial experience

---

# Future Roadmap

Planned features:
- Investment tracking
- AI anomaly detection
- Financial forecasting
- Bank synchronization
- Multi-currency support
- Team/shared wallets
- Push notifications
- Advanced analytics dashboard

---

# License

MIT License.
