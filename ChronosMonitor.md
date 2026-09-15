# 系統架構書：跨語言輕量級排程監控面板 (ChronosMonitor)

## 1. 專案背景與目標 (Background & Objectives)
在微服務或混合語言的架構中（如同時存在 Laravel 系統排程與 Go 背景常駐執行緒），缺乏一個統一、輕量的監控中心。開發者往往需要登入不同伺服器或查閱分散的 Log 才能確認任務狀態。
**目標：** 打造一個無關語言 (Language-agnostic)、易於自託管 (Self-hosted) 的監控服務。任何語言的 Worker 只需要透過極簡的 HTTP API 即可回報任務狀態。

## 2. 核心功能 (MVP Features)
1. **HTTP 狀態回報 API：** 接收 `start`, `success`, `failed`, `heartbeat` 等狀態。
2. **即時監控儀表板：** 動態展示目前正在執行的任務、近期失敗任務、任務平均耗時。
3. **任務逾期警告 (TTL)：** 若任務超過預期時間未回報完成，自動標記為 Timeout/殭屍狀態。
4. **錯誤堆疊追蹤 (Stack Trace)：** 任務失敗時可夾帶錯誤訊息，方便在面板上直接 Debug。

## 3. 技術選型 (Tech Stack)
* **後端 (Backend):** Go (Golang) + Gin / Fiber 框架
  * *優勢：* 輕量、編譯成單一執行檔易於部署、高併發處理 HTTP 請求。
* **前端 (Frontend):** Vue 3 (Composition API) + Tailwind CSS + Vite
  * *優勢：* 反應式更新即時數據、組件化管理圖表與列表。
* **資料庫 (Database):** SQLite (預設) / PostgreSQL (可選)
  * *優勢：* MVP 階段使用 SQLite 可達成「零依賴」部署，適合輕量級監控；後期資料量大可平滑遷移至 PostgreSQL。
* **即時通訊 (Real-time):** WebSocket 或 Server-Sent Events (SSE)
  * *優勢：* 任務狀態改變時主動推播至 Vue 儀表板，無須頻繁 Polling。

## 4. 系統架構圖 (System Architecture)

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