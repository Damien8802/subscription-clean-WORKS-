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

    // Выполняем все миграции
    if err := runMigrations(); err != nil {
        return fmt.Errorf("failed to run migrations: %w", err)
    }

    // СОЗДАЕМ ВСЕ НЕДОСТАЮЩИЕ ТАБЛИЦЫ
    if err := createAllMissingTables(); err != nil {
        log.Printf("⚠️ Ошибка при создании дополнительных таблиц: %v", err)
    }

    // Создаем тестового пользователя
    if err := createTestUser(); err != nil {
        log.Printf("⚠️ Ошибка при создании тестового пользователя: %v", err)
    }

    return nil
}

func CloseDB() {
    if Pool != nil {
        Pool.Close()
        log.Println("🛑 Соединение с PostgreSQL закрыто")
    }
}

// runMigrations выполняет все миграции в правильном порядке
func runMigrations() error {
    // Создаем таблицу для отслеживания миграций
    if err := createMigrationsTable(); err != nil {
        return fmt.Errorf("failed to create migrations table: %w", err)
    }

    // Список миграций
    migrations := []struct {
        name string
        fn   func() error
    }{
        {"create_users_table", createUsersTable},
        {"add_telegram_id", addTelegramIDToUsers},
        {"create_subscriptions_tables", createSubscriptionsTables},
        {"create_api_keys_table", createAPIKeysTable},
        {"create_referrals_table", createReferralsTable},
        {"create_twofa_tables", createTwoFATable},
        {"create_referral_program_tables", createReferralProgramTables},
        {"create_verification_tables", createVerificationTables},
        {"create_user_tokens_table", createUserTokensTable},
        {"create_admin_tables", createAdminTables},
        {"create_crm_tables", createCRMTables},
        {"create_notification_settings", createNotificationSettingsTable},
        {"create_notification_log", createNotificationLogTable},
    }

    // Выполняем каждую миграцию, если она не была выполнена
    for _, migration := range migrations {
        if err := runMigrationSimple(migration.name, migration.fn); err != nil {
            return fmt.Errorf("migration %s failed: %w", migration.name, err)
        }
    }

    log.Println("✅ Все миграции успешно выполнены")
    return nil
}

