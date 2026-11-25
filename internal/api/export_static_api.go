package api

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type staticCredentialItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Account  string `json:"account"`
	Password string `json:"password"`
	Env      string `json:"env"`
	Role     string `json:"role"`
}

type staticServiceGroup struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Host        string                 `json:"host"`
	Port        string                 `json:"port"`
	Env         string                 `json:"env"`
	Groups      []string               `json:"groups"`
	Credentials []staticCredentialItem `json:"credentials"`
}

// projectExportStaticRoute handles POST /api/workspace/projects/{id}/export-static
func (h *Handler) projectExportStaticRoute(w http.ResponseWriter, r *http.Request, g *graph.Graph, subPath string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	idx := strings.Index(subPath, "/export-static")
	if idx == -1 {
		writeError(w, http.StatusNotFound, "not_found", "invalid export-static path")
		return
	}

	projectID, err := url.PathUnescape(subPath[:idx])
	if err != nil || projectID == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "project id is invalid")
		return
	}

	workspace, err := AggregateProjectWorkspace(g, projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", err.Error())
		return
	}

	// 1. 批量解密该项目下的所有凭据
	passwords := make(map[string]string)
	if h.SecretProvider != nil {
		for _, c := range workspace.Credentials {
			ref := ""
			if c.AccountCredential != nil && c.AccountCredential.AccessReference != "" {
				ref = c.AccountCredential.AccessReference
			} else {
				ref = c.ID
			}
			sec, err := h.SecretProvider.Read(r.Context(), ref)
			if err == nil && len(sec.Value) > 0 {
				passwords[c.ID] = string(sec.Value)
			}
		}
	}

	// 2. 宿主服务与环境归类聚合
	serviceGroups := buildStaticServiceGroups(workspace, passwords)

	// 3. 渲染单文件纯静态独立 HTML
	htmlContent := renderStandaloneHTML(workspace.Project.Name, projectID, serviceGroups, workspace.Credentials, passwords)

	// 4. 落盘到静态目录
	exportDir := os.Getenv("STATIC_EXPORT_DIR")
	if exportDir == "" {
		exportDir = "/data/static"
	}

	projectSlug := strings.TrimPrefix(projectID, "projects/")
	projectSlug = strings.ReplaceAll(projectSlug, "/", "_")
	targetDir := filepath.Join(exportDir, "projects", projectSlug)
	historyDir := filepath.Join(targetDir, "history")

	if err := os.MkdirAll(historyDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir_failed", fmt.Sprintf("failed to create static directory: %v", err))
		return
	}

	now := time.Now()
	ts := now.Format("20060102_150405")
	indexPath := filepath.Join(targetDir, "index.html")
	historyPath := filepath.Join(historyDir, ts+".html")

	if err := os.WriteFile(indexPath, []byte(htmlContent), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "write_failed", fmt.Sprintf("failed to write static index.html: %v", err))
		return
	}
	_ = os.WriteFile(historyPath, []byte(htmlContent), 0644)

	relativeURL := fmt.Sprintf("/static/projects/%s/index.html", projectSlug)
	historyURL := fmt.Sprintf("/static/projects/%s/history/%s.html", projectSlug, ts)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"projectId":       projectID,
		"projectSlug":     projectSlug,
		"filePath":        indexPath,
		"historyPath":     historyPath,
		"url":             relativeURL,
		"historyUrl":      historyURL,
		"credentialCount": len(workspace.Credentials),
		"decryptedCount":  len(passwords),
		"generatedAt":     now.Format(time.RFC3339),
	})
}

func classifyEnvKey(text string) string {
	t := strings.ToLower(text)
	if strings.Contains(t, "prod") || strings.Contains(t, "生产") {
		return "prod"
	}
	if strings.Contains(t, "test") || strings.Contains(t, "测试") || strings.Contains(t, "一体化") {
		return "test"
	}
	if strings.Contains(t, "c0130") || strings.Contains(t, "现场") {
		return "c0130"
	}
	return "infra"
}

