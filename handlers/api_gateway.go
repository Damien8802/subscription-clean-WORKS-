package handlers

import (
"bytes"
"encoding/json"
"fmt"
"io"
"log"
"net/http"
"strconv"
"strings"
"time"

"subscription-system/config"
"subscription-system/database"
"subscription-system/models"

"github.com/gin-gonic/gin"
)

type GenerateAPIKeyRequest struct {
Name       string                 `json:"name" binding:"required"`
QuotaLimit int64                  `json:"quota_limit" binding:"required,min=1000"`
Providers  map[string]interface{} `json:"providers" binding:"required"`
}

func GenerateAPIKeyHandler(c *gin.Context) {
cfg := config.Load()
if cfg.SkipAuth {
var userID string
err := database.Pool.QueryRow(c.Request.Context(), "SELECT id FROM users ORDER BY created_at LIMIT 1").Scan(&userID)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "no users found"})
return
}
var req GenerateAPIKeyRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}
rawKey, apiKey, err := models.GenerateAPIKey(userID, req.Name, req.Providers, -1)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
return
}
c.JSON(http.StatusCreated, gin.H{
"api_key":   rawKey,
"key_id":    apiKey.ID,
"name":      apiKey.Name,
"quota":     -1,
"providers": req.Providers,
})
return
}
userID, exists := c.Get("userID")
if !exists {
c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
return
}
var req GenerateAPIKeyRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}
role, _ := c.Get("userRole")
isAdmin := role == "admin"
var quotaLimit int64
if isAdmin {
quotaLimit = -1
} else {
plan, err := models.GetUserActivePlan(userID.(string))
if err != nil {
c.JSON(http.StatusForbidden, gin.H{"error": "active subscription required to create API keys"})
return
}
if plan.AIQuota == 0 {
c.JSON(http.StatusForbidden, gin.H{"error": "your plan does not include AI access"})
return
}
quotaLimit = plan.AIQuota
}
rawKey, apiKey, err := models.GenerateAPIKey(userID.(string), req.Name, req.Providers, quotaLimit)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
return
}
c.JSON(http.StatusCreated, gin.H{
"api_key":   rawKey,
"key_id":    apiKey.ID,
"name":      apiKey.Name,
"quota":     apiKey.QuotaLimit,
"providers": req.Providers,
})
}

func ListAPIKeysHandler(c *gin.Context) {
userID, exists := c.Get("userID")
if !exists {
c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
return
}
keys, err := models.GetAPIKeysByUser(userID.(string))
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list API keys"})
return
}
result := make([]gin.H, 0, len(keys))
for _, k := range keys {
result = append(result, gin.H{
"id":          k.ID,
"name":        k.Name,
"quota_limit": k.QuotaLimit,
"quota_used":  k.QuotaUsed,
"is_active":   k.IsActive,
"created_at":  k.CreatedAt,
"providers":   k.ProviderCredentials,
})
}
c.JSON(http.StatusOK, gin.H{"keys": result})
}

func RevokeAPIKeyHandler(c *gin.Context) {
keyID := c.Param("id")
userID, exists := c.Get("userID")
if !exists {
c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
return
}
keys, err := models.GetAPIKeysByUser(userID.(string))
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify key ownership"})
return
}
found := false
for _, k := range keys {
if k.ID == keyID {
found = true
break
}
}
if !found {
c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
return
}
falseVal := false
err = models.UpdateAPIKey(keyID, nil, &falseVal, nil)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke key"})
return
}
c.JSON(http.StatusOK, gin.H{"message": "key revoked"})
}

type ChatCompletionRequest struct {
Model    string `json:"model" binding:"required"`
Messages []struct {
Role    string `json:"role"`
Content string `json:"content"`
} `json:"messages" binding:"required"`
Stream bool `json:"stream"`
}

