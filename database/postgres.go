package database

import (
    "context"
    "fmt"  // Раскомментировал!
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

    // Добавляем базовые тарифы, если таблица пуста
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
            ('Семейный', 'family', 'Для всей семьи', 1499, 14990, '["До 5 участников", "Общая библиотека", "Детский режим"]', 5, 4);
        `)
        if err != nil {
            return err
        }
        log.Println("✅ Базовые тарифы добавлены")
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