// createMigrationsTable создает таблицу для отслеживания выполненных миграций
func createMigrationsTable() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS schema_migrations (
            id SERIAL PRIMARY KEY,
            migration_name VARCHAR(255) NOT NULL UNIQUE,
            executed_at TIMESTAMP DEFAULT NOW()
        );
    `)
    return err
}

// runMigrationSimple выполняет миграцию, если она еще не была выполнена
func runMigrationSimple(name string, migrationFunc func() error) error {
    // Проверяем, была ли выполнена миграция
    var exists bool
    err := Pool.QueryRow(context.Background(),
        "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE migration_name = $1)", name).Scan(&exists)
    if err != nil {
        return err
    }

    if exists {
        log.Printf("📋 Миграция '%s' уже выполнена, пропускаем", name)
        return nil
    }

    log.Printf("🔄 Выполняется миграция: %s", name)

    // Выполняем миграцию
    if err := migrationFunc(); err != nil {
        return err
    }

    // Записываем в лог миграций
    _, err = Pool.Exec(context.Background(),
        "INSERT INTO schema_migrations (migration_name) VALUES ($1)", name)
    if err != nil {
        return err
    }

    log.Printf("✅ Миграция '%s' успешно выполнена", name)
    return nil
}

func createUsersTable() error {
    // Создаем расширение
    _, err := Pool.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "pgcrypto";`)
    if err != nil {
        return err
    }

    // Создаем таблицу со всеми колонками сразу
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS users (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            email VARCHAR(255) UNIQUE NOT NULL,
            password_hash VARCHAR(255) NOT NULL,
            name VARCHAR(100),
            role VARCHAR(20) DEFAULT 'user',
            email_verified BOOLEAN DEFAULT false,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW(),
            password_changed_at TIMESTAMP DEFAULT NOW(),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111'
        );
    `)
    if err != nil {
        return err
    }

    // Создаем индексы
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
        CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица users создана")
    return nil
}

func addTelegramIDToUsers() error {
    // Проверяем существование колонки
    var exists bool
    err := Pool.QueryRow(context.Background(), `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'users' AND column_name = 'telegram_id'
        );
    `).Scan(&exists)
    if err != nil {
        return err
    }

    if !exists {
        _, err = Pool.Exec(context.Background(), `
            ALTER TABLE users ADD COLUMN telegram_id BIGINT UNIQUE;
        `)
        if err != nil {
            return err
        }
        log.Println("✅ Добавлено поле telegram_id в users")
    }

    return nil
}

func createSubscriptionsTables() error {
    // Создаем таблицу планов
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

    // Создаем таблицу подписок пользователей
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
            ai_quota_used INTEGER DEFAULT 0,
            ai_quota_reset TIMESTAMP DEFAULT NOW(),
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    // Создаем индексы
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_id ON user_subscriptions(user_id);
        CREATE INDEX IF NOT EXISTS idx_user_subscriptions_status ON user_subscriptions(status);
    `)
    if err != nil {
        return err
    }

    // Добавляем базовые тарифы
    var count int
    err = Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM subscription_plans`).Scan(&count)
    if err != nil {
        return err
    }

    if count == 0 {
        _, err = Pool.Exec(context.Background(), `
            INSERT INTO subscription_plans (name, code, description, price_monthly, price_yearly, features, max_users, sort_order) VALUES
            ('Базовый', 'basic', 'Для небольших команд и стартапов', 299, 2990, '["1 пользователь", "5 проектов", "Базовая поддержка"]', 1, 1),
            ('Профессиональный', 'pro', 'Для растущего бизнеса', 999, 9990, '["5 пользователей", "Неограниченно проектов", "Приоритетная поддержка", "API доступ"]', 5, 2),
            ('Корпоративный', 'enterprise', 'Для крупных компаний', 2999, 29990, '["Неограниченно пользователей", "Персональный менеджер", "SLA 99.9%", "Интеграции"]', 999, 3),
            ('Семейный', 'family', 'Для всей семьи', 1499, 14990, '["До 5 участников", "Общая библиотека", "Детский режим"]', 5, 4)
            ON CONFLICT (code) DO NOTHING;
        `)
        if err != nil {
            return err
        }
        log.Println("✅ Базовые тарифы добавлены")
    }

    log.Println("✅ Таблицы подписок созданы")
    return nil
}

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

    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
        CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица api_keys создана")
    return nil
}

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

    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_referrals_user_id ON referrals(user_id);
        CREATE INDEX IF NOT EXISTS idx_referrals_referred_id ON referrals(referred_id);
        CREATE INDEX IF NOT EXISTS idx_referrals_status ON referrals(status);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица referrals создана")
    return nil
}

func createTwoFATable() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS twofa (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            secret VARCHAR(255) NOT NULL,
            enabled BOOLEAN DEFAULT false,
            backup_codes TEXT[] DEFAULT '{}',
            created_at TIMESTAMP DEFAULT NOW(),
            expires_at TIMESTAMP DEFAULT NOW() + INTERVAL '10 minutes',
            updated_at TIMESTAMP DEFAULT NOW(),
            UNIQUE(user_id)
        );
    `)
    if err != nil {
        return err
    }

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

    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_trusted_devices_user_id ON trusted_devices(user_id);
        CREATE INDEX IF NOT EXISTS idx_trusted_devices_expires ON trusted_devices(expires_at);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблицы 2FA созданы")
    return nil
}