func ChatCompletionsHandler(c *gin.Context) {
apiKeyID, exists := c.Get("apiKeyID")
if !exists {
c.JSON(http.StatusInternalServerError, gin.H{"error": "missing api key context"})
return
}
userID, _ := c.Get("apiKeyUserID")
if userID == nil {
userID = "unknown"
}
quotaLimit, _ := c.Get("quotaLimit")
isUnlimited := quotaLimit != nil && quotaLimit.(int64) == -1

providerCredentialsRaw, _ := c.Get("providerCredentials")
var providerCreds map[string]interface{}
if err := json.Unmarshal(providerCredentialsRaw.([]byte), &providerCreds); err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid provider credentials"})
return
}

var req ChatCompletionRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

if !isUnlimited {
plan, err := models.GetUserActivePlan(userID.(string))
if err != nil {
c.JSON(http.StatusForbidden, gin.H{"error": "no active subscription or plan not found"})
return
}
var allowedModels []string
if err := json.Unmarshal(plan.AIModels, &allowedModels); err != nil {
allowedModels = []string{}
}
allowed := false
for _, m := range allowedModels {
if m == "*" || m == req.Model || strings.HasPrefix(req.Model, m+"/") {
allowed = true
break
}
}
if !allowed {
c.JSON(http.StatusForbidden, gin.H{"error": "model not allowed by your subscription plan"})
return
}
}

var provider, apiKey, baseURL string
switch {
case req.Model == "deepseek-chat" || req.Model == "deepseek-reasoner":
provider = "deepseek"
apiKey, _ = providerCreds["deepseek"].(string)
baseURL = "https://api.deepseek.com/v1/chat/completions"
case strings.HasPrefix(req.Model, "openai/"):
provider = "openai"
apiKey, _ = providerCreds["openai"].(string)
baseURL = "https://api.openai.com/v1/chat/completions"
case strings.HasPrefix(req.Model, "yandex/"):
provider = "yandex"
cfg := config.Load()
if cfg.YandexFolderID == "" || cfg.YandexAPIKey == "" {
c.JSON(http.StatusBadRequest, gin.H{"error": "YandexGPT not configured"})
return
}
apiKey = cfg.YandexAPIKey
baseURL = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
case strings.HasPrefix(req.Model, "gigachat/"):
provider = "gigachat"
cfg := config.Load()
if cfg.GigaChatAuthKey == "" {
c.JSON(http.StatusBadRequest, gin.H{"error": "GigaChat not configured"})
return
}
apiKey = cfg.GigaChatAuthKey
baseURL = "https://gigachat.devices.sberbank.ru/api/v1/chat/completions"
case strings.HasPrefix(req.Model, "ollama/"):
provider = "ollama"
apiKey = ""
baseURL = "http://localhost:11434/v1/chat/completions"
default:
c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported model"})
return
}

if provider != "ollama" && apiKey == "" {
c.JSON(http.StatusBadRequest, gin.H{"error": "API key for " + provider + " not configured"})
return
}

startTime := time.Now()

var jsonData []byte
if provider == "yandex" {
cfg := config.Load()
var yandexReq struct {
ModelURI string `json:"modelUri"`
CompletionOptions struct {
Stream      bool    `json:"stream"`
Temperature float64 `json:"temperature"`
MaxTokens   int     `json:"maxTokens"`
} `json:"completionOptions"`
Messages []struct {
Role string `json:"role"`
Text string `json:"text"`
} `json:"messages"`
}
yandexReq.ModelURI = fmt.Sprintf("gpt://%s/yandexgpt-lite", cfg.YandexFolderID)
yandexReq.CompletionOptions.Stream = false
yandexReq.CompletionOptions.Temperature = 0.6
yandexReq.CompletionOptions.MaxTokens = 2000
for _, msg := range req.Messages {
yandexReq.Messages = append(yandexReq.Messages, struct {
Role string `json:"role"`
Text string `json:"text"`
}{
Role: msg.Role,
Text: msg.Content,
})
}
jsonData, _ = json.Marshal(yandexReq)
} else {
jsonData, _ = json.Marshal(req)
}