// classifyServiceGroups 依据服务/主机名称与 ID 推断所属业务分组，按图表专用 ID 归一
func classifyServiceGroups(key string) []string {
	ts := strings.ToLower(key)
	if strings.Contains(ts, "conn") || strings.Contains(ts, "连接器") {
		return []string{"连接器"}
	}
	if strings.Contains(ts, "region") || strings.Contains(ts, "区域") {
		return []string{"区域节点"}
	}
	if strings.Contains(ts, "biz") || strings.Contains(ts, "业务节点") {
		return []string{"业务节点"}
	}
	if strings.Contains(ts, "bastion") || strings.Contains(ts, "堡垒") {
		return []string{"运维接入"}
	}
	if strings.Contains(ts, "shenyu") || strings.Contains(ts, "网关") {
		return []string{"网关组件"}
	}
	if strings.Contains(ts, "vpn") {
		return []string{"运维接入", "网关组件"}
	}
	return []string{"基础组件"}
}

func buildStaticServiceGroups(data ProjectWorkspaceData, passwords map[string]string) []staticServiceGroup {
	credToHost := make(map[string]string)
	for _, r := range data.Relations {
		if r.Predicate == graph.HasCredential {
			credToHost[r.To] = r.From
		}
	}

	groupsMap := make(map[string]*staticServiceGroup)
	groupOrder := make([]string, 0)

	for _, c := range data.Credentials {
		hostID := credToHost[c.ID]
		groupKey := hostID
		groupName := ""
		host := ""
		port := ""
		env := classifyEnvKey(c.ID + " " + c.Name)

		if hostID != "" {
			var foundInst *graph.Node
			for _, inst := range data.Instances {
				if inst.ID == hostID {
					foundInst = &inst
					break
				}
			}
			if foundInst != nil {
				groupName = foundInst.Name
				if foundInst.SoftwareInstance != nil {
					if len(foundInst.SoftwareInstance.Domains) > 0 {
						host = foundInst.SoftwareInstance.Domains[0]
					} else {
						host = foundInst.Name
					}
					if len(foundInst.SoftwareInstance.Ports) > 0 {
						port = fmt.Sprintf("%d", foundInst.SoftwareInstance.Ports[0])
					}
				}
				env = classifyEnvKey(foundInst.ID + " " + foundInst.Name)
			} else {
				var foundMach *graph.Node
				for _, m := range data.Machines {
					if m.ID == hostID {
						foundMach = &m
						break
					}
				}
				if foundMach != nil {
					groupName = foundMach.Name
					if foundMach.Machine != nil {
						if foundMach.Machine.Hostname != "" {
							host = foundMach.Machine.Hostname
						} else if len(foundMach.Machine.IPs) > 0 {
							host = foundMach.Machine.IPs[0]
						}
					}
					env = classifyEnvKey(foundMach.ID + " " + foundMach.Name)
				}
			}
		}

		if groupKey == "" {
			groupKey = c.ID
			groupName = c.Name
		}

		groups := classifyServiceGroups(groupKey + " " + groupName)
		env = classifyEnvKey(c.ID + " " + c.Name)

		account := ""
		if c.AccountCredential != nil {
			account = c.AccountCredential.Account
		}

		role := c.Name
		parts := strings.Split(groupName, " ")
		if len(parts) > 0 && strings.HasPrefix(role, parts[0]) {
			cleaned := strings.TrimSpace(strings.TrimPrefix(role, parts[0]))
			if cleaned != "" {
				role = cleaned
			}
		}

		item := staticCredentialItem{
			ID:       c.ID,
			Name:     c.Name,
			Account:  account,
			Password: passwords[c.ID],
			Env:      env,
			Role:     role,
		}

		if g, exists := groupsMap[groupKey]; exists {
			g.Credentials = append(g.Credentials, item)
		} else {
			grp := &staticServiceGroup{
				ID:          groupKey,
				Name:        groupName,
				Host:        host,
				Port:        port,
				Env:         env,
				Groups:      groups,
				Credentials: []staticCredentialItem{item},
			}
			groupsMap[groupKey] = grp
			groupOrder = append(groupOrder, groupKey)
		}
	}

	result := make([]staticServiceGroup, 0, len(groupOrder))
	for _, k := range groupOrder {
		result = append(result, *groupsMap[k])
	}
	return result
}

