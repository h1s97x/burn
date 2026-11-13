# burn

一次性秘密分享。`echo "密码" | curl -T - https://host` 得到一条链接，被读走一次就永远消失。

用来发 SSH 私钥、一次性 token、数据库密码这类东西。

## 用法

上传：

```bash
echo "密码" | curl -T - https://burn.example.com
```

返回：

```
https://burn.example.com/k3f9x2vq0w7m4n8z
```

读取（读走即销毁，第二次 404）：

```bash
curl https://burn.example.com/k3f9x2vq0w7m4n8z
```

浏览器打开会先看到一个确认按钮，点了才真正销毁。原因是**聊天软件和邮件客户端会自动预览链接** —— 如果不加这一步，你把链接贴进群里，对方还没点，机器人已经把秘密烧掉了。

### 存活时间

默认 24 小时，可以按次指定，上限由服务端配置控制：

```bash
cat ~/.ssh/id_ed25519 | curl -T - "https://burn.example.com/?ttl=10m"
```

`ttl` 用 Go 的时长写法：`30s`、`10m`、`2h`、`7d` 不支持（用 `168h`）。返回值里带两个响应头：

```
X-Expires-In: 600
X-Expires-At: 2026-11-13T09:12:08Z
```

### alias

```bash
alias burn='curl -T - https://burn.example.com'
echo "token" | burn
```

Windows PowerShell：

```powershell
function burn { $input | curl -T - https://burn.example.com }
```

## 运行

```bash
docker run -d \
  --name burn \
  -p 9999:9999 \
  -v ./data:/data \
  --restart unless-stopped \
  docker.cnb.cool/h1s97x/burn:latest
```

所有参数都能用环境变量给，优先级 **命令行 > 环境变量 > 默认值**。

| 参数 | 环境变量 | 默认值 | 说明 |
|------|----------|--------|------|
| `-L` | `BURN_LISTEN_ADDR` | `0.0.0.0` | 监听地址 |
| `-p` | `BURN_PORT` | `9999` | 监听端口 |
| `-s` | `BURN_SLUG_LEN` | `16` | slug 长度，低于 8 直接拒绝启动 |
| `-o` | `BURN_OUTPUT_DIR` | `data` | 存储目录 |
| `-B` | `BURN_MAX_BODY` | `65536` | 单次上传上限（字节） |
| `-t` | `BURN_DEFAULT_TTL` | `24h` | 默认存活时间 |
| `-T` | `BURN_MAX_TTL` | `168h` | 允许申请的最长存活时间 |
| `-w` | `BURN_WRITE_LIMIT` | `30` | 每 IP 每窗口上传次数（0 不限制） |
| `-r` | `BURN_READ_LIMIT` | `120` | 每 IP 每窗口读取次数（0 不限制） |
| `-W` | `BURN_RATE_WINDOW` | `1m` | 限流窗口 |
| `-H` | `BURN_REAL_IP_HEADER` | | 取真实 IP 的请求头，反代后面填 `X-Forwarded-For` |
| `-P` | `BURN_BASE_PATH` | | 子路径部署，如 `/burn` |

## 它怎么做到的

存储就是文件系统：一条秘密 = 一个目录 `data/<slug>/`，里面放 `index.txt` 和 `expires`。

销毁不用标志位，用 `rename`：

```
data/<slug>/  →  data/.burning/<slug>-<random>/
```

`os.Rename` 在同一个文件系统上是原子的，所以**并发读只有一个请求能成功搬到 `.burning`**，其余全部 404 —— 不需要分布式锁，也不会读到一半被另一个人读走。搬到 `.burning` 之后立刻读内容，`defer` 删目录；即使进程在读的时候崩了，重启时的清扫会把 `.burning` 里超过一小时的残留清掉。

这也是为什么 slug 不能复用：同内容两次上传得到两条独立链接，一次读取不会连带毁掉另一条。

## 边界

- **不做加密**：内容是明文落盘的，保护来自 `0700` 权限、一次读取、尽快删除。如果你的威胁模型包含「服务器磁盘被人拿走」，这个工具不适用，去看 age/sops。
- **不记录访问日志**：谁读过不记录，日志里只有上传和清扫的计数。
- **不做账号、不做管理接口**：没有列表、没有删除。链接即凭据。
- **只保证一次**：不做「N 次后销毁」，那需要额外的计数状态，会把原子 rename 的简洁性弄没。
- **反代要自己配**：TLS 由前置 nginx/caddy 负责，服务本身只跑 HTTP。

## 测试

```bash
go test ./...
```

覆盖了并发认领只有一个成功、过期 410、限流 0 值语义、路径穿越、TTL 夹取、浏览器确认页不泄露内容等边界。

## 许可证

[MIT](LICENSE)
