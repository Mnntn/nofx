# 🏗️ NOFX Architecture Documentation

**Language:** [English](README.md) | [中文](README.zh-CN.md)

Technical documentation for developers who want to understand NOFX internals.

---

## 📋 Overview

NOFX is a full-stack AI trading platform with:
- **Backend:** Go (Gin framework, SQLite)
- **Frontend:** React/TypeScript (Vite, TailwindCSS)
- **Architecture:** Microservice-inspired modular design

---

## 📁 Project Structure

```
nofx/
├── main.go                          # Program entry (multi-trader manager)
├── config.json                      # ~~Multi-trader config~~ (Now via web interface)
├── trading.db                       # SQLite database (traders, models, exchanges)
│
├── api/                            # HTTP API service
│   └── server.go                   # Gin framework, RESTful API
│
├── trader/                         # Trading core
│   ├── auto_trader.go              # Auto trading main controller
│   ├── interface.go                # Unified trader interface
│   ├── binance_futures.go          # Binance API wrapper
│   ├── hyperliquid_trader.go       # Hyperliquid DEX wrapper
│   └── aster_trader.go             # Aster DEX wrapper
│
├── manager/                        # Multi-trader management
│   └── trader_manager.go           # Manages multiple trader instances
│
├── config/                         # Configuration & database
│   └── database.go                 # SQLite operations and schema
│
├── auth/                           # Authentication
│   └── jwt.go                      # JWT token management & 2FA
│
├── mcp/                            # Model Context Protocol - AI communication
│   └── client.go                   # AI API client (DeepSeek/Qwen/Custom)
│
├── decision/                       # AI decision engine
│   ├── engine.go                   # Decision logic with historical feedback
│   └── prompt_manager.go           # Prompt template system
│
├── market/                         # Market data fetching
│   └── data.go                     # Market data & technical indicators (TA-Lib)
│   └── api_client.go               # Market data acquisition API
│   └── websocket_client.go         # Market data acquisition WebSocket interface
│   └── combined_streams.go         # Market data acquisition: Combined streaming (single link to subscribe to multiple cryptocurrencies)
│   └── monitor.go                  # Market data cache
│   └── types.go                    # market structure

├── pool/                           # Coin pool management
│   └── coin_pool.go                # AI500 + OI Top merged pool
│
├── logger/                         # Logging system
│   └── decision_logger.go          # Decision recording + performance analysis
│
├── decision_logs/                  # Decision log storage (JSON files)
│   ├── {trader_id}/                # Per-trader logs
│   └── {timestamp}.json            # Individual decisions
│
└── web/                            # React frontend
    ├── src/
    │   ├── components/             # React components
    │   │   ├── EquityChart.tsx     # Equity curve chart
    │   │   ├── ComparisonChart.tsx # Multi-AI comparison chart
    │   │   └── CompetitionPage.tsx # Competition leaderboard
    │   ├── lib/api.ts              # API call wrapper
    │   ├── types/index.ts          # TypeScript types
    │   ├── stores/                 # Zustand state management
    │   ├── index.css               # Binance-style CSS
    │   └── App.tsx                 # Main app
    ├── package.json                # Frontend dependencies
    └── vite.config.ts              # Vite configuration
```

---

## 🔧 Core Dependencies

### Backend (Go)

| Package | Purpose | Version |
|---------|---------|---------|
| `github.com/gin-gonic/gin` | HTTP API framework | v1.9+ |
| `github.com/adshao/go-binance/v2` | Binance API client | v2.4+ |
| `github.com/markcheno/go-talib` | Technical indicators (TA-Lib) | Latest |
| `github.com/mattn/go-sqlite3` | SQLite database driver | v1.14+ |
| `github.com/golang-jwt/jwt/v5` | JWT authentication | v5.0+ |
| `github.com/pquerna/otp` | 2FA/TOTP support | v1.4+ |
| `golang.org/x/crypto` | Password hashing (bcrypt) | Latest |

### Frontend (React + TypeScript)

| Package | Purpose | Version |
|---------|---------|---------|
| `react` + `react-dom` | UI framework | 18.3+ |
| `typescript` | Type safety | 5.8+ |
| `vite` | Build tool | 6.0+ |
| `recharts` | Charts (equity, comparison) | 2.15+ |
| `swr` | Data fetching & caching | 2.2+ |
| `zustand` | State management | 5.0+ |
| `tailwindcss` | CSS framework | 3.4+ |
| `lucide-react` | Icon library | Latest |

---

## 🗂️ System Architecture

### High-Level Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                      PRESENTATION LAYER                          │
│    React SPA (Vite + TypeScript + TailwindCSS)                  │
│    - Competition dashboard, trader management UI                 │
│    - Real-time charts (Recharts), authentication pages           │
└──────────────────────────────────────────────────────────────────┘
                             ↓ HTTP/JSON API
