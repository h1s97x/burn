package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type Settings struct {
	ListenAddr   string
	Port         int
	SlugLen      int
	OutputDir    string
	MaxBody      int64
	DefaultTTL   time.Duration
	MaxTTL       time.Duration
	WriteLimit   int
	ReadLimit    int
	RateWindow   time.Duration
	RealIPHeader string
	BasePath     string
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func parseArgs() Settings {
	fs := flag.NewFlagSet("burn", flag.ExitOnError)

	listenAddr := fs.String("L", envString("BURN_LISTEN_ADDR", "0.0.0.0"), "监听地址")
	port := fs.Int("p", envInt("BURN_PORT", 9999), "监听端口")
	slugLen := fs.Int("s", envInt("BURN_SLUG_LEN", 16), "slug 长度（决定链接的不可猜测性）")
	outputDir := fs.String("o", envString("BURN_OUTPUT_DIR", "data"), "秘密存储目录")
	maxBody := fs.Int64("B", envInt64("BURN_MAX_BODY", 65536), "单次上传上限（字节）")
	defaultTTL := fs.Duration("t", envDuration("BURN_DEFAULT_TTL", 24*time.Hour), "默认存活时间")
	maxTTL := fs.Duration("T", envDuration("BURN_MAX_TTL", 7*24*time.Hour), "允许申请的最长存活时间")
	writeLimit := fs.Int("w", envInt("BURN_WRITE_LIMIT", 30), "每 IP 每窗口上传次数上限（0 不限制）")
	readLimit := fs.Int("r", envInt("BURN_READ_LIMIT", 120), "每 IP 每窗口读取次数上限（0 不限制）")
	rateWindow := fs.Duration("W", envDuration("BURN_RATE_WINDOW", time.Minute), "限流时间窗口")
	realIPHeader := fs.String("H", envString("BURN_REAL_IP_HEADER", ""), "取真实 IP 的请求头，如 X-Forwarded-For")
	basePath := fs.String("P", envString("BURN_BASE_PATH", ""), "子路径部署，如 /burn")

	_ = fs.Parse(os.Args[1:])

	return Settings{
		ListenAddr:   *listenAddr,
		Port:         *port,
		SlugLen:      *slugLen,
		OutputDir:    *outputDir,
		MaxBody:      *maxBody,
		DefaultTTL:   *defaultTTL,
		MaxTTL:       *maxTTL,
		WriteLimit:   *writeLimit,
		ReadLimit:    *readLimit,
		RateWindow:   *rateWindow,
		RealIPHeader: *realIPHeader,
		BasePath:     *basePath,
	}
}

func main() {
	settings := parseArgs()

	if settings.SlugLen < 8 {
		log.Fatalf("[ERROR] slug 长度至少 8 位，否则链接可被枚举")
	}
	if settings.MaxTTL > 0 && settings.DefaultTTL > settings.MaxTTL {
		log.Fatalf("[ERROR] 默认 TTL 不能大于最大 TTL")
	}

	if err := os.MkdirAll(settings.OutputDir, 0700); err != nil {
		log.Fatalf("[ERROR] 无法创建存储目录: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storage := NewStorage(settings.OutputDir, settings.SlugLen, settings.DefaultTTL, settings.MaxTTL)
	writeLimiter := NewRateLimiter(settings.WriteLimit, settings.RateWindow)
	readLimiter := NewRateLimiter(settings.ReadLimit, settings.RateWindow)
	server := NewServer(settings, storage, writeLimiter, readLimiter)

	if n := storage.Sweep(time.Now()); n > 0 {
		log.Printf("[STATUS] 启动清扫：移除 %d 条过期秘密", n)
	}

	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if n := storage.Sweep(now); n > 0 {
					log.Printf("[STATUS] 定期清扫：移除 %d 条过期秘密", n)
				}
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Printf("[STATUS] 正在关闭...")
		cancel()
		server.Shutdown()
	}()

	log.Printf("[STATUS] 监听 %s:%d，目录 %s，默认 TTL %s", settings.ListenAddr, settings.Port, settings.OutputDir, settings.DefaultTTL)
	log.Println("============================================================")

	if err := server.Run(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[ERROR] %v", err)
	}
}
