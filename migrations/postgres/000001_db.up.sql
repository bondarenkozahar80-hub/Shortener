-- Создание таблиц
CREATE TABLE IF NOT EXISTS urls (
                                    id SERIAL PRIMARY KEY,
                                    short VARCHAR(30) UNIQUE NOT NULL,
    original TEXT NOT NULL,
    custom_alias VARCHAR(30) UNIQUE, -- может быть NULL
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP            -- может быть NULL
    );

CREATE TABLE IF NOT EXISTS clicks (
                                      id BIGSERIAL PRIMARY KEY,
                                      short VARCHAR(30) NOT NULL REFERENCES urls(short) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ip VARCHAR(45),           -- nullable, соответствует *string
    referer TEXT,             -- nullable
    browser VARCHAR(50),      -- nullable
    os VARCHAR(50),           -- nullable
    device VARCHAR(50),       -- nullable
    raw_ua TEXT               -- nullable
    );

-- Индексы для быстрого агрегирования
CREATE INDEX idx_clicks_short_created ON clicks(short, created_at);
CREATE INDEX idx_clicks_browser ON clicks(browser);
CREATE INDEX idx_clicks_os ON clicks(os);
CREATE INDEX idx_clicks_device ON clicks(device);

-- ===========================
-- Тестовые данные
-- ===========================

-- Добавляем 3 тестовые ссылки
INSERT INTO urls (short, original, custom_alias, created_at) VALUES
                                                                 ('abc123', 'https://example.com/page1', NULL, '2025-07-15 10:00:00'),
                                                                 ('def456', 'https://example.com/page2', 'myalias', '2025-08-08 12:00:00'),
                                                                 ('ghi789', 'https://example.com/page3', NULL, '2025-08-23 09:00:00');

-- Клики для abc123 (старше 30 дней)
INSERT INTO clicks (short, created_at, ip, referer, browser, os, device, raw_ua) VALUES
                                                                                     ('abc123', '2025-07-10 14:00:00', '192.168.1.1', 'https://google.com', 'Chrome', 'Windows', 'Desktop', 'UA1'),
                                                                                     ('abc123', '2025-07-12 09:00:00', '192.168.1.2', 'https://bing.com', 'Firefox', 'Linux', 'Desktop', 'UA2'),
                                                                                     ('abc123', '2025-07-14 18:30:00', '192.168.1.3', NULL, 'Safari', 'macOS', 'Desktop', 'UA3');

-- Клики для def456 (Last30Days, но не Last7Days)
INSERT INTO clicks (short, created_at, ip, referer, browser, os, device, raw_ua) VALUES
                                                                                     ('def456', '2025-08-05 11:00:00', '10.0.0.1', 'https://example.com', 'Chrome', 'Windows', 'Desktop', 'UA4'),
                                                                                     ('def456', '2025-08-10 16:45:00', '10.0.0.2', NULL, 'Safari', 'iOS', 'Mobile', 'UA5'),
                                                                                     ('def456', '2025-08-15 08:20:00', '10.0.0.3', 'https://google.com', 'Firefox', 'Android', 'Mobile', 'UA6');

-- Клики для ghi789 (Last7Days и Last30Days)
INSERT INTO clicks (short, created_at, ip, referer, browser, os, device, raw_ua) VALUES
                                                                                     ('ghi789', '2025-08-22 10:00:00', '172.16.0.1', 'https://bing.com', 'Chrome', 'Windows', 'Desktop', 'UA7'),
                                                                                     ('ghi789', '2025-08-25 12:30:00', '172.16.0.2', 'https://yahoo.com', 'Edge', 'Windows', 'Desktop', 'UA8'),
                                                                                     ('ghi789', '2025-08-26 15:15:00', '172.16.0.3', 'https://duckduckgo.com', 'Safari', 'macOS', 'Desktop', 'UA9'),
                                                                                     ('ghi789', '2025-08-27 09:45:00', '172.16.0.4', NULL, 'Chrome', 'Android', 'Mobile', 'UA10');
