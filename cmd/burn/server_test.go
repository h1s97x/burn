package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestServer(t *testing.T, limit int) (*Server, *Storage, string) {
	t.Helper()
	dir := t.TempDir()
	st := NewStorage(dir, 12, time.Hour, 24*time.Hour)
	settings := Settings{
		SlugLen:    12,
		OutputDir:  dir,
		MaxBody:    4096,
		DefaultTTL: time.Hour,
		MaxTTL:     24 * time.Hour,
		WriteLimit: limit,
		ReadLimit:  limit,
		RateWindow: time.Minute,
	}
	return NewServer(settings, st, NewRateLimiter(limit, time.Minute), NewRateLimiter(limit, time.Minute)), st, dir
}

func upload(t *testing.T, s *Server, body, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/"+query, strings.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	s.handleRoot(w, req)
	return w
}

func TestUploadThenBurn(t *testing.T) {
	s, _, _ := newTestServer(t, 0)

	w := upload(t, s, "s3cret", "")
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: got %d, want 201 (%s)", w.Code, w.Body.String())
	}
	loc := strings.TrimSpace(w.Body.String())
	if !strings.HasSuffix(loc, "/"+loc[strings.LastIndex(loc, "/")+1:]) {
		t.Fatalf("bad url %q", loc)
	}
	slug := loc[strings.LastIndex(loc, "/")+1:]

	// curl 风格：Accept 不含 text/html，直接读走。
	req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
	req.Header.Set("Accept", "*/*")
	w = httptest.NewRecorder()
	s.handleRoot(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "s3cret" {
		t.Fatalf("read: got %d %q", w.Code, w.Body.String())
	}

	// 第二次必须 404 —— 这就是全部的意义。
	w = httptest.NewRecorder()
	s.handleRoot(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("second read: got %d, want 404", w.Code)
	}
}

func TestBrowserGetsConfirmPageWithoutBurning(t *testing.T) {
	s, _, _ := newTestServer(t, 0)
	w := upload(t, s, "topsecret", "")
	slug := strings.TrimSpace(w.Body.String())
	slug = slug[strings.LastIndex(slug, "/")+1:]

	browserReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		rec := httptest.NewRecorder()
		s.handleRoot(rec, req)
		return rec
	}

	// GET 两次都只给确认页，秘密还在。
	for i := 0; i < 2; i++ {
		rec := browserReq()
		if rec.Code != http.StatusOK {
			t.Fatalf("confirm page: got %d", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "topsecret") {
			t.Fatal("confirm page must not leak the secret")
		}
		if !strings.Contains(rec.Body.String(), `method="post"`) {
			t.Fatal("confirm page should post back")
		}
	}

	// 提交表单才真正销毁。
	form := httptest.NewRequest(http.MethodPost, "/"+slug, strings.NewReader(""))
	form.Header.Set("Accept", "text/html")
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleRoot(rec, form)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "topsecret") {
		t.Fatalf("form claim: got %d", rec.Code)
	}
	if again := browserReq(); again.Code != http.StatusNotFound {
		t.Fatalf("after claim: got %d, want 404", again.Code)
	}
}

func TestConcurrentClaimOnlyOneWins(t *testing.T) {
	s, _, _ := newTestServer(t, 0)
	w := upload(t, s, "race", "")
	slug := strings.TrimSpace(w.Body.String())
	slug = slug[strings.LastIndex(slug, "/")+1:]

	const n = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
			req.Header.Set("Accept", "text/plain")
			rec := httptest.NewRecorder()
			s.handleRoot(rec, req)
			if rec.Code == http.StatusOK {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if ok != 1 {
		t.Fatalf("expected exactly 1 successful claim, got %d", ok)
	}
}

func TestHeadPeekDoesNotBurn(t *testing.T) {
	s, _, _ := newTestServer(t, 0)
	w := upload(t, s, "peek", "")
	slug := strings.TrimSpace(w.Body.String())
	slug = slug[strings.LastIndex(slug, "/")+1:]

	req := httptest.NewRequest(http.MethodHead, "/"+slug, nil)
	w = httptest.NewRecorder()
	s.handleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("peek: got %d", w.Code)
	}

	// 内容仍然可读。
	req = httptest.NewRequest(http.MethodGet, "/"+slug, nil)
	req.Header.Set("Accept", "text/plain")
	w = httptest.NewRecorder()
	s.handleRoot(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "peek" {
		t.Fatalf("read after peek: got %d %q", w.Code, w.Body.String())
	}
}

func TestTTLFromQueryAndClamp(t *testing.T) {
	s, st, _ := newTestServer(t, 0)

	w := upload(t, s, "short", "?ttl=90s")
	if w.Code != http.StatusCreated {
		t.Fatalf("ttl upload: got %d", w.Code)
	}
	if got := w.Header().Get("X-Expires-In"); got != "90" {
		t.Fatalf("X-Expires-In = %q, want 90", got)
	}

	// 超过上限被夹到 MaxTTL(24h)。
	w = upload(t, s, "long", "?ttl=1000h")
	if got := w.Header().Get("X-Expires-In"); got != "86400" {
		t.Fatalf("clamped X-Expires-In = %q, want 86400", got)
	}

	// 非法 ttl 直接 400，不猜。
	if w := upload(t, s, "bad", "?ttl=forever"); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid ttl: got %d, want 400", w.Code)
	}

	_ = st
}

