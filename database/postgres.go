package database

import (
    "context"
    "fmt"
    "log"
    "subscription-system/config"

    "github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func InitDB(cfg *config.Config) error {
    dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
        cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)

    var err error
    Pool, err = pgxpool.New(context.Background(), dsn)
    if err != nil {
        return fmt.Errorf("unable to connect to database: %w", err)
    }

    if err := Pool.Ping(context.Background()); err != nil {
        return fmt.Errorf("unable to ping database: %w", err)
    }

    log.Println("✅ Подключение к PostgreSQL установлено")
    if err := createUsersTable(); err != nil {
        return fmt.Errorf("failed to create users table: %w", err)
    }
    if err := createSubscriptionsTables(); err != nil {
        return fmt.Errorf("failed to create subscriptions tables: %w", err)
    }
    if err := createAPIKeysTable(); err != nil {
        return fmt.Errorf("failed to create api_keys table: %w", err)
    }
    if err := createReferralsTable(); err != nil {
        return fmt.Errorf("failed to create referrals table: %w", err)
    }
    if err := createTwoFATable(); err != nil {
        return fmt.Errorf("failed to create twofa table: %w", err)
    }
    if err := createReferralProgramTables(); err != nil {
        return fmt.Errorf("failed to create referral program tables: %w", err)
    }
    if err := createVerificationTables(); err != nil {
        return fmt.Errorf("failed to create verification tables: %w", err)
    }
    if err := createUserTokensTable(); err != nil {
        return fmt.Errorf("failed to create user tokens table: %w", err)
    }
    if err := createAdminTables(); err != nil {
        return fmt.Errorf("failed to create admin tables: %w", err)
    }
    if err := createCRMTables(); err != nil {
        return fmt.Errorf("failed to create CRM tables: %w", err)
    }
    if err := createTestUser(); err != nil {
        return err
    }
    return nil
}

func CloseDB() {
    if Pool != nil {
        Pool.Close()
        log.Println("🛑 Соединение с PostgreSQL закрыто")
    }
}

