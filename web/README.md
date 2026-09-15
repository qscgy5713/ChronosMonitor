# ChronosMonitor Dashboard（前端）

ChronosMonitor 的即時監控儀表板。Vue 3（Composition API）+ Tailwind CSS v4 + Vite，透過 SSE 訂閱後端事件，狀態變化立即反映，不用 polling、不用重新整理。

整體專案說明（後端 API、架構、部署）請看[專案根目錄的 README](../README.md)，這份只講前端本身。

## 開發

後端要先跑起來（提供 `/api/*` 與 `/api/v1/stream`）：

```bash
# repo 根目錄
make dev-backend    # http://localhost:8080
```

再啟動前端 dev server：

```bash
npm install
npm run dev          # http://localhost:5173
```

`vite.config.js` 已經設定 `/api` proxy 到 `http://localhost:8080`，開發時直接打 `http://localhost:5173` 就好，不會有 CORS 問題。

## Build

```bash
npm run build
```

輸出目錄是 `../internal/webui/dist`（不是預設的 `dist/`），因為後端會用 `go:embed` 把這裡的產物直接打包進 Go 執行檔，做成單一執行檔部署。正常情況下不用手動跑這個指令——用 repo 根目錄的 `make build` 會自動處理（build 前端 → embed → `go build`）。

## 專案結構

```
src/
  main.js                    掛載 App
  style.css                  @import "tailwindcss"
  App.vue                    版面組裝
  composables/
    useTasks.js              資料層：抓初始清單 + 訂閱 SSE + 計算統計，畫面唯一的資料來源
  components/
    StatCard.vue             KPI 卡片
    StatusBadge.vue          狀態徽章（running/success/failed/timeout）
    FailurePanel.vue         近期失敗/逾時清單，含 error_message stack trace
    TaskTable.vue            完整任務列表
  utils/
    format.js                時間/耗時/ID 格式化
```

## 運作方式

1. 進站時 `useTasks()` 先 `GET /api/v1/tasks` 抓最近 200 筆任務當初始資料。
2. 接著開 `new EventSource('/api/v1/stream')`，訂閱五種事件：`task.started`、`task.heartbeat`、`task.succeeded`、`task.failed`、`task.timeout`。
3. 每個事件都帶完整的任務物件，直接用 `run_id` 當 key upsert 進本地的 `Map`，畫面（統計數字、失敗清單、任務表）都是這份 `Map` 算出來的 computed。

沒有額外的狀態管理套件（Pinia/Vuex）——單頁面、單一資料來源，用 `reactive(Map)` + `computed` 就夠了，之後如果頁面變多再考慮加。

## 技術棧

- [Vite](https://vite.dev/)
- [Vue 3](https://vuejs.org/)（`<script setup>`）
- [Tailwind CSS v4](https://tailwindcss.com/)（`@tailwindcss/vite` plugin，沒有額外的 `tailwind.config.js`）

## 測試

目前沒有前端測試。這是薄的展示層（fetch + SSE + render），真正有邏輯的部分（TTL 判斷、事件廣播、SPA fallback）都在後端且已經覆蓋測試，詳見根目錄 README 的「測試」章節。