func renderStandaloneHTML(projectName, projectID string, groups []staticServiceGroup, credentials []graph.Node, passwords map[string]string) string {
	groupsJSON, _ := json.Marshal(groups)
	safeProjectName := html.EscapeString(projectName)
	genTime := time.Now().Format("2006-01-02 15:04:05")

	tmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>__PROJECT_NAME__ · 密码速查静态档案</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: #ffffff;
      color: #111827;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      font-size: 12px;
      line-height: 1.4;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
    }
    header {
      background: #ffffff;
      border-bottom: 1px solid #e5e7eb;
      padding: 6px 14px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      position: sticky;
      top: 0;
      z-index: 50;
    }
    .header-left {
      display: flex;
      align-items: baseline;
      gap: 8px;
    }
    .header-title {
      font-size: 14px;
      font-weight: 600;
      color: #111827;
    }
    .header-sub {
      font-size: 12px;
      color: #6b7280;
    }
    .header-meta {
      font-size: 11px;
      color: #9ca3af;
    }
    .tag-standalone {
      background: #ecfdf5;
      color: #047857;
      border: 1px solid #a7f3d0;
      font-size: 11px;
      padding: 1px 6px;
      border-radius: 3px;
      margin-left: 6px;
    }
    main {
      padding: 8px 14px 24px;
      flex: 1;
    }
    .toolbar-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 8px;
      gap: 12px;
      flex-wrap: wrap;
    }
    .tabs-bar {
      display: flex;
      gap: 4px;
      border-bottom: 1px solid #e5e7eb;
      padding-bottom: 4px;
    }
    .tab-btn {
      background: none;
      border: 1px solid transparent;
      padding: 3px 10px;
      font-size: 12px;
      color: #4b5563;
      cursor: pointer;
      border-radius: 4px;
      transition: all 0.1s;
    }
    .tab-btn:hover {
      background: #f3f4f6;
      color: #111827;
    }
    .tab-btn.active {
      background: #111827;
      color: #ffffff;
      font-weight: 600;
    }
    .search-box {
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .filter-bar {
      display: flex;
      align-items: center;
      gap: 4px;
      flex-wrap: wrap;
      padding: 2px 0 6px;
      margin-bottom: 6px;
      border-bottom: 1px solid #f3f4f6;
    }
    .filter-bar .filter-label {
      font-size: 11px;
      color: #9ca3af;
      margin-right: 2px;
    }
    .filter-pill {
      background: #ffffff;
      border: 1px solid #e5e7eb;
      padding: 1px 8px;
      font-size: 11px;
      color: #4b5563;
      cursor: pointer;
      border-radius: 10px;
      line-height: 1.5;
      white-space: nowrap;
    }
    .filter-pill:hover {
      border-color: #cbd5e1;
      color: #111827;
    }
    .filter-pill.active {
      background: #eef2ff;
      border-color: #c7d2fe;
      color: #3730a3;
      font-weight: 600;
    }
    .filter-pill .n {
      color: #9ca3af;
      font-size: 10.5px;
      margin-left: 3px;
    }
    .filter-pill.active .n {
      color: #6366f1;
    }
    .search-input {
      border: 1px solid #d1d5db;
      border-radius: 3px;
      padding: 3px 8px;
      font-size: 12px;
      width: 200px;
      outline: none;
    }
    .search-input:focus {
      border-color: #2563eb;
    }
    .count-badge {
      font-size: 11.5px;
      color: #6b7280;
    }
    /* 服务聚合多列自适应网格 */
    .service-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
      gap: 8px;
    }
    .service-card {
      border: 1px solid #e5e7eb;
      border-radius: 4px;
      background: #ffffff;
      overflow: hidden;
      display: flex;
      flex-direction: column;
    }
    .service-card:hover {
      border-color: #cbd5e1;
    }
    .card-header {
      background: #f9fafb;
      border-bottom: 1px solid #f3f4f6;
      padding: 4px 8px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      min-height: 26px;
    }
    .card-title {
      display: flex;
      align-items: center;
      gap: 6px;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
      flex: 1;
    }
    .card-title .srv-name {
      font-weight: 600;
      font-size: 12px;
      color: #111827;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .card-title .srv-id {
      font-size: 11px;
      color: #9ca3af;
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    }
    .card-meta {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-shrink: 0;
    }
    .srv-host {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 11px;
      color: #4b5563;
      background: #f3f4f6;
      padding: 1px 4px;
      border-radius: 2px;
      cursor: pointer;
    }
    .srv-host:hover {
      background: #eff6ff;
      color: #1d4ed8;
    }
    .srv-count {
      font-size: 11px;
      color: #9ca3af;
    }
    .card-body {
      padding: 2px 4px;
    }
    .micro-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 11.5px;
    }
    .micro-table tr {
      border-bottom: 1px solid #f9fafb;
    }
    .micro-table tr:last-child {
      border-bottom: none;
    }
    .micro-table tr:hover {
      background: #f9fafb;
    }
    .micro-table td {
      padding: 3px 4px;
      vertical-align: middle;
      line-height: 1.25;
    }
    .col-role {
      width: 30%;
      color: #374151;
      font-weight: 500;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .col-account {
      width: 26%;
    }
    .col-secret {
      width: 24%;
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    }
    .col-action {
      width: 20%;
      text-align: right;
      white-space: nowrap;
    }
    .plain-code {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 11.5px;
      background: #f3f4f6;
      color: #374151;
      padding: 1px 4px;
      border-radius: 2px;
      cursor: pointer;
    }
    .plain-code:hover {
      background: #e5e7eb;
    }
    .plain-masked {
      color: #9ca3af;
      letter-spacing: 1px;
      cursor: pointer;
      user-select: none;
    }
    .plain-revealed {
      color: #b91c1c;
      font-weight: 600;
    }
    .action-btn {
      background: none;
      border: none;
      color: #2563eb;
      font-size: 11.5px;
      cursor: pointer;
      padding: 1px 3px;
      border-radius: 2px;
      line-height: 1.2;
    }
    .action-btn:hover {
      background: #eff6ff;
      color: #1d4ed8;
    }
    .action-btn.secondary {
      color: #6b7280;
    }
    .action-btn.secondary:hover {
      background: #f3f4f6;
      color: #374151;
    }
    /* 弱提示弹窗 (Toast) */
    #toast {
      position: fixed;
      bottom: 24px;
      left: 50%;
      transform: translateX(-50%);
      background: rgba(17, 24, 39, 0.9);
      color: #fff;
      padding: 6px 14px;
      border-radius: 4px;
      font-size: 12px;
      opacity: 0;
      pointer-events: none;
      transition: opacity 0.2s ease;
      z-index: 9999;
    }
    #toast.show {
      opacity: 1;
    }
    .env-badge {
      display: inline-block;
      font-size: 10px;
      padding: 0 4px;
      border-radius: 2px;
      background: #f3f4f6;
      color: #6b7280;
      margin-left: 4px;
    }
  </style>
