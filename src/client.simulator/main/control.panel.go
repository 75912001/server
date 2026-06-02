package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

type ControlPanelConfig struct {
	Enable bool   `yaml:"enable"`
	Addr   string `yaml:"addr"`
}

type ControlPanel struct {
	manager   *RobotManager
	server    *http.Server
	closeOnce sync.Once
}

type controlPanelOverview struct {
	Stats       RobotStatsSnapshot `json:"stats"`
	StatsText   string             `json:"statsText"`
	Total       int                `json:"total"`
	Robots      []robotView        `json:"robots"`
	Gateways    []gatewayView      `json:"gateways"`
	APIMessages []apiView          `json:"apiMessages"`
}

type robotView struct {
	UID         uint64 `json:"uid"`
	GatewayAddr string `json:"gatewayAddr"`
	Connected   bool   `json:"connected"`
	Verified    bool   `json:"verified"`
	UserReady   bool   `json:"userReady"`
	NextSession uint32 `json:"nextSession"`
	Seq         uint64 `json:"seq"`
	Pending     int    `json:"pending"`
}

type gatewayView struct {
	Key  string `json:"key"`
	Addr string `json:"addr"`
}

type apiView struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type controlPanelSendReq struct {
	Scope   string `json:"scope"`
	UID     uint64 `json:"uid"`
	Message string `json:"message"`
}

type controlPanelSendRes struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Queued  int    `json:"queued"`
}

func normalizeControlPanelConfig(cfg *ConfigYaml) {
	if cfg.ControlPanel.Addr == "" {
		cfg.ControlPanel.Addr = "127.0.0.1:18080"
	}
}

func StartControlPanel(ctx context.Context, manager *RobotManager) (*ControlPanel, error) {
	if !GConfigYaml.ControlPanel.Enable {
		return nil, nil
	}
	panel := &ControlPanel{manager: manager}
	mux := http.NewServeMux()
	mux.HandleFunc("/", panel.handleIndex)
	mux.HandleFunc("/api/overview", panel.handleOverview)
	mux.HandleFunc("/api/send", panel.handleSend)
	server := &http.Server{
		Addr:              GConfigYaml.ControlPanel.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, err
	}
	panel.server = server
	go func() {
		<-ctx.Done()
		panel.Stop()
	}()
	go func() {
		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ColorPrintf(Red, "control panel stopped: %v\n", err)
			log.Errorf("control panel stopped: %v", err)
		}
	}()
	ColorPrintf(Cyan, "control panel: http://%s/\n", server.Addr)
	log.Infof("control panel started addr=%s", server.Addr)
	return panel, nil
}

func (p *ControlPanel) Stop() {
	p.closeOnce.Do(func() {
		if p.server == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.server.Shutdown(ctx)
	})
}

func (p *ControlPanel) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(controlPanelHTML))
}

func (p *ControlPanel) handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	overview := controlPanelOverview{
		Stats:       p.manager.stats.Snapshot(),
		Total:       p.manager.Total(),
		Robots:      p.manager.RobotViews(200),
		Gateways:    discoveredGatewayViews(),
		APIMessages: loadAPIViews(),
	}
	overview.StatsText = overview.Stats.String(overview.Total)
	writeJSON(w, overview)
}

