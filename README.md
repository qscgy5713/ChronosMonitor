# ChronosMonitor

跨語言、輕量級的排程任務監控面板。任何語言的 worker（Laravel cron、Go 背景服務、Node.js script...）只要透過簡單的 HTTP API 回報任務狀態，就能在一個即時儀表板上看到執行中任務、失敗任務、逾時警告與耗時統計。

不需要 agent、不需要在目標主機裝東西——單一 Go 執行檔內建前端，複製過去執行就能用。

## 系統架構

```mermaid
graph TD
    subgraph Client [Client Applications]
        L[Laravel Cron] -->|HTTP POST| API
        G[Go Worker] -->|HTTP POST| API
        N[Node.js Script] -->|HTTP POST| API
    end

    subgraph ChronosMonitor [Chronos Monitor Server]
        API[HTTP API Server] --> EB[Event Broker]
        EB --> DB[(SQLite / PostgreSQL)]
        EB --> WS[WebSocket / SSE Server]
    end

    subgraph Dashboard [Web UI]
        Vue[Vue 3 SPA] <-->|WebSocket| WS
        Vue <-->|HTTP GET| API
    end
```

> 目前實作用的是 SSE，不是圖上寫的 WebSocket（單向推播場景 SSE 更簡單，效果相同）。

任務不會被「偵測」到——ChronosMonitor 完全是被動接收回報的架構：worker/cron 自己主動呼叫 API 才會被記錄，沒有呼叫就不存在。詳見下方 [API](#api) 與 [demo-worker](#demo-worker本機模擬任務) 章節。

## 功能

- **HTTP 狀態回報 API**：`start` / `heartbeat` / `success` / `failed`
- **即時監控儀表板**：Vue 3 + SSE，狀態變化立即反映，不用重新整理
- **任務逾期警告（TTL）**：任務超過宣告的時間沒有心跳/完成，自動標記為 `timeout`
- **Missed run 偵測（dead man's switch）**：可以額外註冊「這個任務應該至少每 N 秒跑一次」，完全沒 `start` 也能被抓到並觸發告警——這是 TTL 機制的對稱功能，TTL 抓「跑了但卡住」，這個抓「該跑但根本沒跑」
- **錯誤堆疊追蹤**：失敗任務可夾帶 `error_message`（含 stack trace），直接在面板上看
- **告警通知**：任務失敗/逾時可透過 webhook 通知 Slack / Discord，內建頻率限制避免洗版
- **API 認證**：可選的 API key，保護回報端點不被亂打（尤其是接了告警之後，沒認證等於誰都能發假失敗事件洗你的 Slack）
- **資料保留/清理**：可選的背景清理工作，自動刪除超過保留期限的已結束任務，避免資料表無限長大；`running` 狀態不管多舊都不會被清
- **SQLite / PostgreSQL 雙資料庫支援**：預設零依賴 SQLite，設定環境變數即可切換 PostgreSQL
- **單一執行檔部署**：前端建置產物透過 `go:embed` 打包進二進位檔
- **Docker / docker-compose 部署**：`docker compose up` 一鍵跑起來，image 約 38MB（`scratch` + 純 Go 靜態編譯，無 CGO）

## 技術棧

| 層 | 技術 |
|---|---|
| 後端 | Go + [Gin](https://github.com/gin-gonic/gin) |
| 前端 | Vue 3（Composition API）+ Tailwind CSS + Vite |
| 資料庫 | SQLite（[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)，純 Go 無 CGO）/ PostgreSQL（[pgx](https://github.com/jackc/pgx)） |
| 即時通訊 | Server-Sent Events（SSE） |

## 專案結構

```
cmd/
  server/          進入點：組裝 config / db / store / broker / sweeper / router
  demo-worker/      範例 worker，示範如何用 HTTP 呼叫回報任務狀態
internal/
  config/          環境變數設定
  db/              資料庫連線，SQLite/Postgres 切換與 SQL placeholder 轉換
  models/          TaskRun / Schedule 資料模型
  store/           task_runs、task_schedules 資料存取層（含 TTL 掃描、missed-run 偵測邏輯）
  broker/          記憶體內 pub/sub，供 SSE 推播與告警訂閱用
  sweeper/         背景 goroutine，定期掃描逾期（TTL）任務
  missedrun/       背景 goroutine，定期檢查有沒有該跑但沒跑的排程
  notifier/        失敗/逾時/missed run 告警：webhook 訊息格式化 + 頻率限制 dispatcher
  retention/       背景 goroutine，定期刪除超過保留期限的已結束任務
  handlers/        Gin handler（HTTP API + SSE stream）
  router/          路由註冊、內嵌前端靜態檔案伺服
  webui/           go:embed 內嵌前端 build 產物
web/               Vue 3 前端（Vite 專案）
Makefile
```

## 快速開始

### 開發模式（雙 process，前端熱重載）

```bash
# 後端：http://localhost:8080
make dev-backend

# 前端：http://localhost:5173（會自動 proxy /api 到 8080）
make dev-frontend
```

開發時打開 `http://localhost:5173` 看畫面。

### 單一執行檔（模擬正式部署）

```bash
make build   # 建前端 -> embed -> go build，產生 ./chronosmonitor
./chronosmonitor
```

打開 `http://localhost:8080` 即可看到儀表板，API 與前端同一個 port。

### Docker

不裝 Go、Node、SQLite 工具鏈，一個指令跑起來（預設用 SQLite，資料存在 named volume）：

```bash
docker compose up -d
```

打開 `http://localhost:8080`。要用 PostgreSQL 而不是 SQLite，加 `--profile postgres` 順便帶起一個 PostgreSQL 容器：

```bash
CHRONOS_DB_DRIVER=postgres docker compose --profile postgres up -d
```

`docker-compose.yml` 沒有設 `depends_on` 卡健康檢查——靠 `restart: unless-stopped` 自動重試，所以**第一次啟動時 log 會看到幾次 `connection refused` 屬於正常現象**，等 PostgreSQL ready 後會自己接上。這是刻意的取捨：加 `depends_on` 健康檢查會讓 `postgres` service 即使沒開 `--profile postgres` 也被強制帶起來，違背它「選配」的設計。

其他環境變數（`CHRONOS_API_KEY`、`CHRONOS_ALERT_WEBHOOK_URL` 等，見下方環境變數表）都可以直接用 shell 環境變數傳進去，`docker-compose.yml` 都接好了：

```bash
CHRONOS_API_KEY=some-secret CHRONOS_ALERT_WEBHOOK_URL=https://hooks.slack.com/xxx docker compose up -d
```

只想 build image 不透過 compose：

```bash
docker build -t chronosmonitor .
docker run -d -p 8080:8080 -v chronos-data:/data chronosmonitor
```

Image 是 multi-stage build（Node 建前端 → Go 靜態編譯 → `scratch` runtime），因為 SQLite（modernc.org/sqlite）跟 PostgreSQL（pgx）driver 都是純 Go 實作、不需要 CGO，所以最終 image 只有一個執行檔 + CA 憑證，大小約 38MB。

### demo-worker：本機模擬任務

`cmd/demo-worker` 是一個假的 Go worker，純粹示範「怎麼從你自己的程式碼呼叫 ChronosMonitor 的 API」——不是產品的一部分，只是本機展示用。

```bash
go run ./cmd/demo-worker
```

行為：每 3 秒跑一個假任務（`nightly-backup` / `invoice-sync` / `email-digest` / `report-export` 隨機挑一個），呼叫 `POST /api/v1/events/start` 後依機率模擬三種結局：

| 機率 | 情境 | 對應行為 |
|---|---|---|
| 70% | 成功 | 送 2 次 `heartbeat`，再送 `success`，`duration_ms` 約 900ms |
| 20% | 失敗 | 400ms 後送 `failed`，帶一段假的 stack trace 當 `error_message` |
| 10% | 卡住不放 | 呼叫 `start` 之後完全不再回報任何事件，永遠停在 `running` |

另外，每次 `start` 有獨立的 10% 機率會帶上 `ttl_seconds: 3`——如果剛好跟「卡住不放」的情境同時中，那筆任務會在 3 秒後被 sweeper 標記成 `timeout`，可以在儀表板上看到逾期效果（畢竟是機率疊機率，多跑幾輪、或縮短輪詢間隔比較容易碰到）。

可用環境變數：

| 變數 | 預設值 | 說明 |
|---|---|---|
| `CHRONOS_BASE_URL` | `http://localhost:8080` | ChronosMonitor server 的位址 |

想連到別台機器上的 ChronosMonitor 測試：

```bash
CHRONOS_BASE_URL=http://your-host:8080 go run ./cmd/demo-worker
```

## 環境變數

| 變數 | 預設值 | 說明 |
|---|---|---|
| `CHRONOS_PORT` | `8080` | HTTP 監聽 port |
| `CHRONOS_DB_DRIVER` | `sqlite` | `sqlite` 或 `postgres` |
| `CHRONOS_DB_PATH` | `data/chronos.db` | SQLite 檔案路徑（`CHRONOS_DB_DRIVER=sqlite` 時使用） |
| `CHRONOS_DB_DSN` | (空) | PostgreSQL 連線字串，例：`postgres://user:pass@host:5432/db?sslmode=disable`（`CHRONOS_DB_DRIVER=postgres` 時必填） |
| `CHRONOS_TTL_SWEEP_INTERVAL_SECONDS` | `30` | TTL 逾期掃描的間隔秒數 |
| `CHRONOS_MISSED_RUN_CHECK_INTERVAL_SECONDS` | `60` | Missed-run 檢查的間隔秒數。沒有開關可以關閉——只有真的用 API 註冊過排程的任務才會被檢查，沒註冊的完全不受影響 |
| `CHRONOS_ALERT_WEBHOOK_URL` | (空) | 設定後啟用告警；任務 `failed` / `timeout` / missed schedule 時會 POST 到這個 URL。不設定就完全不啟用（預設） |
| `CHRONOS_ALERT_WEBHOOK_FORMAT` | `slack` | `slack`（`{"text":...}`，Mattermost 等 Slack-compatible 服務也吃這個格式）/ `discord`（`{"content":...}`）/ `generic`（原始任務 JSON，接自己的系統用） |
| `CHRONOS_ALERT_RATE_LIMIT_PER_MINUTE` | `10` | 每分鐘最多送出幾則告警，超過的直接丟棄並記 log，避免大量任務同時失敗時洗版。設 `0` 表示不限制 |
| `CHRONOS_API_KEY` | (空) | 設定後，整個 `/api/v1/*`（含回報跟查詢）都需要帶這把 key 才能存取。不設定就完全不需要認證（預設，向下相容） |
| `CHRONOS_RETENTION_DAYS` | `0` | 已結束任務的保留天數，超過就會被自動刪除。`0` 表示永久保留（預設） |
| `CHRONOS_RETENTION_SWEEP_INTERVAL_SECONDS` | `3600` | 清理工作的檢查間隔秒數（預設 1 小時） |

## Missed Run 偵測（Dead Man's Switch）

TTL 逾期偵測只能抓「任務已經 `start` 但卡住不放」；如果一個 cron 該跑的時候完全沒跑（程式沒被觸發、被排程系統漏掉、主機掛了...），TTL 機制完全不會知道，因為根本沒有一筆 run 存在。Missed run 偵測補上這一塊：額外註冊一個「這個 `task_name` 應該多久跑一次」的期望，背景服務會定期檢查有沒有超過期限沒收到 `start`。

支援兩種模式，註冊時**擇一**：

**簡單間隔模式**——適合「大概每 N 秒跑一次」這種粗略排程：

```bash
curl -X POST localhost:8080/api/v1/schedules \
  -H 'Content-Type: application/json' \
  -d '{
    "task_name": "daily-report",
    "expected_interval_seconds": 86400,
    "grace_period_seconds": 1800
  }'
```

**Cron expression 模式**——適合非均勻排程（只在平日跑、每月固定幾號跑...），標準 5 欄位語法（`分 時 日 月 星期`）：

```bash
curl -X POST localhost:8080/api/v1/schedules \
  -H 'Content-Type: application/json' \
  -d '{
    "task_name": "weekday-report",
    "cron_expression": "0 9 * * 1-5",
    "grace_period_seconds": 600
  }'
```

兩者的差別不只是語法——間隔模式沒辦法正確表達「只在平日跑」：如果用「每 24 小時」去逼近「平日 9 點跑」，系統會把週五跑完到週一該跑之間那 ~72 小時的正常空檔誤判成 missed。Cron 模式會正確算出「下一次該執行的時間」，不會有這個問題。沒有特別需求的話，簡單間隔模式已經夠用；只有排程本身就不均勻時才需要 cron 模式。

`grace_period_seconds` 兩種模式都適用，是「超過預期時間之後，再給多久緩衝才真的算 missed」。

之後只要這個 `task_name` 正常呼叫 `POST /api/v1/events/start`，排程就會自動被「摸」一下（reset 計時、狀態轉回 `ok`）——不需要額外呼叫任何 API，跟平常回報流程完全一樣。

一旦超過期限沒收到，狀態會轉成 `missed` 並觸發一次告警（跟 `failed`/`timeout` 走同一條 webhook 管線）；之後在下次真的收到 `start`之前，不會重複告警轟炸。

查詢目前所有排程狀態：`GET /api/v1/schedules`；取消追蹤：`DELETE /api/v1/schedules/:taskName`。

這個功能**完全是 opt-in**——沒有註冊過排程的 `task_name` 不受任何影響，跟現有行為完全相容。

## 資料保留

`CHRONOS_RETENTION_DAYS` 有設定值（> 0）才會啟用。只刪除已經結束的任務（`success` / `failed` / `timeout`），依 `finished_at` 判斷是否超過保留期限；`running` 狀態的任務不管多舊都不會被刪。

```bash
CHRONOS_RETENTION_DAYS=90 ./chronosmonitor   # 只保留最近 90 天的已結束任務
```

啟動時會立刻跑一次清理（不會等到第一個檢查間隔才開始），之後依 `CHRONOS_RETENTION_SWEEP_INTERVAL_SECONDS` 定期執行。

## API 認證

`CHRONOS_API_KEY` 有設定值才會啟用，保護範圍是**整個 `/api/v1/*`**——回報端點（`start`/`heartbeat`/`success`/`failed`）跟查詢端點（`tasks`/`stream`）都要帶 key。`/healthz` 跟前端頁面本身不受影響（不然連登入畫面都載不出來）。

```bash
CHRONOS_API_KEY=some-long-random-string ./chronosmonitor
```

worker 端呼叫 API 時帶 header：

```bash
curl -X POST localhost:8080/api/v1/events/start \
  -H "Authorization: Bearer some-long-random-string" \
  -H 'Content-Type: application/json' \
  -d '{"task_name":"daily-report"}'
```

瀏覽器原生的 `EventSource`（SSE 用）沒辦法自訂 header，所以 `/api/v1/stream` 也接受用 query string 帶 key：`/api/v1/stream?token=some-long-random-string`。儀表板前端會自動處理這件事——第一次打開時若偵測到 401，會跳出一個輸入 API key 的畫面，輸入後存在瀏覽器的 `localStorage`，之後就不用再輸入。

`cmd/demo-worker` 也支援 `CHRONOS_API_KEY` 環境變數，設定跟連 server 用的值一致即可。

## 告警通知

`CHRONOS_ALERT_WEBHOOK_URL` 有設定值才會啟用，只有 `task.failed` 和 `task.timeout` 兩種事件會觸發告警（成功/心跳/開始不會）。

### Slack

在 Slack 建一個 [Incoming Webhook](https://api.slack.com/messaging/webhooks)，拿到 URL 後：

```bash
CHRONOS_ALERT_WEBHOOK_URL=https://hooks.slack.com/services/xxx/yyy/zzz \
CHRONOS_ALERT_WEBHOOK_FORMAT=slack \
./chronosmonitor
```

### Discord

伺服器設定 → 整合 → Webhook，複製 Webhook URL：

```bash
CHRONOS_ALERT_WEBHOOK_URL=https://discord.com/api/webhooks/xxx/yyy \
CHRONOS_ALERT_WEBHOOK_FORMAT=discord \
./chronosmonitor
```

### 訊息範例

```
🔴 *invoice-sync* failed (run `a1b2c3d4-...`) from `laravel-cron`
Duration: 132ms
```connection refused to db:5432```
```

逾時任務會用 ⏰ + `timed out` 字樣。錯誤訊息超過 500 字元會被截斷，避免一則 stack trace 洗掉整個頻道。

### 頻率限制

同一分鐘視窗內超過 `CHRONOS_ALERT_RATE_LIMIT_PER_MINUTE` 則的告警會被丟棄（不送 webhook），只在伺服器 log 留一行 `alert suppressed (rate limit): ...`，等下一個視窗重新計算。這是為了避免像「共用的資料庫掛掉，二十個 cron 同時失敗」這種情境把 Slack 頻道洗爆。

## 本機測試 PostgreSQL（Docker）

不想裝 PostgreSQL 也能快速測：用 Docker 起一個暫時的容器即可，不需要額外的 docker-compose 設定檔。

```bash
# 1. 起一個暫時的 PostgreSQL 容器
docker run -d --name chronos-pg \
  -e POSTGRES_PASSWORD=chronos \
  -e POSTGRES_DB=chronosmonitor \
  -p 15432:5432 \
  postgres:16-alpine

# 2. 等它 ready
docker exec chronos-pg pg_isready -U postgres

# 3. 用 postgres 模式啟動 ChronosMonitor（會自動建表，不用手動 migrate）
CHRONOS_DB_DRIVER=postgres \
CHRONOS_DB_DSN="postgres://postgres:chronos@localhost:15432/chronosmonitor?sslmode=disable" \
go run ./cmd/server
```

測完清掉容器：

```bash
docker rm -f chronos-pg
```

正式環境上串接既有的 PostgreSQL，只要把 `CHRONOS_DB_DSN` 換成真正的連線字串即可，不需要額外設定，schema 會在啟動時自動 migrate。

## API

所有回報類 API 都是 `POST`，`Content-Type: application/json`。若設定了 `CHRONOS_API_KEY`，以下所有 `/api/v1/*` 端點都需要帶 `Authorization: Bearer <key>`（見上方「API 認證」章節）。

### `POST /api/v1/events/start`

回報任務開始。`run_id` 選填，不給的話由伺服器產生 UUID；`ttl_seconds` 選填，不給就永不逾期。

```json
// request
{ "task_name": "daily-report", "source": "laravel-cron", "ttl_seconds": 300 }

// response 201
{ "run_id": "2d62bf7e-..." }
```

### `POST /api/v1/events/heartbeat`

回報存活，會重置 TTL 計時。

```json
{ "run_id": "2d62bf7e-..." }
```

### `POST /api/v1/events/success`

回報成功，伺服器自動計算 `duration_ms`。

```json
{ "run_id": "2d62bf7e-..." }
```

### `POST /api/v1/events/failed`

回報失敗，可附帶錯誤訊息/stack trace。

```json
{ "run_id": "2d62bf7e-...", "error_message": "Traceback ...\n  connection refused" }
```

### `GET /api/v1/tasks?status=`

列出最近 200 筆任務，依開始時間新到舊排序。`status` 選填（`running` / `success` / `failed` / `timeout`）。

### `GET /api/v1/tasks/:runID`

查詢單一任務。

### `GET /api/v1/stream`

SSE endpoint，前端用 `new EventSource('/api/v1/stream')` 訂閱。事件類型：

| Event | 觸發時機 |
|---|---|
| `task.started` | 任務開始 |
| `task.heartbeat` | 收到心跳 |
| `task.succeeded` | 任務成功 |
| `task.failed` | 任務失敗 |
| `task.timeout` | TTL 逾期被判定為 timeout |
| `schedule.missed` | 註冊過的排程逾期沒收到 `start` |

每個事件的 `data` 都是完整的任務物件（JSON）；`schedule.missed` 因為沒有真正的 run，`run_id` 會是空字串，詳情放在 `error_message`。

### `POST /api/v1/schedules`

註冊或更新一個 missed-run 排程期望。`task_name` 沒有對應的真實任務也可以先註冊（例如任務還沒部署，先設好告警）。`expected_interval_seconds` 與 `cron_expression` 必須**擇一**提供。

```json
// request（簡單間隔模式）
{ "task_name": "daily-report", "expected_interval_seconds": 86400, "grace_period_seconds": 1800 }

// request（cron 模式，標準 5 欄位語法）
{ "task_name": "weekday-report", "cron_expression": "0 9 * * 1-5", "grace_period_seconds": 600 }

// response 204 No Content
```

### `GET /api/v1/schedules`

列出所有註冊的排程與目前狀態（`ok` / `missed`）、`last_seen_at`。

### `DELETE /api/v1/schedules/:taskName`

取消追蹤這個 `task_name` 的排程期望。

### `GET /healthz`

健康檢查。

## 測試

```bash
make test   # go test ./... -race
```

涵蓋 store（含 TTL 時間比較邏輯）、broker（並發語意）、handlers（API 契約）、router（含 SPA fallback 的 regression test）、sweeper、db（SQL placeholder 轉換）等後端核心邏輯。前端目前是薄的展示層，未另外加測試。

## 授權

尚未指定授權條款。
