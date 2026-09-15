<?php

return [
    'base_url' => env('CHRONOS_BASE_URL', 'http://localhost:8080'),
    'api_key' => env('CHRONOS_API_KEY'),

    // Whenever ->chronos() is used on a scheduled event, automatically
    // register a missed-run schedule with ChronosMonitor using that event's
    // own cron expression — no separate call to POST /api/v1/schedules
    // needed. Disable if you'd rather manage schedules yourself.
    'auto_register_schedule' => env('CHRONOS_AUTO_REGISTER_SCHEDULE', true),

    // Grace period (seconds) added to a schedule's own cadence before a
    // missed run is flagged, used when auto-registering.
    'grace_period_seconds' => env('CHRONOS_GRACE_PERIOD_SECONDS', 300),
];
