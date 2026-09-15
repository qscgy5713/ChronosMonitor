# ChronosMonitor for Laravel

讓 Laravel 的排程指令自動回報狀態給 [ChronosMonitor](../../README.md)，不用改任何一行原本的指令邏輯。

## 為什麼不是自己包 `Http::post()`

除了幫你打 `start`/`success`/`failed` 這幾個 API，這個套件會用 Laravel 排程本身已經知道的 cron expression，**自動幫你註冊 missed-run 偵測**（見主專案的「Missed Run 偵測」章節）。也就是說：

```php
Schedule::command('backup:run')->dailyAt('02:00')->chronos('daily-backup');
```

這一行同時做到：
1. 執行時回報 `start` / `success` / `failed`
2. 用 `0 2 * * *`（`dailyAt('02:00')` 對應的 cron expression）自動註冊 missed-run 排程——如果哪天這個指令因為 crontab 掛掉、伺服器沒開機而**完全沒被觸發**，ChronosMonitor 一樣抓得到，不用你另外手動呼叫 `/api/v1/schedules`。

## 安裝

```bash
composer require chronos-monitor/laravel
```

Laravel 的 package auto-discovery 會自動註冊 Service Provider 跟 `Chronos` facade，不用手動加。

`.env` 設定：

```env
CHRONOS_BASE_URL=http://localhost:8080
CHRONOS_API_KEY=            # 選填，ChronosMonitor 有開 CHRONOS_API_KEY 才需要
CHRONOS_AUTO_REGISTER_SCHEDULE=true
CHRONOS_GRACE_PERIOD_SECONDS=300
```

## 用法一：排程指令（推薦）

在 `routes/console.php`（或舊版 Laravel 的 `app/Console/Kernel.php`）裡，任何 `$schedule->command(...)` 後面加 `->chronos('task-name')`：

```php
use Illuminate\Support\Facades\Schedule;

Schedule::command('backup:run')->dailyAt('02:00')->chronos('daily-backup');
Schedule::command('invoice:sync')->everyFiveMinutes()->chronos('invoice-sync');
```

`task-name` 是你在 ChronosMonitor 儀表板上看到的名字，跟指令本身的 signature 無關,可以自己取。

可選的第二個參數：

```php
Schedule::command('backup:run')->dailyAt('02:00')->chronos('daily-backup', [
    'grace_period_seconds' => 1800, // 覆寫 config 裡的預設值
]);
```

## 用法二：任意程式碼（queued job、自訂邏輯等）

不是透過 Laravel Scheduler 觸發的程式碼，可以用 `Chronos::track()` 手動包一段邏輯：

```php
use ChronosMonitor\Laravel\Facades\Chronos;

Chronos::track('nightly-reconciliation', function () {
    // 你的邏輯...
    return $result;
});
```

成功會回報 `success`；拋出例外會回報 `failed`（附上例外訊息）並**重新拋出原本的例外**，不會吞掉錯誤。這個用法不會自動註冊 missed-run 排程（沒有 cron expression 可用），要的話自己呼叫：

```php
Chronos::registerSchedule('nightly-reconciliation', '0 3 * * *', gracePeriodSeconds: 900);
```

## 設計上的取捨

- ChronosMonitor 連不上時，不會讓你的排程指令跟著失敗——所有回報都是「盡力而為」，失敗只會寫 log（`Log::warning`），不會拋出例外中斷你的任務。
- `->chronos()` 只在指令**真的被執行**時才會打 API（掛在 `before()`），不會因為 `schedule:run` 每分鐘都跑一次就一直發 HTTP request 註冊排程。