┌──────────────────────────────────────────────────────────────────┐
│                      API LAYER (Gin Router)                      │
│    /api/traders, /api/status, /api/positions, /api/decisions    │
│    Authentication middleware (JWT), CORS handling                │
└──────────────────────────────────────────────────────────────────┘
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│                      BUSINESS LOGIC LAYER                        │
│  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────┐ │
│  │ TraderManager    │  │ DecisionEngine   │  │ MarketData     │ │
│  │ - Multi-trader   │  │ - AI reasoning   │  │ - K-lines      │ │
│  │   orchestration  │  │ - Risk control   │  │ - Indicators   │ │
│  └──────────────────┘  └──────────────────┘  └────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│                      DATA ACCESS LAYER                           │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐     │
│  │ SQLite DB    │  │ File Logger  │  │ External APIs      │     │
│  │ - Traders    │  │ - Decisions  │  │ - Binance          │     │
│  │ - Models     │  │ - Performance│  │ - Hyperliquid      │     │
│  │ - Exchanges  │  │   analysis   │  │ - Aster            │     │
│  └──────────────┘  └──────────────┘  └────────────────────┘     │
└──────────────────────────────────────────────────────────────────┘
```

### Component Diagram

*(Coming soon: detailed component interaction diagram)*

---

## 📚 Core Modules

### 1. Trader System (`trader/`)

**Purpose:** Trading execution layer with multi-exchange support

**Key Files:**
- `auto_trader.go` - Main trading orchestrator (100+ lines)
- `interface.go` - Unified trader interface
- `binance_futures.go` - Binance API wrapper
- `hyperliquid_trader.go` - Hyperliquid DEX wrapper
- `aster_trader.go` - Aster DEX wrapper

**Design Pattern:** Strategy pattern with interface-based abstraction

**Example:**
```go
type ExchangeClient interface {
    GetAccount() (*AccountInfo, error)
    GetPositions() ([]*Position, error)
    CreateOrder(*OrderParams) (*Order, error)
    // ... more methods
}
```

---

### 2. Decision Engine (`decision/`)

**Purpose:** AI-powered trading decision making

**Key Files:**
- `engine.go` - Decision logic with historical feedback
- `prompt_manager.go` - Template system for AI prompts

**Features:**
- Chain-of-Thought reasoning
- Historical performance analysis
- Risk-aware decision making
- Multi-model support (DeepSeek, Qwen, custom)

**Flow:**
```
Historical Data → Prompt Generation → AI API Call →
Decision Parsing → Risk Validation → Execution
```

---

### 3. Market Data System (`market/`)

**Purpose:** Fetch and analyze market data

**Key Files:**
- `data.go` - Market data fetching and technical indicators

**Features:**
- Multi-timeframe K-line data (3min, 4hour)
  - Technical indicators via TA-Lib:
    - EMA (20, 50)
    - MACD
    - RSI (7, 14)
    - ATR (volatility)
  - Open Interest tracking

---

### 4. Manager (`manager/`)

**Purpose:** Multi-trader orchestration

**Key Files:**
- `trader_manager.go` - Manages multiple trader instances

**Responsibilities:**
- Trader lifecycle (start, stop, restart)
- Resource allocation
- Concurrent execution coordination

---

### 5. API Server (`api/`)

**Purpose:** HTTP API for frontend communication

**Key Files:**
- `server.go` - Gin framework RESTful API

**Endpoints:**
```
GET  /api/traders           # List all traders
POST /api/traders           # Create trader
POST /api/traders/:id/start # Start trader
GET  /api/status            # System status
GET  /api/positions         # Current positions
GET  /api/decisions/latest  # Recent decisions
```

---

### 6. Database Layer (`config/`)

**Purpose:** SQLite data persistence

**Key Files:**
- `database.go` - Database operations and schema

**Tables:**
- `users` - User accounts (with 2FA support)
- `ai_models` - AI model configurations
- `exchanges` - Exchange credentials
- `traders` - Trader instances
- `equity_history` - Performance tracking
- `system_config` - Application settings

---

### 7. Authentication (`auth/`)

**Purpose:** User authentication and authorization

**Features:**
- JWT token-based auth
- 2FA with TOTP (Google Authenticator)
- Bcrypt password hashing
- Admin mode (simplified single-user)

---

## 🔄 Request Flow Examples

### Example 1: Create New Trader

```
User Action (Frontend)
    ↓
POST /api/traders
    ↓
API Server (auth middleware)
    ↓
Database.CreateTrader()
    ↓
TraderManager.StartTrader()
    ↓
AutoTrader.Run() → goroutine
    ↓
Response: {trader_id, status}
```

### Example 2: Trading Decision Cycle

```
AutoTrader (every 3-5 min)
    ↓
1. FetchAccountStatus()
    ↓
2. GetOpenPositions()
    ↓
3. FetchMarketData() → TA-Lib indicators
    ↓
4. AnalyzeHistory() → last 20 trades
    ↓
