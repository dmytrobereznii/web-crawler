CREATE TABLE IF NOT EXISTS crawl_jobs(
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status text NOT NULL,
    url text NOT NULL,
    duration interval NOT NULL,
    visits int NOT NULL,
    created_at timestamptz NOT NULL,
    completed_at timestamptz
);