func createReferralProgramTables() error {
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

    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_referral_programs_user ON referral_programs(user_id);
        CREATE INDEX IF NOT EXISTS idx_referral_commissions_referrer ON referral_commissions(referrer_id);
        CREATE INDEX IF NOT EXISTS idx_referral_commissions_referred ON referral_commissions(referred_id);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблицы партнерских программ созданы")
    return nil
}

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
    `)
    if err != nil {
        return err
    }

    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_verification_codes_user ON verification_codes(user_id);
        CREATE INDEX IF NOT EXISTS idx_verification_codes_code ON verification_codes(code);
        CREATE INDEX IF NOT EXISTS idx_verification_codes_expires ON verification_codes(expires_at);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблицы верификации созданы")
    return nil
}

func createUserTokensTable() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS user_tokens (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            token TEXT NOT NULL,
            expires_at TIMESTAMP NOT NULL,
            created_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_user_tokens_token ON user_tokens(token);
        CREATE INDEX IF NOT EXISTS idx_user_tokens_user ON user_tokens(user_id);
        CREATE INDEX IF NOT EXISTS idx_user_tokens_expires ON user_tokens(expires_at);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица пользовательских токенов создана")
    return nil
}

func createAdminTables() error {
    // Создаем таблицу администраторов
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS admin_users (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            role VARCHAR(50) DEFAULT 'admin',
            permissions JSONB,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return fmt.Errorf("failed to create admin_users: %w", err)
    }

    // Создаем таблицу логов администраторов
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS admin_logs (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            admin_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
            action VARCHAR(255) NOT NULL,
            entity_type VARCHAR(100),
            entity_id UUID,
            old_data JSONB,
            new_data JSONB,
            ip_address VARCHAR(45),
            user_agent TEXT,
            created_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return fmt.Errorf("failed to create admin_logs: %w", err)
    }

    // Создаем таблицу настроек администратора
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS admin_settings (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            setting_key VARCHAR(255) UNIQUE NOT NULL,
            setting_value TEXT,
            setting_type VARCHAR(50) DEFAULT 'string',
            description TEXT,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return fmt.Errorf("failed to create admin_settings: %w", err)
    }

    // Создаем таблицу платежей
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
    `)
    if err != nil {
        return err
    }

    // Создаем таблицу алертов безопасности
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
    `)
    if err != nil {
        return err
    }

    // Создаем индексы
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_payments_user ON payments(user_id);
        CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
        CREATE INDEX IF NOT EXISTS idx_security_alerts_timestamp ON security_alerts(timestamp);
        CREATE INDEX IF NOT EXISTS idx_security_alerts_ip ON security_alerts(ip);
        CREATE INDEX IF NOT EXISTS idx_admin_logs_admin ON admin_logs(admin_id);
        CREATE INDEX IF NOT EXISTS idx_admin_logs_created ON admin_logs(created_at);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблицы админ-панели созданы")
    return nil
}

func createCRMTables() error {
    // Создаем таблицу клиентов
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS crm_customers (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID REFERENCES users(id) ON DELETE SET NULL,
            name VARCHAR(255) NOT NULL,
            email VARCHAR(255) UNIQUE NOT NULL,
            phone VARCHAR(50),
            company VARCHAR(255),
            status VARCHAR(50) DEFAULT 'lead',
            responsible VARCHAR(255) DEFAULT '',
            source VARCHAR(255) DEFAULT '',
            comment TEXT DEFAULT '',
            lead_score FLOAT DEFAULT 0,
            created_at TIMESTAMP DEFAULT NOW(),
            last_seen TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return err
    }

    // Создаем таблицу сделок
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS crm_deals (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            customer_id UUID NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
            user_id UUID REFERENCES users(id) ON DELETE SET NULL,
            title VARCHAR(255) NOT NULL,
            value DECIMAL(10,2) NOT NULL,
            stage VARCHAR(50) DEFAULT 'lead',
            probability INT DEFAULT 0,
            responsible VARCHAR(255) DEFAULT '',
            source VARCHAR(255) DEFAULT '',
            comment TEXT DEFAULT '',
            expected_close DATE,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW(),
            closed_at TIMESTAMP
        );
    `)
    if err != nil {
        return err
    }

    // Создаем таблицу вложений
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
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось создать таблицу deal_attachments: %v", err)
    }

    // Создаем таблицу истории CRM
    _, err = Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS crm_history (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            entity_type VARCHAR(20) NOT NULL,
            entity_id UUID NOT NULL,
            action VARCHAR(20) NOT NULL,
            user_id UUID REFERENCES users(id) ON DELETE SET NULL,
            changes JSONB,
            created_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось создать таблицу crm_history: %v", err)
    }

    // Создаем индексы
    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_crm_customers_email ON crm_customers(email);
        CREATE INDEX IF NOT EXISTS idx_crm_customers_status ON crm_customers(status);
        CREATE INDEX IF NOT EXISTS idx_crm_customers_user ON crm_customers(user_id);
        CREATE INDEX IF NOT EXISTS idx_crm_customers_lead_score ON crm_customers(lead_score);
        CREATE INDEX IF NOT EXISTS idx_crm_deals_customer ON crm_deals(customer_id);
        CREATE INDEX IF NOT EXISTS idx_crm_deals_stage ON crm_deals(stage);
        CREATE INDEX IF NOT EXISTS idx_crm_deals_user ON crm_deals(user_id);
        CREATE INDEX IF NOT EXISTS idx_deal_attachments_deal ON deal_attachments(deal_id);
        CREATE INDEX IF NOT EXISTS idx_crm_history_entity ON crm_history(entity_type, entity_id);
        CREATE INDEX IF NOT EXISTS idx_crm_history_created ON crm_history(created_at);
    `)
    if err != nil {
        log.Printf("⚠️ Не удалось создать индексы: %v", err)
    }

    log.Println("✅ Таблицы CRM созданы")
    return nil
}

func createNotificationSettingsTable() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS user_notification_settings (
            user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
            telegram_enabled BOOLEAN DEFAULT false,
            email_enabled BOOLEAN DEFAULT true,
            events TEXT[] DEFAULT '{}',
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return fmt.Errorf("failed to create notification_settings table: %w", err)
    }

    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_notification_settings_user ON user_notification_settings(user_id);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица user_notification_settings создана")
    return nil
}

func createNotificationLogTable() error {
    _, err := Pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS notification_log (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID REFERENCES users(id) ON DELETE CASCADE,
            type VARCHAR(50) NOT NULL,
            details JSONB,
            created_at TIMESTAMP DEFAULT NOW()
        );
    `)
    if err != nil {
        return fmt.Errorf("failed to create notification_log table: %w", err)
    }

    _, err = Pool.Exec(context.Background(), `
        CREATE INDEX IF NOT EXISTS idx_notification_log_user ON notification_log(user_id);
        CREATE INDEX IF NOT EXISTS idx_notification_log_created ON notification_log(created_at);
        CREATE INDEX IF NOT EXISTS idx_notification_log_type ON notification_log(type);
    `)
    if err != nil {
        return err
    }

    log.Println("✅ Таблица notification_log создана")
    return nil
}

