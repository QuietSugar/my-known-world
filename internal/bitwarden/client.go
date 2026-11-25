package bitwarden

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultServerURL = "https://vault.bitwarden.com"

// Item 一条解密后的条目
type Item struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	Notes        string   `json:"notes,omitempty"`
	Uris         []string `json:"uris,omitempty"`
	RevisionDate string   `json:"revision_date,omitempty"`
}

// SkippedItem 同步中跳过的条目
type SkippedItem struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

const (
	skipOrg = "组织条目暂不支持（需先解组织密钥）"
)

// Config 访问单一 Bitwarden / Vaultwarden 账号的凭据
type Config struct {
	ServerURL      string `json:"server_url"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	MasterPassword string `json:"master_password"`
}

// Enabled 是否启用（ServerURL 非空）
func (c Config) Enabled() bool {
	return strings.TrimSpace(c.ServerURL) != ""
}

// Client Bitwarden 官方 API 客户端
type Client struct {
	serverURL    string
	clientID     string
	clientSecret string
	token        string
	http         *http.Client
}

// NewClient 构造客户端
func NewClient(cfg Config) *Client {
	base := strings.TrimSpace(cfg.ServerURL)
	if base == "" {
		base = defaultServerURL
	}
	return &Client{
		serverURL:    strings.TrimRight(base, "/"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		http:         &http.Client{Timeout: 60 * time.Second},
	}
}

// Login 用个人 API Key 换 access token
func (c *Client) Login() error {
	form := url.Values{
		"grant_type":        {"client_credentials"},
		"client_id":         {c.clientID},
		"client_secret":     {c.clientSecret},
		"scope":             {"api"},
		"device_identifier": {"myknownworld-bitwarden"},
		"device_name":       {"myknownworld"},
		"device_type":       {"0"},
	}
	resp, err := c.http.PostForm(c.serverURL+"/identity/connect/token", form)
	if err != nil {
		return errOf(errKindNetwork,
			"检查「服务地址」是否可达",
			"连接 %s 失败: %v", c.serverURL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := explainAuthError(raw)
		kind := errKindAuth
		hint := "在网页端「设置 → 安全 → 密钥」核对 Client ID 与 Client Secret"
		if resp.StatusCode >= 500 {
			kind = errKindUpstream
			hint = "服务端出错了，稍后重试；持续失败请看服务端日志"
		}
		return errOf(kind, hint, "鉴权失败（HTTP %d）：%s", resp.StatusCode, msg)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("解析鉴权响应失败: %w", err)
	}
	if out.AccessToken == "" {
		return errOf(errKindUpstream,
			"服务端返回的内容不合预期，可能是版本不兼容",
			"鉴权没有返回 access_token")
	}
	c.token = out.AccessToken
	return nil
}

type syncResponse struct {
	Profile struct {
		Email string `json:"email"`
		Key   string `json:"key"`
	} `json:"profile"`
	Items []cipherItem `json:"ciphers"`
}

// Pull 拉取 /api/sync
func (c *Client) Pull() (*syncResponse, error) {
	req, err := http.NewRequest(http.MethodGet, c.serverURL+"/api/sync", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errOf(errKindNetwork,
			"检查「服务地址」是否可达", "拉取条目失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		kind, hint := errKindUpstream, "服务端出错了，稍后重试；持续失败请看服务端日志"
		if resp.StatusCode == http.StatusUnauthorized {
			kind, hint = errKindAuth, "凭据可能已失效：在网页端重新生成 Client Secret"
		}
		return nil, errOf(kind, hint, "拉取条目失败（HTTP %d）：%s", resp.StatusCode, explainAuthError(raw))
	}

	var sync syncResponse
	if err := json.NewDecoder(resp.Body).Decode(&sync); err != nil {
		return nil, fmt.Errorf("解析条目失败: %w", err)
	}
	return &sync, nil
}

func (c *Client) prelogin(email string) (preloginInfo, error) {
	var info preloginInfo
	body, _ := json.Marshal(map[string]string{"email": email})
	resp, err := c.http.Post(c.serverURL+"/identity/accounts/prelogin", "application/json", bytes.NewReader(body))
	if err != nil {
		return info, errOf(errKindNetwork,
			"检查「服务地址」是否可达", "查询 KDF 参数失败: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("查询 KDF 参数失败（HTTP %d）：%s", resp.StatusCode, explainAuthError(raw))
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return info, fmt.Errorf("解析 KDF 参数失败: %w", err)
	}
	return info, nil
}

// Fetch 拉取全量条目并解出明文
func (c *Client) Fetch(masterPassword string) ([]Item, []SkippedItem, error) {
	if c.token == "" {
		if err := c.Login(); err != nil {
			return nil, nil, err
		}
	}
	sync, err := c.Pull()
	if err != nil {
		return nil, nil, err
	}
	if sync.Profile.Key == "" {
		return nil, nil, errOf(errKindUpstream,
			"服务端没有返回解锁所需的密钥，可能是版本不兼容",
			"服务端没有返回 profile.key，无法解出条目内容")
	}
	if masterPassword == "" {
		return nil, nil, errOf(errKindConfig,
			"在配置中填写主密码", "需要主密码才能解开条目")
	}

	info, err := c.prelogin(sync.Profile.Email)
	if err != nil {
		return nil, nil, err
	}
	if info.KDF != 0 {
		return nil, nil, errOf(errKindConfig,
			"该账号的密钥派生方式暂不支持，暂无法同步",
			"该账号使用 Argon2id（kdfType=%d），本应用目前只支持 PBKDF2", info.KDF)
	}
	keys, err := unlockUserKey(sync.Profile.Key, masterPassword, sync.Profile.Email, info.KDFIterations)
	if err != nil {
		return nil, nil, err
	}

	items := make([]Item, 0, len(sync.Items))
	var skipped []SkippedItem
	for _, it := range sync.Items {
		if it.Type != 1 || it.ID == "" || it.DeletedDate != "" {
			continue
		}
		if it.OrganizationID != "" {
			skipped = append(skipped, SkippedItem{ID: it.ID, Reason: skipOrg})
			continue
		}
		item, reason := decryptItem(keys.entry, it)
		if reason != "" {
			skipped = append(skipped, SkippedItem{ID: it.ID, Reason: reason})
			continue
		}
		items = append(items, item)
	}
	return items, skipped, nil
}

type cipherItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Notes          string `json:"notes"`
	RevisionDate   string `json:"revisionDate"`
	Type           int    `json:"type"`
	DeletedDate    string `json:"deletedDate"`
	OrganizationID string `json:"organizationId"`
	Login          *struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Uris     []struct {
			URI string `json:"uri"`
		} `json:"uris"`
	} `json:"login"`
	Fields []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"fields"`
}

