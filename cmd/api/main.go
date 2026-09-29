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
	"syscall"
	"time"
)

type App struct {
	db        *sql.DB
	config    Config
	dummyHash string
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
	mux.HandleFunc("GET /health", wrap(a.health))
	mux.HandleFunc("POST /api/auth/login", wrap(a.login))
	mux.HandleFunc("POST /api/auth/logout", wrap(a.logout))
	mux.HandleFunc("GET /api/me", wrap(a.me))
	mux.HandleFunc("GET /api/page-auth", wrap(a.pageAuth))
	mux.HandleFunc("GET /api/classes/{classId}/materials", wrap(a.list))
	mux.HandleFunc("POST /api/classes/{classId}/materials", wrap(a.upload))
	mux.HandleFunc("GET /api/classes/{classId}/materials/{id}", wrap(a.detail))
	mux.HandleFunc("GET /api/classes/{classId}/materials/{id}/file", wrap(a.download))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
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
	a := App{db: db, config: cfg, dummyHash: dummy}
	s := http.Server{Addr: cfg.Listen, Handler: a.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	}()
	log.Print("CampusClaw Go API 已启动")
	if e = s.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal("API 启动失败")
	}
}