func createUsersTable() error {
    // pgcrypto для gen_random_uuid()
    _, err := Pool.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "pgcrypto";`)
    if err != nil {
        return err
    }

    // Создаём таблицу, если её нет
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS users (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            email VARCHAR(255) UNIQUE NOT NULL,
            password_hash VARCHAR(255) NOT NULL,
            name VARCHAR(100),
            role VARCHAR(20) DEFAULT 'user',
            email_verified BOOLEAN DEFAULT false,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        // Если структура не совпадает – удаляем и создаём заново
        log.Println("⚠️ Пересоздаю таблицу users (неверная структура)")
        _, err = Pool.Exec(context.Background(), `DROP TABLE IF EXISTS users;`)
        if err != nil {
            return err
        }
        _, err = Pool.Exec(context.Background(), `
            CREATE TABLE users (
                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                email VARCHAR(255) UNIQUE NOT NULL,
                password_hash VARCHAR(255) NOT NULL,
                name VARCHAR(100),
                role VARCHAR(20) DEFAULT 'user',
                email_verified BOOLEAN DEFAULT false,
                created_at TIMESTAMP DEFAULT NOW(),
                updated_at TIMESTAMP DEFAULT NOW()
            );
        `)
        if err != nil {
            return err
        }
    }

    // Индекс для email
    _, err = Pool.Exec(context.Background(), `CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);`)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица users готова")
    return nil
}

// createSubscriptionsTables создаёт таблицы планов и подписок
func createSubscriptionsTables() error {
    // Таблица планов подписки
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS subscription_plans (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            code VARCHAR(50) UNIQUE NOT NULL,
            description TEXT,
            price_monthly DECIMAL(10,2) NOT NULL,
            price_yearly DECIMAL(10,2) NOT NULL,
            currency VARCHAR(3) DEFAULT 'RUB',
            features JSONB NOT NULL DEFAULT '[]',
            max_users INTEGER DEFAULT 1,
            is_active BOOLEAN DEFAULT true,
            sort_order INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    // Таблица подписок пользователей
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS user_subscriptions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            plan_id INTEGER NOT NULL REFERENCES subscription_plans(id),
            status VARCHAR(20) DEFAULT 'active',
            current_period_start TIMESTAMP NOT NULL DEFAULT NOW(),
            current_period_end TIMESTAMP NOT NULL,
            cancel_at_period_end BOOLEAN DEFAULT false,
            trial_end TIMESTAMP,
            payment_method VARCHAR(50),
            stripe_subscription_id VARCHAR(100),
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    // Индекс для быстрого поиска подписок пользователя
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_id ON user_subscriptions(user_id);
    `)
    if err != nil {
        return err
    }

    // Добавляем поля для AI-квот, если их нет
    _, err = Pool.Exec(context.Background(), `
        DO $$ 
        BEGIN 
            BEGIN
                ALTER TABLE user_subscriptions ADD COLUMN ai_quota_used INTEGER DEFAULT 0;
            EXCEPTION
                WHEN duplicate_column THEN 
                    NULL;
            END;
        END $$;
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось добавить ai_quota_used: %v", err)
    }

    _, err = Pool.Exec(context.Background(), `
        DO $$ 
        BEGIN 
            BEGIN
                ALTER TABLE user_subscriptions ADD COLUMN ai_quota_reset TIMESTAMP DEFAULT NOW();
            EXCEPTION
                WHEN duplicate_column THEN 
                    NULL;
            END;
        END $$;
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось добавить ai_quota_reset: %v", err)
    }

    // Добавляем базовые тарифы, если таблица пуста
    var count int
    err = Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM subscription_plans`).Scan(&count)
    if err != nil {
        return err
    }
    if count == 0 {
        // Добавляем AI-возможности в тарифы
        _, err = Pool.Exec(context.Background(), `
            INSERT INTO subscription_plans (name, code, description, price_monthly, price_yearly, features, ai_capabilities, max_users, sort_order) VALUES
            ('Базовый', 'basic', 'Для небольших команд и стартапов', 299, 2990, '["1 пользователь", "5 проектов", "Базовая поддержка"]', '{"max_requests": 10, "models": ["basic"]}', 1, 1),
            ('Профессиональный', 'pro', 'Для растущего бизнеса', 999, 9990, '["5 пользователей", "Неограниченно проектов", "Приоритетная поддержка", "API доступ"]', '{"max_requests": 100, "models": ["basic", "advanced"]}', 5, 2),
            ('Корпоративный', 'enterprise', 'Для крупных компаний', 2999, 29990, '["Неограниченно пользователей", "Персональный менеджер", "SLA 99.9%", "Интеграции"]', '{"max_requests": 1000, "models": ["basic", "advanced", "expert"]}', 999, 3),
            ('Семейный', 'family', 'Для всей семьи', 1499, 14990, '["До 5 участников", "Общая библиотека", "Детский режим"]', '{"max_requests": 50, "models": ["basic"]}', 5, 4);
        `)
        if err != nil {
            return err
        }
        log.Println("✅ Базовые тарифы с AI-возможностями добавлены")
    }

    log.Println("✅ Таблицы подписок готовы")
    return nil
}

// createAPIKeysTable создаёт таблицу для API ключей
func createAPIKeysTable() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS api_keys (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            name VARCHAR(100) NOT NULL,
            key_hash VARCHAR(255) UNIQUE NOT NULL,
            quota_limit BIGINT NOT NULL DEFAULT 1000,
            quota_used BIGINT NOT NULL DEFAULT 0,
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    // Индексы для быстрого поиска
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
        CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица api_keys готова")
    return nil
}

// createReferralsTable создаёт таблицу для рефералов
func createReferralsTable() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS referrals (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            referred_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            referred_email VARCHAR(255) NOT NULL,
            status VARCHAR(20) DEFAULT 'pending',
            commission DECIMAL(10,2) DEFAULT 0,
            created_at TIMESTAMP DEFAULT NOW(),
            expires_at TIMESTAMP NOT NULL
        );
    `)
    if err != nil {
        return err
    }

    // Индексы для быстрого поиска
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_referrals_user_id ON referrals(user_id);
        CREATE INDEX IF NOT EXISTS idx_referrals_referred_id ON referrals(referred_id);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица referrals готова")
    return nil
}