func TestExpiredSecretIsGone(t *testing.T) {
	s, st, dir := newTestServer(t, 0)
	slug, _, err := st.Save([]byte("old"), 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
	req.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	s.handleRoot(w, req)
	if w.Code != http.StatusGone {
		t.Fatalf("expired read: got %d, want 410", w.Code)
	}

	// 读一次就删掉了，不再占磁盘。
	if _, err := os.Stat(filepath.Join(dir, slug)); !os.IsNotExist(err) {
		t.Fatalf("expired entry should be removed after read, stat err = %v", err)
	}
}

func TestSweepRemovesExpiredAndLeavesLive(t *testing.T) {
	_, st, dir := newTestServer(t, 0)

	dead, _, err := st.Save([]byte("dead"), 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	live, _, err := st.Save([]byte("live"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)

	if n := st.Sweep(time.Now()); n != 1 {
		t.Fatalf("sweep removed %d, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(dir, dead)); !os.IsNotExist(err) {
		t.Fatal("expired entry survived sweep")
	}
	if _, err := os.Stat(filepath.Join(dir, live)); err != nil {
		t.Fatal("live entry was removed")
	}
}

func TestRateLimitZeroMeansUnlimited(t *testing.T) {
	s, _, _ := newTestServer(t, 0)
	for i := 0; i < 200; i++ {
		if w := upload(t, s, "x", ""); w.Code != http.StatusCreated {
			t.Fatalf("request %d: got %d, want 201 with limit=0", i, w.Code)
		}
	}
}

func TestRateLimitRejectsBurst(t *testing.T) {
	s, _, _ := newTestServer(t, 3)
	codes := make([]int, 0, 5)
	for i := 0; i < 5; i++ {
		codes = append(codes, upload(t, s, "x", "").Code)
	}
	for i, c := range codes {
		want := http.StatusCreated
		if i >= 3 {
			want = http.StatusTooManyRequests
		}
		if c != want {
			t.Fatalf("request %d: got %d, want %d (all: %v)", i, c, want, codes)
		}
	}
}

func TestSlugNotReusedOnCollision(t *testing.T) {
	// 同内容连续上传两次必须得到两条不同的链接，否则一次读取会连带销毁另一条。
	s, _, _ := newTestServer(t, 0)
	first := strings.TrimSpace(upload(t, s, "same", "").Body.String())
	second := strings.TrimSpace(upload(t, s, "same", "").Body.String())
	if first == second {
		t.Fatalf("same content got the same url %q", first)
	}
}

func TestNoStoreHeaders(t *testing.T) {
	s, _, _ := newTestServer(t, 0)
	w := upload(t, s, "h", "")
	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := w.Header().Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
		t.Fatalf("X-Robots-Tag = %q", got)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	s, _, _ := newTestServer(t, 0)
	for _, p := range []string{"/../etc/passwd", "/Upper", "/a%2Fb"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req.Header.Set("Accept", "text/plain")
		w := httptest.NewRecorder()
		s.handleRoot(w, req)
		if w.Code == http.StatusOK {
			t.Fatalf("%q was served", p)
		}
	}
}

func TestEmptyUploadRejected(t *testing.T) {
	s, _, _ := newTestServer(t, 0)
	if w := upload(t, s, "", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("empty upload: got %d, want 400", w.Code)
	}
}

func TestOversizeUploadRejected(t *testing.T) {
	s, _, _ := newTestServer(t, 0)
	big := strings.Repeat("a", 8192)
	if w := upload(t, s, big, ""); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize upload: got %d, want 413", w.Code)
	}
}
