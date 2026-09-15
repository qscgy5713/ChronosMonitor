<?php

namespace ChronosMonitor\Laravel;

use Closure;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Log;
use Throwable;

class ChronosClient
{
    public function __construct(
        protected string $baseUrl,
        protected ?string $apiKey = null,
    ) {
    }

    /**
     * Report that a task started. Returns the run_id (needed for
     * success()/failed()/heartbeat()), or null if the report failed — a
     * ChronosMonitor outage should never break the task it's monitoring.
     */
    public function start(string $taskName, ?string $source = null, ?int $ttlSeconds = null): ?string
    {
        $payload = array_filter([
            'task_name' => $taskName,
            'source' => $source,
            'ttl_seconds' => $ttlSeconds,
        ], fn ($value) => $value !== null);

        $response = $this->request()->post($this->url('/api/v1/events/start'), $payload);

        if (! $response->successful()) {
            Log::warning("ChronosMonitor: failed to report start for task [{$taskName}]: HTTP {$response->status()}");

            return null;
        }

        return $response->json('run_id');
    }

    public function heartbeat(string $runId): void
    {
        $this->fireAndForget('heartbeat', ['run_id' => $runId]);
    }

    public function success(string $runId): void
    {
        $this->fireAndForget('success', ['run_id' => $runId]);
    }

    public function failed(string $runId, string $errorMessage = ''): void
    {
        $this->fireAndForget('failed', ['run_id' => $runId, 'error_message' => $errorMessage]);
    }

    /**
     * Register (or update) a missed-run schedule expectation.
     */
    public function registerSchedule(string $taskName, string $cronExpression, int $gracePeriodSeconds = 0): void
    {
        $response = $this->request()->post($this->url('/api/v1/schedules'), [
            'task_name' => $taskName,
            'cron_expression' => $cronExpression,
            'grace_period_seconds' => $gracePeriodSeconds,
        ]);

        if (! $response->successful()) {
            Log::warning("ChronosMonitor: failed to register schedule for task [{$taskName}]: HTTP {$response->status()}");
        }
    }

    /**
     * Run $work, reporting start/success/failed around it. For code that
     * isn't a Laravel-scheduled command (queued jobs, custom Artisan
     * commands, anything) — the ->chronos() Schedule macro is the better
     * fit for scheduled commands specifically.
     */
    public function track(string $taskName, Closure $work, ?string $source = null): mixed
    {
        $runId = $this->start($taskName, $source);

        try {
            $result = $work();
            if ($runId) {
                $this->success($runId);
            }

            return $result;
        } catch (Throwable $e) {
            if ($runId) {
                $this->failed($runId, $e->getMessage());
            }

            throw $e;
        }
    }

    protected function fireAndForget(string $event, array $payload): void
    {
        $response = $this->request()->post($this->url("/api/v1/events/{$event}"), $payload);

        if (! $response->successful()) {
            Log::warning("ChronosMonitor: failed to report {$event}: HTTP {$response->status()}");
        }
    }

    protected function request()
    {
        $request = Http::timeout(5)->acceptJson();

        if ($this->apiKey) {
            $request = $request->withToken($this->apiKey);
        }

        return $request;
    }

    protected function url(string $path): string
    {
        return rtrim($this->baseUrl, '/').$path;
    }
}