proxyReq, err := http.NewRequest("POST", baseURL, bytes.NewBuffer(jsonData))
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
return
}
if provider != "ollama" {
proxyReq.Header.Set("Authorization", "Bearer "+apiKey)
}
proxyReq.Header.Set("Content-Type", "application/json")

client := &http.Client{}
resp, err := client.Do(proxyReq)
durationMs := int(time.Since(startTime).Milliseconds())

if err != nil {
errMsg := err.Error()
_ = models.LogAIRequest(apiKeyID.(string), fmt.Sprint(userID), req.Model, 0, 0, 0, durationMs, 500, &errMsg)
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to call provider"})
return
}
defer resp.Body.Close()

bodyBytes, _ := io.ReadAll(resp.Body)
log.Printf("📥 Полный ответ от провайдера: %s", string(bodyBytes))

if resp.StatusCode != http.StatusOK {
_ = models.LogAIRequest(apiKeyID.(string), fmt.Sprint(userID), req.Model, 0, 0, 0, durationMs, resp.StatusCode, nil)
c.Data(resp.StatusCode, "application/json", bodyBytes)
return
}

var answer string
if provider == "yandex" {
var yandexResp struct {
Result struct {
Alternatives []struct {
Message struct {
Role string `json:"role"`
Text string `json:"text"`
} `json:"message"`
} `json:"alternatives"`
Usage struct {
InputTextTokens  string `json:"inputTextTokens"`
CompletionTokens string `json:"completionTokens"`
TotalTokens      string `json:"totalTokens"`
} `json:"usage"`
} `json:"result"`
}
if err := json.Unmarshal(bodyBytes, &yandexResp); err != nil {
log.Printf("❌ Ошибка парсинга ответа Yandex: %v", err)
c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid response from YandexGPT"})
return
}
if len(yandexResp.Result.Alternatives) == 0 {
c.JSON(http.StatusOK, gin.H{
"choices": []map[string]interface{}{
{
"message": map[string]interface{}{
"content": "Не удалось получить ответ от AI.",
},
},
},
})
return
}
answer = yandexResp.Result.Alternatives[0].Message.Text
totalTokens, _ := strconv.Atoi(yandexResp.Result.Usage.TotalTokens)
if !isUnlimited && totalTokens > 0 {
_ = models.IncrementQuotaUsed(apiKeyID.(string), int64(totalTokens))
}
} else {
var providerResp struct {
Choices []struct {
Message struct {
Content string `json:"content"`
} `json:"message"`
} `json:"choices"`
Usage struct {
TotalTokens int `json:"total_tokens"`
} `json:"usage"`
Error *struct {
Message string `json:"message"`
} `json:"error,omitempty"`
}
if err := json.Unmarshal(bodyBytes, &providerResp); err != nil {
log.Printf("❌ Ошибка парсинга ответа: %v", err)
c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid response from provider"})
return
}
if providerResp.Error != nil {
c.JSON(http.StatusOK, gin.H{
"choices": []map[string]interface{}{
{
"message": map[string]interface{}{
"content": providerResp.Error.Message,
},
},
},
})
return
}
if len(providerResp.Choices) == 0 {
c.JSON(http.StatusOK, gin.H{
"choices": []map[string]interface{}{
{
"message": map[string]interface{}{
"content": "Не удалось получить ответ от AI.",
},
},
},
})
return
}
answer = providerResp.Choices[0].Message.Content
if !isUnlimited && providerResp.Usage.TotalTokens > 0 {
_ = models.IncrementQuotaUsed(apiKeyID.(string), int64(providerResp.Usage.TotalTokens))
}
}

c.JSON(http.StatusOK, gin.H{
"choices": []map[string]interface{}{
{
"message": map[string]interface{}{
"content": answer,
},
},
},
})
}