func decryptItem(k encKey, it cipherItem) (Item, string) {
	item := Item{ID: it.ID, RevisionDate: it.RevisionDate}

	var err error
	if item.Name, err = k.decryptStr(it.Name); err != nil {
		return Item{}, "名称解不开：" + err.Error()
	}
	if item.Notes, err = k.decryptStr(it.Notes); err != nil {
		return Item{}, "备注解不开：" + err.Error()
	}
	if it.Login != nil {
		if item.Username, err = k.decryptStr(it.Login.Username); err != nil {
			return Item{}, "用户名解不开：" + err.Error()
		}
		if item.Password, err = k.decryptStr(it.Login.Password); err != nil {
			return Item{}, "密码解不开：" + err.Error()
		}
		for _, u := range it.Login.Uris {
			if u.URI == "" {
				continue
			}
			if plain, uerr := k.decryptStr(u.URI); uerr == nil {
				item.Uris = append(item.Uris, plain)
			}
		}
	}
	if item.Password == "" {
		for _, f := range it.Fields {
			if !strings.EqualFold(f.Name, "password") {
				continue
			}
			if p, ferr := k.decryptStr(f.Value); ferr == nil {
				item.Password = p
			}
			break
		}
	}
	return item, ""
}

func explainAuthError(raw []byte) string {
	var e struct {
		Message            string          `json:"message"`
		Error              string          `json:"error"`
		ErrorDescription   string          `json:"error_description"`
		ValidationMessages json.RawMessage `json:"validationErrors"`
	}
	if json.Unmarshal(raw, &e) == nil {
		msg := firstNonEmpty(e.Message, e.ErrorDescription, e.Error)
		switch {
		case strings.Contains(msg, "Invalid client_id"), strings.Contains(msg, "client_secret"):
			return "Client ID 或 Client Secret 不对（在网页端「设置 → 安全 → 密钥」重新生成）"
		case strings.Contains(msg, "device_identifier"):
			return "服务端要求设备标识"
		}
		if msg != "" {
			return msg
		}
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
