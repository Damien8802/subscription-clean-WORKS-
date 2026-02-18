package yandex_search

import (
    "strings"
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "time"
)

type SearchClient struct {
    apiKey     string
    folderID   string
    httpClient *http.Client
}

func NewClient(apiKey, folderID string) *SearchClient {
    return &SearchClient{
        apiKey:     apiKey,
        folderID:   folderID,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

type SearchRequest struct {
    Query        string
    GroupsOnPage int
    DocsInGroup  int
    MaxPassages  int
}

type AsyncSearchResponse struct {
    ID          string `json:"id"`
    Description string `json:"description"`
    Done        bool   `json:"done"`
}

type OperationResponse struct {
    Done     bool   `json:"done"`
    Response *struct {
        Type    string `json:"@type"`
        RawData string `json:"rawData"`
    } `json:"response"`
    Error *struct {
        Code    int    `json:"code"`
        Message string `json:"message"`
    } `json:"error"`
}

type SearchResult struct {
    URL     string
    Title   string
    Snippet string
}

func (c *SearchClient) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
    asyncResp, err := c.startAsyncSearch(ctx, req)
    if err != nil {
        return nil, err
    }
    opResp, err := c.waitForOperation(ctx, asyncResp.ID)
    if err != nil {
        return nil, err
    }
    if opResp.Response == nil || opResp.Response.RawData == "" {
        return nil, fmt.Errorf("no data in operation response")
    }
    xmlData, err := base64.StdEncoding.DecodeString(opResp.Response.RawData)
    if err != nil {
        return nil, fmt.Errorf("failed to decode base64: %w", err)
    }
    return parseXMLResults(xmlData)
}

func (c *SearchClient) startAsyncSearch(ctx context.Context, req SearchRequest) (*AsyncSearchResponse, error) {
    url := "https://searchapi.api.cloud.yandex.net/v2/web/searchAsync"
    bodyMap := map[string]interface{}{
        "query": map[string]interface{}{
            "queryText":  req.Query,
            "searchType": 1, // 1 = WEB
        },
        "groupsOnPage": req.GroupsOnPage,
        "docsInGroup":  req.DocsInGroup,
        "maxPassages":  req.MaxPassages,
    }
    jsonBody, err := json.Marshal(bodyMap)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    httpReq.Header.Set("Authorization", "Api-Key "+c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-folder-id", c.folderID)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("http error: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("searchAsync failed (status %d): %s", resp.StatusCode, string(bodyBytes))
    }

    var asyncResp AsyncSearchResponse
    if err := json.NewDecoder(resp.Body).Decode(&asyncResp); err != nil {
        return nil, fmt.Errorf("failed to decode async response: %w", err)
    }
    return &asyncResp, nil
}

func (c *SearchClient) waitForOperation(ctx context.Context, opID string) (*OperationResponse, error) {
    pollInterval := 2 * time.Second
    maxAttempts := 30
    for i := 0; i < maxAttempts; i++ {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(pollInterval):
        }
        url := fmt.Sprintf("https://operation.api.cloud.yandex.net/operations/%s", opID)
        req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
        if err != nil {
            continue
        }
        req.Header.Set("Authorization", "Api-Key "+c.apiKey)
        resp, err := c.httpClient.Do(req)
        if err != nil {
            continue
        }
        defer resp.Body.Close()
        if resp.StatusCode != 200 {
            continue
        }
        var opResp OperationResponse
        if err := json.NewDecoder(resp.Body).Decode(&opResp); err != nil {
            return nil, fmt.Errorf("failed to decode operation response: %w", err)
        }
        if opResp.Done {
            if opResp.Error != nil {
                return nil, fmt.Errorf("operation error: %s", opResp.Error.Message)
            }
            return &opResp, nil
        }
    }
    return nil, fmt.Errorf("timeout waiting for operation")
}

func parseXMLResults(xmlData []byte) ([]SearchResult, error) {
    type Passage struct {
        Text string `xml:",chardata"`
    }
    type Doc struct {
        URL      string    `xml:"url"`
        Title    string    `xml:"title"`
        Headline string    `xml:"headline"`
        Passage  []Passage `xml:"passage"`
        // Добавляем возможные поля
        Meta     []struct {
            Name  string `xml:"name,attr"`
            Value string `xml:"value,attr"`
        } `xml:"meta"`
    }
    type Group struct {
        Doc Doc `xml:"doc"`
    }
    type Response struct {
        Results []struct {
            Grouping []struct {
                Group []Group `xml:"group"`
            } `xml:"grouping"`
        } `xml:"results"`
    }
    var yandexResp struct {
        Response Response `xml:"response"`
    }
    if err := xml.Unmarshal(xmlData, &yandexResp); err != nil {
        return nil, fmt.Errorf("failed to parse XML: %w", err)
    }
    var results []SearchResult
    if len(yandexResp.Response.Results) > 0 {
        for _, grouping := range yandexResp.Response.Results[0].Grouping {
            for _, group := range grouping.Group {
                doc := group.Doc
                // Собираем все возможные текстовые поля
                var allTexts []string
                if doc.Headline != "" {
                    allTexts = append(allTexts, doc.Headline)
                }
                for _, p := range doc.Passage {
                    if p.Text != "" {
                        allTexts = append(allTexts, p.Text)
                    }
                }
                for _, m := range doc.Meta {
                    if m.Value != "" {
                        allTexts = append(allTexts, m.Value)
                    }
                }
                snippet := strings.Join(allTexts, " | ")
                if len(snippet) > 500 {
                    snippet = snippet[:500] + "…"
                }
                results = append(results, SearchResult{
                    URL:     doc.URL,
                    Title:   doc.Title,
                    Snippet: snippet,
                })
            }
        }
    }
    return results, nil
}