</head>
<body>
  <header>
    <div class="header-left">
      <h1 class="header-title">__PROJECT_NAME__</h1>
      <span class="header-sub">/ 密码速查静态档案</span>
      <span class="tag-standalone">纯静态脱机可用</span>
    </div>
    <div class="header-meta">
      生成时间: __GEN_TIME__ · 凭据总数: __CRED_COUNT__ 条
    </div>
  </header>

  <main>
    <div class="toolbar-row">
      <div class="tabs-bar" id="tabsBar"></div>
      <div class="search-box">
        <span class="count-badge" id="statsCount"></span>
      </div>
    </div>

    <div class="filter-bar" id="filterBar" hidden></div>

    <div class="service-grid" id="serviceGrid"></div>
  </main>

  <div id="toast"></div>

  <script>
    const RAW_GROUPS = __RAW_GROUPS_JSON__;
    let activeEnv = 'all';
    let activeGroup = 'all';

    function showToast(msg) {
      const t = document.getElementById('toast');
      if (!t) return;
      t.textContent = msg;
      t.className = 'show';
      clearTimeout(t._timer);
      t._timer = setTimeout(() => { t.className = ''; }, 1600);
    }

    function copyText(text, label) {
      if (!text) return;
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(() => {
          showToast('已复制 ' + (label || '') + ': ' + text);
        }).catch(() => fallbackCopy(text, label));
      } else {
        fallbackCopy(text, label);
      }
    }

    function fallbackCopy(text, label) {
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand('copy');
        showToast('已复制 ' + (label || ''));
      } catch (e) {
        prompt('请手动复制:', text);
      }
      document.body.removeChild(ta);
    }

    const revealedMap = {};

    function toggleReveal(credId) {
      revealedMap[credId] = !revealedMap[credId];
      render();
    }

    function initTabs() {
      const tabsBar = document.getElementById('tabsBar');
      if (!tabsBar) return;
      const envs = new Set();
      (RAW_GROUPS || []).forEach(g => { if (g.env) envs.add(g.env); });

      const envLabels = {
        'prod': '生产环境 (Prod)',
        'test': '测试环境 (Test)',
        'c0130': '现场专区 (C0130)',
        'infra': '公共基础 (Infra)'
      };

      let html = '<button class="tab-btn active" data-env="all">全部环境</button>';
      const preferred = ['prod', 'test', 'c0130', 'infra'];
      preferred.forEach(e => {
        if (envs.has(e)) {
          html += '<button class="tab-btn" data-env="' + escapeHTML(e) + '">' + escapeHTML(envLabels[e] || e.toUpperCase()) + '</button>';
          envs.delete(e);
        }
      });
      envs.forEach(e => {
        html += '<button class="tab-btn" data-env="' + escapeHTML(e) + '">' + escapeHTML(e.toUpperCase()) + '</button>';
      });
      tabsBar.innerHTML = html;

      tabsBar.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
          tabsBar.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
          btn.classList.add('active');
          activeEnv = btn.getAttribute('data-env') || 'all';
          activeGroup = 'all';
          initGroupFilters();
          render();
        });
      });

      // 默认优先停留在「生产环境」
      const prodBtn = tabsBar.querySelector('.tab-btn[data-env="prod"]');
      if (prodBtn) {
        tabsBar.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
        prodBtn.classList.add('active');
        activeEnv = 'prod';
      }
    }

    // 当前环境下出现过的分组 → 服务实例数量
    function envScopedGroups() {
      const counts = {};
      const order = [];
      (RAW_GROUPS || []).forEach(g => {
        if (activeEnv !== 'all' && g.env !== activeEnv) return;
        const gs = (g.groups && g.groups.length) ? g.groups : ['未分组'];
        gs.forEach(name => {
          if (!(name in counts)) {
            counts[name] = 0;
            order.push(name);
          }
          counts[name]++;
        });
      });
      return { counts: counts, order: order };
    }

    function initGroupFilters() {
      const bar = document.getElementById('filterBar');
      if (!bar) return;
      const scoped = envScopedGroups();

      if (!scoped.order.length || (scoped.order.length === 1 && scoped.order[0] === '未分组')) {
        bar.hidden = true;
        bar.innerHTML = '';
        activeGroup = 'all';
        return;
      }

      bar.hidden = false;
      const total = Object.keys(scoped.counts).reduce((s, k) => s + scoped.counts[k], 0);
      let html = '<span class="filter-label">分组</span>';
      html += '<button class="filter-pill' + (activeGroup === 'all' ? ' active' : '') + '" data-group="all">全部<span class="n">' + total + '</span></button>';
      scoped.order.forEach(name => {
        html += '<button class="filter-pill' + (activeGroup === name ? ' active' : '') + '" data-group="' + escapeHTML(name) + '">' +
          escapeHTML(name) + '<span class="n">' + scoped.counts[name] + '</span></button>';
      });
      bar.innerHTML = html;

      bar.querySelectorAll('.filter-pill').forEach(pill => {
        pill.addEventListener('click', () => {
          activeGroup = pill.getAttribute('data-group') || 'all';
          bar.querySelectorAll('.filter-pill').forEach(p => p.classList.remove('active'));
          pill.classList.add('active');
          render();
        });
      });
    }

    function groupMatches(g) {
      if (activeGroup === 'all') return true;
      const gs = (g.groups && g.groups.length) ? g.groups : ['未分组'];
      return gs.indexOf(activeGroup) !== -1;
    }

    function render() {
      const grid = document.getElementById('serviceGrid');
      if (!grid) return;
      grid.innerHTML = '';

      let totalMatchingCreds = 0;
      let matchingGroupsCount = 0;

      (RAW_GROUPS || []).forEach(g => {
        // 环境过滤
        if (activeEnv !== 'all' && g.env !== activeEnv) {
          return;
        }

        // 分组过滤
        if (!groupMatches(g)) {
          return;
        }

        const matchedCreds = (g.credentials || []);
        if (matchedCreds.length === 0) return;

        matchingGroupsCount++;
        totalMatchingCreds += matchedCreds.length;

        const card = document.createElement('div');
        card.className = 'service-card';

        const srvId = g.id || '';
        const shortID = srvId.replace(/^services\/[^\/]+\/|^machine\/[^\/]+\/|^credentials\/[^\/]+\//, '');
        const hostPort = g.host ? (g.host + (g.port ? ':' + g.port : '')) : '';
        const envUpper = (g.env || '').toUpperCase();

        let headerHTML = '<div class="card-header">' +
          '<div class="card-title">' +
            '<span class="srv-name">' + escapeHTML(g.name || '未命名服务') + '</span>' +
            '<span class="srv-id" title="' + escapeHTML(srvId) + '">(' + escapeHTML(shortID || srvId) + ')</span>' +
            (activeEnv === 'all' && envUpper ? '<span class="env-badge">' + escapeHTML(envUpper) + '</span>' : '') +
          '</div>' +
          '<div class="card-meta">' +
            (hostPort ? '<span class="srv-host" title="点击复制地址" onclick="copyText(\'' + escapeJS(hostPort) + '\', \'地址\')">' + escapeHTML(hostPort) + '</span>' : '') +
            '<span class="srv-count">' + matchedCreds.length + ' 账号</span>' +
          '</div>' +
        '</div>';

        let rowsHTML = '<div class="card-body"><table class="micro-table"><tbody>';
        matchedCreds.forEach(c => {
          const isRevealed = !!revealedMap[c.id];
          const pw = c.password || '';
          const maskedOrPlain = isRevealed ?
            '<span class="plain-revealed">' + escapeHTML(pw || '(空)') + '</span>' :
            '<span class="plain-masked" title="点击复制密码" onclick="copyText(\'' + escapeJS(pw) + '\', \'密码\')">••••••••</span>';

          rowsHTML += '<tr>' +
            '<td class="col-role" title="' + escapeHTML(c.name || '') + '">' + escapeHTML(c.role || c.name || '默认') + '</td>' +
            '<td class="col-account"><span class="plain-code" title="点击复制账号" onclick="copyText(\'' + escapeJS(c.account || '默认') + '\', \'账号\')">' + escapeHTML(c.account || '默认') + '</span></td>' +
            '<td class="col-secret">' + maskedOrPlain + '</td>' +
            '<td class="col-action">' +
              '<button class="action-btn" onclick="copyText(\'' + escapeJS(pw) + '\', \'密码\')">复制</button>' +
              '<button class="action-btn secondary" onclick="toggleReveal(\'' + escapeJS(c.id || '') + '\')">' + (isRevealed ? '隐藏' : '查看') + '</button>' +
            '</td>' +
          '</tr>';
        });
        rowsHTML += '</tbody></table></div>';

        card.innerHTML = headerHTML + rowsHTML;
        grid.appendChild(card);
      });

      const statsElem = document.getElementById('statsCount');
      if (statsElem) {
        statsElem.textContent = matchingGroupsCount + ' 服务 · ' + totalMatchingCreds + ' 账号';
      }
      if (matchingGroupsCount === 0) {
        grid.innerHTML = '<div style="grid-column: 1/-1; text-align: center; color: #9ca3af; padding: 40px 0;">未检索到匹配的服务或凭据</div>';
      }
    }

    function escapeHTML(str) {
      if (str === null || str === undefined) return '';
      return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
    }

    function escapeJS(str) {
      if (str === null || str === undefined) return '';
      return String(str).replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/"/g, '\\"').replace(/\n/g, '\\n').replace(/\r/g, '');
    }

    // 事件绑定
    // 初始化渲染
    initTabs();
    initGroupFilters();
    render();
  </script>
</body>
</html>`

	res := strings.ReplaceAll(tmpl, "__PROJECT_NAME__", safeProjectName)
	res = strings.ReplaceAll(res, "__GEN_TIME__", genTime)
	res = strings.ReplaceAll(res, "__CRED_COUNT__", fmt.Sprintf("%d", len(credentials)))
	res = strings.ReplaceAll(res, "__RAW_GROUPS_JSON__", string(groupsJSON))
	return res
}