func createTestUser() error {
    var count int
    err := Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&count)
    if err != nil {
        return err
    }

    if count == 0 {
        // Хэш пароля "admin123"
        hash := "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi"
        _, err = Pool.Exec(context.Background(), `
            INSERT INTO users (email, password_hash, name, role, tenant_id, password_changed_at) 
            VALUES ('admin@example.com', $1, 'Admin', 'admin', '11111111-1111-1111-1111-111111111111', NOW())
            ON CONFLICT (email) DO NOTHING;
        `, hash)
        if err != nil {
            return err
        }
        log.Println("✅ Создан тестовый пользователь: admin@example.com / admin123")
    }
    return nil
}

// createAllMissingTables создает все недостающие таблицы
func createAllMissingTables() error {
    tables := []string{
        // Логистика
        `CREATE TABLE IF NOT EXISTS logistics_orders (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            order_number VARCHAR(100),
            tracking_number VARCHAR(100),
            status VARCHAR(50),
            customer_name VARCHAR(255),
            customer_phone VARCHAR(50),
            customer_address TEXT,
            weight DECIMAL(10,2),
            price DECIMAL(10,2),
            user_id UUID REFERENCES users(id),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        )`,
        
        `CREATE TABLE IF NOT EXISTS logistics_shipments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            order_id UUID REFERENCES logistics_orders(id),
            carrier VARCHAR(100),
            tracking_url TEXT,
            status VARCHAR(50),
            shipped_at TIMESTAMP,
            delivered_at TIMESTAMP,
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Товары и склад
        `CREATE TABLE IF NOT EXISTS warehouses (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255),
            address TEXT,
            phone VARCHAR(50),
            manager VARCHAR(255),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        `CREATE TABLE IF NOT EXISTS products (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            sku VARCHAR(100) UNIQUE,
            name VARCHAR(255),
            description TEXT,
            price DECIMAL(10,2),
            cost DECIMAL(10,2),
            quantity INTEGER DEFAULT 0,
            warehouse_id UUID REFERENCES warehouses(id),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Заказы
        `CREATE TABLE IF NOT EXISTS orders (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            order_number VARCHAR(100) UNIQUE,
            customer_name VARCHAR(255),
            customer_email VARCHAR(255),
            customer_phone VARCHAR(50),
            total_amount DECIMAL(10,2),
            status VARCHAR(50),
            payment_status VARCHAR(50),
            user_id UUID REFERENCES users(id),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        )`,
        
        `CREATE TABLE IF NOT EXISTS order_items (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            order_id UUID REFERENCES orders(id),
            product_id UUID REFERENCES products(id),
            quantity INTEGER,
            price DECIMAL(10,2),
            total DECIMAL(10,2),
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Поставщики
        `CREATE TABLE IF NOT EXISTS suppliers (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255),
            contact_person VARCHAR(255),
            phone VARCHAR(50),
            email VARCHAR(255),
            address TEXT,
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        `CREATE TABLE IF NOT EXISTS purchase_orders (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            po_number VARCHAR(100) UNIQUE,
            supplier_id UUID REFERENCES suppliers(id),
            order_date DATE,
            total_amount DECIMAL(10,2),
            status VARCHAR(50),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Финансы
        `CREATE TABLE IF NOT EXISTS chart_of_accounts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            code VARCHAR(20),
            name VARCHAR(255),
            type VARCHAR(50),
            parent_id UUID REFERENCES chart_of_accounts(id),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        `CREATE TABLE IF NOT EXISTS journal_entries (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            entry_number VARCHAR(100),
            entry_date DATE,
            description TEXT,
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        `CREATE TABLE IF NOT EXISTS cash_operations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            operation_date DATE,
            type VARCHAR(50),
            amount DECIMAL(10,2),
            description TEXT,
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Интеграции
        `CREATE TABLE IF NOT EXISTS bitrix_sync_logs (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            entity_type VARCHAR(100),
            entity_id UUID,
            action VARCHAR(50),
            status VARCHAR(50),
            error_message TEXT,
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Аналитика
        `CREATE TABLE IF NOT EXISTS analytics_metrics (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            metric_name VARCHAR(255),
            metric_value DECIMAL(10,2),
            period DATE,
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        `CREATE TABLE IF NOT EXISTS analytics_ltv_predictions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            customer_id UUID,
            predicted_ltv DECIMAL(10,2),
            confidence FLOAT,
            period DATE,
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Уведомления
        `CREATE TABLE IF NOT EXISTS notification_templates (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255),
            subject VARCHAR(255),
            body TEXT,
            type VARCHAR(50),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Документы
        `CREATE TABLE IF NOT EXISTS documents (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255),
            file_path TEXT,
            file_size BIGINT,
            mime_type VARCHAR(100),
            entity_type VARCHAR(100),
            entity_id UUID,
            uploaded_by UUID REFERENCES users(id),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Активности
        `CREATE TABLE IF NOT EXISTS activities (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID REFERENCES users(id),
            action VARCHAR(255),
            details JSONB,
            ip_address VARCHAR(45),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Категории
        `CREATE TABLE IF NOT EXISTS categories (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255),
            parent_id UUID REFERENCES categories(id),
            sort_order INTEGER,
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Теги
        `CREATE TABLE IF NOT EXISTS tags (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(100) UNIQUE,
            color VARCHAR(20),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
        
        // Связи тегов с сущностями
        `CREATE TABLE IF NOT EXISTS entity_tags (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            entity_type VARCHAR(100),
            entity_id UUID,
            tag_id UUID REFERENCES tags(id),
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        )`,
    }
    
    for _, sql := range tables {
        if _, err := Pool.Exec(context.Background(), sql); err != nil {
            log.Printf("⚠️ Ошибка создания таблицы: %v", err)
        }
    }
    
    log.Println("✅ Все дополнительные таблицы проверены/созданы")
    return nil
}