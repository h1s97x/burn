package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	alphabet    = "abcdefghijklmnopqrstuvwxyz0123456789"
	dataName    = "index.txt"
	expiresName = "expires"
	burningDir  = ".burning"
)

var (
	errNotFound = errors.New("not found")
	errGone     = errors.New("gone")
)

// Storage 把一条秘密存成 <dir>/<slug>/index.txt，副作用只有一次读取。
// 销毁靠 os.Rename 的原子性：谁能把目录搬到 .burning 里，谁就是唯一的读者。
type Storage struct {
	dir        string
	slugLen    int
	defaultTTL time.Duration
	maxTTL     time.Duration
}

func NewStorage(dir string, slugLen int, defaultTTL, maxTTL time.Duration) *Storage {
	return &Storage{dir: dir, slugLen: slugLen, defaultTTL: defaultTTL, maxTTL: maxTTL}
}

// Save 写入一条秘密，返回 slug 与实际生效的 TTL。
func (s *Storage) Save(data []byte, ttl time.Duration) (string, time.Duration, error) {
	ttl = s.clampTTL(ttl)
	expires := time.Now().Add(ttl)

	for length := s.slugLen; length <= s.slugLen+8; length++ {
		for attempt := 0; attempt < 64; attempt++ {
			slug := generateSlug(length)
			dir := filepath.Join(s.dir, slug)

			// os.Mkdir 独占创建，slug 撞车时才拿得到 IsExist；MkdirAll 对已存在目录返回 nil，
			// 用它判断会和上一条秘密写进同一个目录。
			err := os.Mkdir(dir, 0700)
			if err != nil {
				if os.IsExist(err) {
					continue
				}
				return "", 0, fmt.Errorf("create dir: %w", err)
			}

			if err := writeFileAtomic(dir, dataName, data); err != nil {
				os.RemoveAll(dir)
				return "", 0, err
			}
			if err := writeFileAtomic(dir, expiresName, []byte(strconv.FormatInt(expires.Unix(), 10))); err != nil {
				os.RemoveAll(dir)
				return "", 0, err
			}
			return slug, ttl, nil
		}
	}
	return "", 0, fmt.Errorf("failed to generate a unique slug")
}

// Claim 认领并销毁一条秘密。并发请求里只有一个能成功 rename，其余都拿 errNotFound。
func (s *Storage) Claim(slug string) ([]byte, error) {
	src := filepath.Join(s.dir, slug)
	burnDir := filepath.Join(s.dir, burningDir)
	if err := os.MkdirAll(burnDir, 0700); err != nil {
		return nil, fmt.Errorf("prepare burn dir: %w", err)
	}

	dst := filepath.Join(burnDir, slug+"-"+generateSlug(8))
	if err := os.Rename(src, dst); err != nil {
		if os.IsNotExist(err) {
			return nil, errNotFound
		}
		return nil, fmt.Errorf("claim: %w", err)
	}
	// 认领即销毁：先 rename 再读，宁可读到一半连接断了丢内容，也不允许被读第二次。
	defer os.RemoveAll(dst)

	expired, err := isExpired(dst, s.defaultTTL)
	if err != nil {
		return nil, errGone
	}
	if expired {
		return nil, errGone
	}

	data, err := os.ReadFile(filepath.Join(dst, dataName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errNotFound
		}
		return nil, fmt.Errorf("read: %w", err)
	}
	return data, nil
}

// Peek 只看是否存在，不销毁。给 HEAD 和浏览器的确认页用。
func (s *Storage) Peek(slug string) error {
	dir := filepath.Join(s.dir, slug)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return errNotFound
	}
	expired, err := isExpired(dir, s.defaultTTL)
	if err != nil {
		return errNotFound
	}
	if expired {
		return errGone
	}
	return nil
}

// Sweep 清掉过期条目和崩溃残留的 .burning 目录，返回删除的条目数。
func (s *Storage) Sweep(now time.Time) int {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		if e.Name() == burningDir {
			removeStaleChildren(path, now.Add(-time.Hour))
			continue
		}
		expired, err := isExpired(path, s.defaultTTL)
		if err != nil {
			continue
		}
		if expired {
			if os.RemoveAll(path) == nil {
				removed++
			}
		}
	}
	return removed
}

func (s *Storage) clampTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return s.defaultTTL
	}
	if s.maxTTL > 0 && ttl > s.maxTTL {
		return s.maxTTL
	}
	return ttl
}

// isExpired 无 expires 文件时退化成「目录 mtime + ttl」，避免残留条目永生。
func isExpired(dir string, fallbackTTL time.Duration) (bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, expiresName))
	if err != nil {
		if os.IsNotExist(err) {
			info, statErr := os.Stat(dir)
			if statErr != nil {
				return true, statErr
			}
			return time.Now().After(info.ModTime().Add(fallbackTTL)), nil
		}
		return false, err
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return true, nil
	}
	return time.Now().After(time.Unix(ts, 0)), nil
}

func removeStaleChildren(dir string, before time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().Before(before) {
			os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

func writeFileAtomic(dir, name string, data []byte) error {
	tmp := filepath.Join(dir, name+".tmp")
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", name, err)
	}
	return nil
}

func generateSlug(length int) string {
	const maxByte = 255 - (256 % len(alphabet))
	out := make([]byte, length)
	for i := 0; i < length; {
		var b [1]byte
		if _, err := rand.Read(b[:]); err != nil {
			continue
		}
		if int(b[0]) > maxByte {
			continue
		}
		out[i] = alphabet[int(b[0])%len(alphabet)]
		i++
	}
	return string(out)
}

func isValidSlug(slug string) bool {
	if slug == "" || len(slug) > 64 {
		return false
	}
	for _, c := range slug {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
