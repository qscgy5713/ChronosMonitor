<?php

namespace ChronosMonitor\Laravel\Facades;

use ChronosMonitor\Laravel\ChronosClient;
use Illuminate\Support\Facades\Facade;

/**
 * @method static string|null start(string $taskName, ?string $source = null, ?int $ttlSeconds = null)
 * @method static void heartbeat(string $runId)
 * @method static void success(string $runId)
 * @method static void failed(string $runId, string $errorMessage = '')
 * @method static void registerSchedule(string $taskName, string $cronExpression, int $gracePeriodSeconds = 0)
 * @method static mixed track(string $taskName, \Closure $work, ?string $source = null)
 *
 * @see ChronosClient
 */
class Chronos extends Facade
{
    protected static function getFacadeAccessor(): string
    {
        return ChronosClient::class;
    }
}