func (p *ControlPanel) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req controlPanelSendReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, controlPanelSendRes{OK: false, Message: err.Error()})
		return
	}
	switch req.Scope {
	case "all":
		queued := p.manager.QueueAllCommand(req.Message)
		writeJSON(w, controlPanelSendRes{OK: true, Message: "queued all " + req.Message, Queued: queued})
	case "uid":
		if err := p.manager.QueueUIDCommand(req.UID, req.Message); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, controlPanelSendRes{OK: false, Message: err.Error()})
			return
		}
		writeJSON(w, controlPanelSendRes{OK: true, Message: "queued uid " + strconv.FormatUint(req.UID, 10) + " " + req.Message, Queued: 1})
	default:
		writeJSONStatus(w, http.StatusBadRequest, controlPanelSendRes{OK: false, Message: "invalid scope"})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func discoveredGatewayViews() []gatewayView {
	discoveredGatewayMu.Lock()
	defer discoveredGatewayMu.Unlock()
	out := make([]gatewayView, 0, len(discoveredGatewayMap))
	for key, addr := range discoveredGatewayMap {
		out = append(out, gatewayView{Key: key, Addr: addr})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

func loadAPIViews() []apiView {
	data, err := loadAPI(apiYamlPath)
	if err != nil {
		return nil
	}
	out := make([]apiView, 0, len(data))
	for name, api := range data {
		out = append(out, apiView{Name: name, ID: api.ID})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

const controlPanelHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>client.simulator 控制面板</title>
<style>
:root{color-scheme:light;--bg:#f6f7f9;--panel:#fff;--line:#d8dde6;--text:#172033;--muted:#687386;--accent:#0f766e;--warn:#b45309;--bad:#b91c1c}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px/1.45 "Segoe UI",Arial,"Microsoft YaHei",sans-serif}
header{height:56px;display:flex;align-items:center;justify-content:space-between;padding:0 24px;border-bottom:1px solid var(--line);background:#fff}
h1{font-size:18px;margin:0;font-weight:650}main{padding:18px 24px;display:grid;gap:16px}
.grid{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:10px}.tile{background:var(--panel);border:1px solid var(--line);border-radius:6px;padding:12px}
.tile span{display:block;color:var(--muted);font-size:12px}.tile strong{display:block;margin-top:6px;font-size:22px}
.bar{display:flex;gap:10px;align-items:center;flex-wrap:wrap;background:var(--panel);border:1px solid var(--line);border-radius:6px;padding:12px}
select,input,button{height:34px;border:1px solid var(--line);border-radius:6px;background:#fff;color:var(--text);padding:0 10px}
button{cursor:pointer;background:var(--accent);color:#fff;border-color:var(--accent);font-weight:600}button.secondary{background:#fff;color:var(--text);border-color:var(--line)}
table{width:100%;border-collapse:collapse;background:var(--panel);border:1px solid var(--line);border-radius:6px;overflow:hidden}
th,td{padding:9px 10px;border-bottom:1px solid var(--line);text-align:left;white-space:nowrap}th{font-size:12px;color:var(--muted);background:#fafbfc}tr:last-child td{border-bottom:0}
.ok{color:var(--accent);font-weight:600}.no{color:var(--bad);font-weight:600}.warn{color:var(--warn);font-weight:600}
.split{display:grid;grid-template-columns:2fr 1fr;gap:16px}.log{min-height:34px;color:var(--muted)}@media(max-width:900px){.grid{grid-template-columns:repeat(2,1fr)}.split{grid-template-columns:1fr}}
</style>
</head>
<body>
<header><h1>client.simulator 控制面板</h1><div id="clock"></div></header>
<main>
  <section class="grid">
    <div class="tile"><span>总数</span><strong id="total">0</strong></div>
    <div class="tile"><span>在线</span><strong id="active">0</strong></div>
    <div class="tile"><span>登录成功</span><strong id="verifyOK">0</strong></div>
    <div class="tile"><span>发送</span><strong id="sent">0</strong></div>
    <div class="tile"><span>接收</span><strong id="received">0</strong></div>
    <div class="tile"><span>失败</span><strong id="failed">0</strong></div>
  </section>
  <section class="bar">
    <select id="message"></select>
    <button onclick="sendAll()">all</button>
    <input id="uid" placeholder="uid" inputmode="numeric">
    <button onclick="sendUID()">uid</button>
    <button class="secondary" onclick="refresh()">刷新</button>
    <div class="log" id="log"></div>
  </section>
  <section class="split">
    <div>
      <table>
        <thead><tr><th>UID</th><th>连接</th><th>登录</th><th>用户数据</th><th>Gateway</th><th>Session</th><th>Seq</th><th>队列</th></tr></thead>
        <tbody id="robots"></tbody>
      </table>
    </div>
    <div>
      <table>
        <thead><tr><th>Gateway</th><th>地址</th></tr></thead>
        <tbody id="gateways"></tbody>
      </table>
    </div>
  </section>
</main>
<script>
let apiMessages=[];
function text(id,v){document.getElementById(id).textContent=v}
function status(v){return v?'<span class="ok">是</span>':'<span class="no">否</span>'}
async function refresh(){
  const res=await fetch('/api/overview');
  const data=await res.json();
  const s=data.stats;
  const active=Math.max(0,(s.ConnectOK||0)-(s.Disconnect||0));
  text('total',data.total||0); text('active',active); text('verifyOK',s.VerifyOK||0); text('sent',s.Sent||0); text('received',s.Received||0); text('failed',(s.SendFail||0)+(s.ResultFail||0)+(s.CommandError||0));
  text('clock',new Date().toLocaleTimeString());
  apiMessages=data.apiMessages||[];
  const select=document.getElementById('message');
  const current=select.value;
  select.innerHTML=apiMessages.map(x=>` + "`" + `<option value="${x.name}">${x.name} ${x.id}</option>` + "`" + `).join('');
  if(current) select.value=current;
  document.getElementById('robots').innerHTML=(data.robots||[]).map(r=>` + "`" + `<tr><td>${r.uid}</td><td>${status(r.connected)}</td><td>${status(r.verified)}</td><td>${status(r.userReady)}</td><td>${r.gatewayAddr||''}</td><td>${r.nextSession}</td><td>${r.seq}</td><td>${r.pending}</td></tr>` + "`" + `).join('');
  document.getElementById('gateways').innerHTML=(data.gateways||[]).map(g=>` + "`" + `<tr><td>${g.key}</td><td>${g.addr}</td></tr>` + "`" + `).join('');
}
async function send(scope,uid){
  const message=document.getElementById('message').value;
  const res=await fetch('/api/send',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({scope,uid,message})});
  const data=await res.json();
  document.getElementById('log').textContent=data.message||'';
  await refresh();
}
function sendAll(){send('all',0)}
function sendUID(){send('uid',Number(document.getElementById('uid').value||0))}
refresh(); setInterval(refresh,2000);
</script>
</body>
</html>`
