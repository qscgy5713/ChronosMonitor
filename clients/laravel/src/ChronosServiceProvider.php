<?php

namespace ChronosMonitor\Laravel;

use Illuminate\Console\Scheduling\Event;
use Illuminate\Support\ServiceProvider;

class ChronosServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->mergeConfigFrom(__DIR__.'/../config/chronos.php', 'chronos');

        $this->app->singleton(ChronosClient::class, function ($app) {
            return new ChronosClient(
                $app['config']->get('chronos.base_url'),
                $app['config']->get('chronos.api_key'),
            );
        });
    }

    public function boot(): void
    {
        $this->publishes([
            __DIR__.'/../config/chronos.php' => config_path('chronos.php'),
        ], 'chronos-config');

        $this->registerScheduleMacro();
    }

    /**
     * Adds ->chronos($taskName) to Illuminate\Console\Scheduling\Event, so
     * an existing scheduled command definition:
     *
     *   $schedule->command('backup:run')->dailyAt('02:00');
     *
     * becomes:
     *
     *   $schedule->command('backup:run')->dailyAt('02:00')->chronos('daily-backup');
     *
     * with zero changes to the command itself. It hooks before()/onSuccess()
     * /onFailure() to report start/success/failed, and — unless disabled via
     * config — registers a missed-run schedule using the event's own cron
     * expression, so a cron/scheduler outage that stops this task from ever
     * running gets caught too, not just a run that starts and fails.
     */
    protected function registerScheduleMacro(): void
    {
        $client = $this->app->make(ChronosClient::class);
        $config = $this->app->make('config');

        Event::macro('chronos', function (string $taskName, array $options = []) use ($client, $config) {
            /** @var Event $this */
            $cronExpression = $this->getExpression();
            $runId = null;

            $this->before(function () use ($client, $config, $taskName, $cronExpression, $options, &$runId) {
                if ($cronExpression && $config->get('chronos.auto_register_schedule', true)) {
                    $client->registerSchedule(
                        $taskName,
                        $cronExpression,
                        $options['grace_period_seconds'] ?? $config->get('chronos.grace_period_seconds', 300),
                    );
                }

                $runId = $client->start($taskName, source: 'laravel-schedule');
            });

            $this->onSuccess(function () use ($client, &$runId) {
                if ($runId) {
                    $client->success($runId);
                }
            });

            $this->onFailure(function () use ($client, &$runId) {
                /** @var Event $this */
                if ($runId) {
                    $client->failed($runId, "Scheduled command exited with status {$this->exitCode}.");
                }
            });

            return $this;
        });
    }
}
