package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type App struct {
	db        *sql.DB
	config    Config
	dummyHash string
	models    ModelConfig
}
type httpError struct {
	status  int
	message string
}

func (e httpError) Error() string           { return e.message }
func fail(status int, message string) error { return httpError{status, message} }
func send(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}
func wrap(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if e := f(w, r); e != nil {
			var h httpError
			if errors.As(e, &h) {
				if h.status == 401 {
					w.Header().Set("WWW-Authenticate", "Bearer")
				}
				_ = send(w, h.status, map[string]any{"error": map[string]string{"message": h.message}})
			} else {
				log.Print("请求处理失败，请检查服务或数据库状态。")
				_ = send(w, 503, map[string]any{"error": map[string]string{"message": "服务暂时不可用，请稍后重试。"}})
			}
		}
	}
}
func (a *App) handler() http.Handler {
	mux := http.NewServeMux()
	// @API GET /openapi.json
	mux.HandleFunc("GET /openapi.json", wrap(a.openAPIHandler))
	// @API GET /health
	mux.HandleFunc("GET /health", wrap(a.health))
	// @API POST /api/auth/login
	mux.HandleFunc("POST /api/auth/login", wrap(a.login))
	// @API POST /api/auth/logout
	mux.HandleFunc("POST /api/auth/logout", wrap(a.logout))
	// @API GET /api/me
	mux.HandleFunc("GET /api/me", wrap(a.me))
	// @API GET /api/classes/{classId}/materials
	mux.HandleFunc("GET /api/classes/{classId}/materials", wrap(a.list))
	// @API POST /api/classes/{classId}/materials
	mux.HandleFunc("POST /api/classes/{classId}/materials", wrap(a.upload))
	// @API GET /api/classes/{classId}/materials/{id}
	mux.HandleFunc("GET /api/classes/{classId}/materials/{id}", wrap(a.detail))
	// @API GET /api/classes/{classId}/materials/{id}/file
	mux.HandleFunc("GET /api/classes/{classId}/materials/{id}/file", wrap(a.download))
	// @API POST /api/retrieval/search
	mux.HandleFunc("POST /api/retrieval/search", wrap(a.searchHandler))
	// @API POST /api/ask
	mux.HandleFunc("POST /api/ask", wrap(a.ask))
	// @API GET /api/retrieval/chunks/{id}
	mux.HandleFunc("GET /api/retrieval/chunks/{id}", wrap(a.chunkDetail))
	// @API POST /api/materials/{id}/reindex
	mux.HandleFunc("POST /api/materials/{id}/reindex", wrap(a.reindex))
	// @API GET /api/prompt
	mux.HandleFunc("GET /api/prompt", wrap(a.promptHandler))
	// @API PUT /api/prompt
	mux.HandleFunc("PUT /api/prompt", wrap(a.promptHandler))
	// @API GET /api/skills
	mux.HandleFunc("GET /api/skills", wrap(a.skillsHandler))
	// @API POST /api/skills
	mux.HandleFunc("POST /api/skills", wrap(a.skillsHandler))
	// @API GET /api/skills/{id}
	mux.HandleFunc("GET /api/skills/{id}", wrap(a.skillHandler))
	// @API PUT /api/skills/{id}
	mux.HandleFunc("PUT /api/skills/{id}", wrap(a.skillHandler))
	// @API DELETE /api/skills/{id}
	mux.HandleFunc("DELETE /api/skills/{id}", wrap(a.skillHandler))
	// @API GET /api/agents
	mux.HandleFunc("GET /api/agents", wrap(a.agentsHandler))
	// @API POST /api/agents
	mux.HandleFunc("POST /api/agents", wrap(a.agentsHandler))
	// @API GET /api/agents/{id}
	mux.HandleFunc("GET /api/agents/{id}", wrap(a.agentHandler))
	// @API PUT /api/agents/{id}
	mux.HandleFunc("PUT /api/agents/{id}", wrap(a.agentHandler))
	// @API GET /api/conversations
	mux.HandleFunc("GET /api/conversations", wrap(a.conversationsHandler))
	// @API POST /api/conversations
	mux.HandleFunc("POST /api/conversations", wrap(a.conversationsHandler))
	// @API GET /api/conversations/{id}
	mux.HandleFunc("GET /api/conversations/{id}", wrap(a.conversationDetail))
	// @API POST /api/conversations/{id}/messages
	mux.HandleFunc("POST /api/conversations/{id}/messages", wrap(a.conversationMessages))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		timeout := 80 * time.Second
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/conversations/") && strings.HasSuffix(r.URL.Path, "/messages") {
			timeout = 120 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		c := http.Client{Timeout: 3 * time.Second}
		r, e := c.Get("http://127.0.0.1:8080/health")
		if e != nil {
			os.Exit(1)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	cfg, e := loadConfig()
	if e != nil {
		log.Fatal(e)
	}
	db, e := openDB(cfg)
	if e != nil {
		log.Fatal("数据库配置无效")
	}
	defer db.Close()
	if e = migrate(db); e != nil {
		log.Fatal("数据库迁移失败，请检查连接与结构")
	}
	if e = seed(db); e != nil {
		log.Fatal("种子初始化失败，请检查种子密码及数据库")
	}
	if e = seedGuidance(db); e != nil {
		log.Fatal("解题配置初始化失败")
	}
	if e = seedAgents(db); e != nil {
		log.Fatal("智能体样本初始化失败")
	}
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		log.Print("数据库结构及样本已就绪")
		return
	}
	if e = os.MkdirAll(cfg.UploadDir, 0700); e != nil {
		log.Fatal("无法创建上传临时目录")
	}
	dummy, e := passwordHash(token())
	if e != nil {
		log.Fatal("密码模块初始化失败")
	}
	a := App{db: db, config: cfg, dummyHash: dummy, models: modelConfig()}
	if _, e = db.Exec("INSERT IGNORE INTO material_indexes(material_id,options,processed_text) SELECT material_id,JSON_OBJECT('strategy','auto'),'' FROM material_contents"); e != nil {
		log.Fatal("索引任务初始化失败")
	}
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go a.indexWorker(workerCtx)
	s := http.Server{Addr: cfg.Listen, Handler: a.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 85 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		stopWorker()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	}()
	log.Print("CampusClaw Go API 已启动")
	if e = s.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal("API 启动失败")
	}
}