5. GeneratePrompt() → full context
    ↓
6. CallAI() → DeepSeek/Qwen
    ↓
7. ParseDecision() → structured output
    ↓
8. ValidateRisk() → position limits, margin
    ↓
9. ExecuteOrders() → exchange API
    ↓
10. LogDecision() → JSON file + database
```

---

## 📊 Data Flow

### Market Data Flow

```
Exchange API
    ↓
market.FetchKlines()
    ↓
TA-Lib.Calculate(EMA, MACD, RSI)
    ↓
DecisionEngine (as context)
    ↓
AI Model (reasoning)
```

### Decision Logging Flow

```
AI Response
    ↓
decision_logger.go
    ↓
JSON file: decision_logs/{trader_id}/{timestamp}.json
    ↓
Database: performance tracking
    ↓
Frontend: /api/decisions/latest
```

---

## 🗄️ Database Schema

### Core Tables

**users**
```sql
- id (INTEGER PRIMARY KEY)
- username (TEXT UNIQUE)
- password_hash (TEXT)
- totp_secret (TEXT)
- is_admin (BOOLEAN)
- created_at (DATETIME)
```

**ai_models**
```sql
- id (INTEGER PRIMARY KEY)
- name (TEXT)
- model_type (TEXT) -- deepseek, qwen, custom
- api_key (TEXT)
- api_url (TEXT)
- enabled (BOOLEAN)
```

**traders**
```sql
- id (TEXT PRIMARY KEY)
- name (TEXT)
- ai_model_id (INTEGER FK)
- exchange_id (INTEGER FK)
- initial_balance (REAL)
- current_equity (REAL)
- status (TEXT) -- running, stopped
- created_at (DATETIME)
```

*(More details: database-schema.md - coming soon)*

---

## 🔌 API Reference

### Authentication

**POST /api/auth/login**
```json
Request: {
  "username": "string",
  "password": "string",
  "totp_code": "string" // optional
}

Response: {
  "token": "jwt_token",
  "user": {...}
}
```

### Trader Management

**GET /api/traders**
```json
Response: {
  "traders": [
    {
      "id": "string",
      "name": "string",
      "status": "running|stopped",
      "balance": 1000.0,
      "roi": 5.2
    }
  ]
}
```

*(Full API reference: api-reference.md - coming soon)*

---

## 🧪 Testing Architecture

### Current State
- ⚠️ No unit tests yet
- ⚠️ Manual testing only
- ⚠️ Testnet verification

### Planned Testing Strategy

**Unit Tests (Priority 1)**
```
trader/binance_futures_test.go
- Mock API responses
- Test precision handling
- Validate order construction
```

**Integration Tests (Priority 2)**
```
- End-to-end trading flow (testnet)
- Multi-trader scenarios
- Database operations
```

**Frontend Tests (Priority 3)**
```
- Component tests (Vitest + React Testing Library)
- API integration tests
- E2E tests (Playwright)
```

*(Testing guide: testing-guide.md - coming soon)*

---

## 🔧 Development Tools

### Build & Run

```bash
# Backend
go build -o nofx
./nofx

# Frontend
cd web
npm run dev

# Docker
docker compose up --build
```

### Code Quality

```bash
# Format Go code
go fmt ./...

# Lint (if configured)
golangci-lint run

# Type check TypeScript
cd web && npm run build
```

---

## 📈 Performance Considerations

### Backend
- **Concurrency:** Each trader runs in separate goroutine
- **Database:** SQLite (good for <100 traders)
- **API Rate Limits:** Handled per exchange
- **Memory:** ~50-100MB per trader

### Frontend
- **Data Fetching:** SWR with 5-10s polling
- **State:** Zustand (lightweight)
- **Bundle Size:** ~500KB (gzipped)

---

## 🔮 Future Architecture Plans

### Planned Improvements

1. **Microservices Split** (if scaling needed)
   - Separate decision engine service
   - Market data service
   - Execution service

2. **Database Migration**
   - Mysql for production (>100 traders)
   - Redis for caching

3. **Event-Driven Architecture**
   - WebSocket for real-time updates
   - Message queue (RabbitMQ/NATS)

4. **Kubernetes Deployment**
   - Helm charts
   - Auto-scaling
   - High availability

---

## 🆘 For Developers

**Want to contribute?**
- Read [Contributing Guide](../../CONTRIBUTING.md)
- Check [Open Issues](https://github.com/tinkle-community/nofx/issues)
- Join [Telegram Community](https://t.me/nofx_dev_community)

**Need clarification?**
- Open a [GitHub Discussion](https://github.com/tinkle-community/nofx/discussions)
- Ask in Telegram

---

## 📚 Related Documentation

- [Getting Started](../getting-started/README.md) - Setup and deployment
- [Contributing](../../CONTRIBUTING.md) - How to contribute
- [Community](../community/README.md) - Bounties and recognition

---

[← Back to Documentation Home](../README.md)
