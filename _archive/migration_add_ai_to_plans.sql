-- Добавляем AI-квоту и разрешённые модели в тарифы
ALTER TABLE subscription_plans 
ADD COLUMN IF NOT EXISTS ai_quota BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS ai_models JSONB NOT NULL DEFAULT '[]';

-- Обновляем существующие тарифы (примерные значения, подставь свои)
UPDATE subscription_plans SET 
ai_quota = 100000,  -- 100k токенов
ai_models = '["deepseek-chat"]'::jsonb
WHERE code = 'basic';

UPDATE subscription_plans SET 
ai_quota = 1000000, -- 1M токенов
ai_models = '["deepseek-chat", "deepseek-reasoner"]'::jsonb
WHERE code = 'pro';

UPDATE subscription_plans SET 
ai_quota = 10000000, -- 10M токенов
ai_models = '["deepseek-chat", "deepseek-reasoner", "openai/gpt-4.1-mini"]'::jsonb
WHERE code = 'enterprise';

UPDATE subscription_plans SET 
ai_quota = -1, -- безлимит
ai_models = '["*"]'::jsonb
WHERE code = 'family'; -- или любой твой VIP-тариф
