package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "embed"
)

//go:embed index.html
var indexHTML string

//go:embed confirm.html
var confirmHTML string

type Server struct {
	settings Settings
	storage  *Storage
	writes   *RateLimiter
	reads    *RateLimiter
	httpSrv  *http.Server
}

func NewServer(settings Settings, storage *Storage, writes, reads *RateLimiter) *Server {
	return &Server{settings: settings, storage: storage, writes: writes, reads: reads}
}

func (s *Server) Run() error {
	handler := http.Handler(http.HandlerFunc(s.handleRoot))

	if s.settings.BasePath != "" {
		bp := s.settings.BasePath
		if !strings.HasPrefix(bp, "/") {
			bp = "/" + bp
		}
		bp = strings.TrimSuffix(bp, "/")
		handler = http.StripPrefix(bp, handler)
		s.settings.BasePath = bp
	}

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.settings.ListenAddr, s.settings.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown() {
	if s.httpSrv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.httpSrv.Shutdown(ctx)
}

// noStore 是所有响应共用的头：秘密不该进任何缓存，也不该被搜索引擎记住。
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	ip := s.peerIP(r)
	noStore(w)

	// 写和读分开计：一个 NAT 后面的人一起看链接，不该被上传限流连坐。
	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		if !s.writes.Allow(ip) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
	} else if !s.reads.Allow(ip) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "/")

	switch {
	case slug == "" && (r.Method == http.MethodPut || r.Method == http.MethodPost):
		s.handleUpload(w, r, ip)
	case slug == "" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		s.handleIndex(w, r)
	case slug != "" && !isValidSlug(slug):
		http.Error(w, "not found", http.StatusNotFound)
	case slug != "" && r.Method == http.MethodHead:
		s.handlePeek(w, slug)
	case slug != "" && r.Method == http.MethodGet:
		s.handleRead(w, r, slug)
	case slug != "" && r.Method == http.MethodPost:
		s.handleBurn(w, r, slug)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request, ip string) {
	body := http.MaxBytesReader(w, r.Body, s.settings.MaxBody)
	data, err := io.ReadAll(body)
	if err != nil {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(data) == 0 {
		http.Error(w, "no data received", http.StatusBadRequest)
		return
	}

	// TTL 可以按次覆盖：?ttl=10m。用裸时长，不引入额外 DSL。
	var ttl time.Duration
	if raw := r.URL.Query().Get("ttl"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			http.Error(w, "invalid ttl, use a duration like 30m or 24h", http.StatusBadRequest)
			return
		}
		ttl = d
	}

	slug, applied, err := s.storage.Save(data, ttl)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	url := s.baseURL(r) + "/" + slug

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Expires-In", strconv.FormatInt(int64(applied.Seconds()), 10))
	w.Header().Set("X-Expires-At", time.Now().Add(applied).UTC().Format(time.RFC3339))
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "%s\n", url)
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request, slug string) {
	// 关键取舍：浏览器打开链接时先给确认页，真正销毁必须显式 POST 或 curl。
	// 否则 IM / 邮件 / 任何会预览链接的爬虫都会替你烧掉这条秘密。
	if prefersHTML(r) {
		if err := s.storage.Peek(slug); err != nil {
			s.writeGone(w, err)
			return
		}
		s.writeConfirm(w, r, slug)
		return
	}

	data, err := s.storage.Claim(slug)
	if err != nil {
		s.writeGone(w, err)
		return
	}
	s.writeSecret(w, data)
}

func (s *Server) handleBurn(w http.ResponseWriter, r *http.Request, slug string) {
	// 浏览器确认页提交的是表单（带 Accept: text/html），curl -X POST 不带 → 回纯文本。
	wantsPage := prefersHTML(r) && strings.Contains(r.Header.Get("Content-Type"), "form")

	data, err := s.storage.Claim(slug)
	if err != nil {
		s.writeGone(w, err)
		return
	}

	if wantsPage {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Replace(secretPageHTML, "{{CONTENT}}", html.EscapeString(string(data)), 1)))
		return
	}
	s.writeSecret(w, data)
}

func (s *Server) handlePeek(w http.ResponseWriter, slug string) {
	if err := s.storage.Peek(slug); err != nil {
		s.writeGone(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) writeSecret(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) writeGone(w http.ResponseWriter, err error) {
	if errors.Is(err, errGone) {
		http.Error(w, "this secret has expired", http.StatusGone)
		return
	}
	http.Error(w, "not found (a burned secret is gone forever)", http.StatusNotFound)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.ReplaceAll(indexHTML, "{{BASE_URL}}", html.EscapeString(s.baseURL(r)))
	page = strings.ReplaceAll(page, "{{DEFAULT_TTL}}", humanDuration(s.settings.DefaultTTL))
	page = strings.ReplaceAll(page, "{{MAX_TTL}}", humanDuration(s.settings.MaxTTL))
	_, _ = w.Write([]byte(page))
}

func (s *Server) writeConfirm(w http.ResponseWriter, r *http.Request, slug string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.ReplaceAll(confirmHTML, "{{SLUG}}", html.EscapeString(slug))
	page = strings.ReplaceAll(page, "{{BASE_URL}}", html.EscapeString(s.baseURL(r)))
	_, _ = w.Write([]byte(page))
}

func (s *Server) baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + s.settings.BasePath
}

func (s *Server) peerIP(r *http.Request) string {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if s.settings.RealIPHeader != "" {
		if val := r.Header.Get(s.settings.RealIPHeader); val != "" {
			if strings.EqualFold(s.settings.RealIPHeader, "X-Forwarded-For") {
				val = strings.TrimSpace(strings.SplitN(val, ",", 2)[0])
			}
			ip = strings.TrimSpace(val)
		}
	}
	return ip
}

// prefersHTML 用 Accept 里有没有 text/html 判断来者是不是浏览器。
// curl 发的是 */*，SDK 客户端多半也是 */*，只有浏览器会主动要 HTML。
func prefersHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	return strings.Contains(accept, "text/html")
}

func humanDuration(d time.Duration) string {
	if d <= 0 {
		return "不限"
	}
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%d 天", int(d/(24*time.Hour)))
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%d 小时", int(d/time.Hour))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%d 分钟", int(d/time.Minute))
	}
	return fmt.Sprintf("%d 秒", int(d/time.Second))
}