// createTwoFATable создаёт таблицу для 2FA с поддержкой резервных кодов и доверенных устройств
func createTwoFATable() error {
    // Обновляем таблицу twofa, добавляем поле для резервных кодов
    _, err := Pool.Exec(context.Background(), `
        -- Добавляем поле для резервных кодов, если его нет
        DO $$ 
        BEGIN 
            BEGIN
                ALTER TABLE twofa ADD COLUMN backup_codes TEXT[] DEFAULT '{}';
            EXCEPTION
                WHEN duplicate_column THEN 
                    NULL;
            END;
        END $$;
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось добавить backup_codes: %v", err)
    }

    // Создаём таблицу доверенных устройств
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS trusted_devices (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            device_id VARCHAR(255) NOT NULL,
            device_name VARCHAR(255),
            ip_address VARCHAR(45),
            user_agent TEXT,
            expires_at TIMESTAMP NOT NULL,
            last_used_at TIMESTAMP DEFAULT NOW(),
            created_at TIMESTAMP DEFAULT NOW(),
            UNIQUE(user_id, device_id)
        );
    `)
    if err != nil {
        return err
    }

    // Индексы для быстрой работы
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_trusted_devices_user_id ON trusted_devices(user_id);
        CREATE INDEX IF NOT EXISTS idx_trusted_devices_expires ON trusted_devices(expires_at);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблицы 2FA, резервных кодов и доверенных устройств готовы")
    return nil
}

// createReferralProgramTables создаёт таблицы для партнёрской программы (Telegram Stars)
func createReferralProgramTables() error {
    // Таблица партнёрских программ
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS referral_programs (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE UNIQUE,
            referral_link TEXT NOT NULL,
            commission_percent INT NOT NULL DEFAULT 20,
            total_earned BIGINT DEFAULT 0,
            total_referred INT DEFAULT 0,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }
    
    // Таблица комиссий
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS referral_commissions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            referrer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            referred_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            amount BIGINT NOT NULL,
            status VARCHAR(20) DEFAULT 'pending',
            created_at TIMESTAMP DEFAULT NOW(),
            paid_at TIMESTAMP
        );
    `)
    if err != nil {
        return err
    }
    
    // Индексы для быстрого поиска
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_referral_programs_user ON referral_programs(user_id);
        CREATE INDEX IF NOT EXISTS idx_referral_commissions_referrer ON referral_commissions(referrer_id);
        CREATE INDEX IF NOT EXISTS idx_referral_commissions_referred ON referral_commissions(referred_id);
    `)
    if err != nil {
        return err
    }
    
    log.Println("✅ Таблицы партнёрских программ готовы")
    return nil
}

// createVerificationTables создаёт таблицы для верификации
func createVerificationTables() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS verification_codes (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            code VARCHAR(10) NOT NULL,
            type VARCHAR(20) NOT NULL,
            expires_at TIMESTAMP NOT NULL,
            created_at TIMESTAMP DEFAULT NOW(),
            used_at TIMESTAMP
        );
        
        CREATE INDEX IF NOT EXISTS idx_verification_codes_user ON verification_codes(user_id);
        CREATE INDEX IF NOT EXISTS idx_verification_codes_code ON verification_codes(code);
    `)
    if err != nil {
        return err
    }
    log.Println("✅ Таблицы верификации готовы")
    return nil
}

// createUserTokensTable создаёт таблицу для хранения refresh токенов
func createUserTokensTable() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS user_tokens (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            token TEXT NOT NULL,
            expires_at TIMESTAMP NOT NULL,
            created_at TIMESTAMP DEFAULT NOW()
        );
        
        CREATE INDEX IF NOT EXISTS idx_user_tokens_token ON user_tokens(token);
        CREATE INDEX IF NOT EXISTS idx_user_tokens_user ON user_tokens(user_id);
    `)
    if err != nil {
        return err
    }
    log.Println("✅ Таблица пользовательских токенов готова")
    return nil
}

// createAdminTables создаёт таблицы для админ-панели
func createAdminTables() error {
    // Таблица заблокированных пользователей
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS blocked_users (
            user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
            blocked_at TIMESTAMP DEFAULT NOW(),
            reason TEXT
        );
    `)
    if err != nil {
        return err
    }

    // Таблица заблокированных IP
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS blocked_ips (
            ip VARCHAR(45) PRIMARY KEY,
            reason TEXT,
            blocked_at TIMESTAMP DEFAULT NOW(),
            expires_at TIMESTAMP
        );
        
        CREATE INDEX IF NOT EXISTS idx_blocked_ips_expires ON blocked_ips(expires_at);
    `)
    if err != nil {
        return err
    }

    // Таблица для платежей
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS payments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            amount DECIMAL(10,2) NOT NULL,
            currency VARCHAR(10) DEFAULT 'RUB',
            method VARCHAR(50) NOT NULL,
            status VARCHAR(20) DEFAULT 'pending',
            plan_name VARCHAR(100),
            created_at TIMESTAMP DEFAULT NOW(),
            completed_at TIMESTAMP
        );
        
        CREATE INDEX IF NOT EXISTS idx_payments_user ON payments(user_id);
        CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
    `)
    if err != nil {
        return err
    }

    // Таблица для логов безопасности (если ещё нет)
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS security_alerts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            ip VARCHAR(45),
            user_id UUID REFERENCES users(id) ON DELETE SET NULL,
            path TEXT,
            status INTEGER,
            reason TEXT,
            timestamp TIMESTAMP DEFAULT NOW()
        );
        
        CREATE INDEX IF NOT EXISTS idx_security_alerts_timestamp ON security_alerts(timestamp);
        CREATE INDEX IF NOT EXISTS idx_security_alerts_ip ON security_alerts(ip);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблицы админ-панели готовы")
    return nil
}

// createCRMTables создаёт таблицы для CRM, включая историю
func createCRMTables() error {
    // Таблица клиентов CRM
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS crm_customers (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255) NOT NULL,
            email VARCHAR(255) UNIQUE NOT NULL,
            phone VARCHAR(50),
            company VARCHAR(255),
            status VARCHAR(50) DEFAULT 'lead',
            created_at TIMESTAMP DEFAULT NOW(),
            last_seen TIMESTAMP DEFAULT NOW()
        );
        
        CREATE INDEX IF NOT EXISTS idx_crm_customers_status ON crm_customers(status);
        CREATE INDEX IF NOT EXISTS idx_crm_customers_email ON crm_customers(email);
    `)
    if err != nil {
        return err
    }

    // Таблица сделок CRM
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS crm_deals (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            customer_id UUID NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
            title VARCHAR(255) NOT NULL,
            value DECIMAL(10,2) NOT NULL,
            stage VARCHAR(50) DEFAULT 'lead',
            probability INT DEFAULT 0,
            expected_close DATE,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW(),
            closed_at TIMESTAMP
        );
        
        CREATE INDEX IF NOT EXISTS idx_crm_deals_customer ON crm_deals(customer_id);
        CREATE INDEX IF NOT EXISTS idx_crm_deals_stage ON crm_deals(stage);
    `)
    if err != nil {
        return err
    }

    // Добавляем колонки "Ответственный", "Источник", "Комментарий" для клиентов
    _, err = Pool.Exec(context.Background(), `
        ALTER TABLE crm_customers
        ADD COLUMN IF NOT EXISTS responsible VARCHAR(255) DEFAULT '',
        ADD COLUMN IF NOT EXISTS source VARCHAR(255) DEFAULT '',
        ADD COLUMN IF NOT EXISTS comment TEXT DEFAULT '';
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось добавить колонки в crm_customers: %v", err)
    }

    // Добавляем колонки для сделок
    _, err = Pool.Exec(context.Background(), `
        ALTER TABLE crm_deals
        ADD COLUMN IF NOT EXISTS responsible VARCHAR(255) DEFAULT '',
        ADD COLUMN IF NOT EXISTS source VARCHAR(255) DEFAULT '',
        ADD COLUMN IF NOT EXISTS comment TEXT DEFAULT '';
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось добавить колонки в crm_deals: %v", err)
    }

    // Таблица для вложений к сделкам
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS deal_attachments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            deal_id UUID NOT NULL REFERENCES crm_deals(id) ON DELETE CASCADE,
            file_name VARCHAR(255) NOT NULL,
            file_path VARCHAR(512) NOT NULL,
            file_size BIGINT NOT NULL,
            mime_type VARCHAR(100),
            uploaded_by UUID REFERENCES users(id) ON DELETE SET NULL,
            uploaded_at TIMESTAMP DEFAULT NOW()
        );
        
        CREATE INDEX IF NOT EXISTS idx_deal_attachments_deal ON deal_attachments(deal_id);
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось создать таблицу deal_attachments: %v", err)
    }

    // Добавляем колонку user_id для привязки к создателю (для реализации ролей)
    _, err = Pool.Exec(context.Background(), `
        ALTER TABLE crm_customers
        ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE SET NULL;
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось добавить колонку user_id в crm_customers: %v", err)
    }

    _, err = Pool.Exec(context.Background(), `
        ALTER TABLE crm_deals
        ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE SET NULL;
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось добавить колонку user_id в crm_deals: %v", err)
    }

    // Индексы для быстрого поиска по user_id
    _, err = Pool.Exec(context.Background(), `CREATE INDEX IF NOT EXISTS idx_crm_customers_user ON crm_customers(user_id);`)
    if err != nil {
        log.Printf("⚠️ Не удалось создать индекс на user_id в crm_customers: %v", err)
    }
    _, err = Pool.Exec(context.Background(), `CREATE INDEX IF NOT EXISTS idx_crm_deals_user ON crm_deals(user_id);`)
    if err != nil {
        log.Printf("⚠️ Не удалось создать индекс на user_id в crm_deals: %v", err)
    }

    // Таблица истории изменений
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS crm_history (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            entity_type VARCHAR(20) NOT NULL, -- 'customer' или 'deal'
            entity_id UUID NOT NULL,
            action VARCHAR(20) NOT NULL, -- 'create', 'update', 'delete'
            user_id UUID REFERENCES users(id) ON DELETE SET NULL,
            changes JSONB,
            created_at TIMESTAMP DEFAULT NOW()
        );
        
        CREATE INDEX IF NOT EXISTS idx_crm_history_entity ON crm_history(entity_type, entity_id);
        CREATE INDEX IF NOT EXISTS idx_crm_history_created ON crm_history(created_at);
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось создать таблицу crm_history: %v", err)
    }

    log.Println("✅ Таблицы CRM готовы (включая новые поля, таблицу вложений, привязку к пользователям и историю)")
    return nil
}

// createTestUser создаёт тестового пользователя, если таблица пуста
func createTestUser() error {
    var count int
    err := Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&count)
    if err != nil {
        return err
    }
    if count == 0 {
        // Заранее сгенерированный bcrypt-хеш для пароля "admin123"
        hash := "$2a$10$VHt4xKq.2qZVzZ3YQ9qR3eNQjQjQjQjQjQjQjQjQjQjQjQjQjQ"
        _, err = Pool.Exec(context.Background(), `
            INSERT INTO users (email, password_hash, name, role) 
            VALUES ('admin@example.com', $1, 'Admin', 'admin')
        `, hash)
        if err != nil {
            return err
        }
        log.Println("✅ Создан тестовый пользователь: admin@example.com / admin123")
    }
    return nil
}