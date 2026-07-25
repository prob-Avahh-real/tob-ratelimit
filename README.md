# tob-ratelimit

面向 ToB 场景的 Go 分层限流库：分布式令牌桶 + Redis Lua 原子操作 + 本地降级，兼顾突发流量与真实用户体验。

## 分层策略

请求按以下顺序逐层校验，**任一层拒绝即拦截**：

```
IP 黑名单 / 爬虫 → 端点专用限流 → 匿名设备 → 登录用户（高配额 + 加权补桶）
                              ↘ 批量请求检测（滑动窗口加价）
```

| 层级 | 对象 | 默认配额 | 说明 |
|------|------|----------|------|
| **Block** | 恶意 IP | 0（硬拒绝 403） | `BlockedIPs` 配置 |
| **Crawler** | 爬虫 IP | 1 req/min | `CrawlerIPs` 配置 |
| **Endpoint** | 敏感路由 | 如登录 5/min/IP | 防 credential stuffing |
| **Anonymous** | 未登录设备 | 10/min，burst 5 | 按 DeviceID 或 IP |
| **User** | 登录用户 | 1000/min，burst 200 | `Weight=1.5` 优先补桶 |

Redis Key 格式：`rl:{tier}:{identity}:{endpoint}`

## 核心能力

1. **分布式令牌桶**（`internal/bucket`）：Lua 脚本保证 refill + consume 原子性；`Weight > 1` 为登录用户提供优先补桶。
2. **分层解析**（`internal/layer`）：根据 IP 信誉、认证状态、设备指纹、端点解析适用规则。
3. **批量检测**（`internal/detector`）：滑动窗口识别同一身份的高频连续调用，动态提高 token 消耗。
4. **HTTP 中间件**（`internal/middleware`）：标准响应头 `X-RateLimit-Limit`、`X-RateLimit-Remaining`、`Retry-After`。
5. **Redis 降级**：Redis 不可用时自动切换本地内存桶，配额按 `LocalFallbackScale`（默认 50%）收紧。

## 快速开始

```go
import (
    goredis "github.com/redis/go-redis/v9"
    "github.com/skat/tob-ratelimit/internal/middleware"
    "github.com/skat/tob-ratelimit/pkg/ratelimit"
)

cfg := ratelimit.DefaultConfig()
cfg.BlockedIPs["203.0.113.99"] = struct{}{}

client := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
limiter := ratelimit.New(cfg, client)

mux := http.NewServeMux()
mux.HandleFunc("/api/data", handler)

http.ListenAndServe(":8080", middleware.Middleware(limiter)(mux))
```

## 目录结构

```
tob-ratelimit/
  internal/bucket/     # 令牌桶 + Redis Lua + 本地降级
  internal/layer/      # 分层规则解析
  internal/detector/   # 批量/高频请求检测
  internal/middleware/ # net/http 中间件
  pkg/ratelimit/       # 对外 API
  examples/basic/      # 可运行示例
```

## 测试

```bash
go test ./...
```

## 设计取舍

- 仅依赖 `net/http` 与 `go-redis`，无重型 Web 框架。
- 登录用户 `Capacity > Rate`（按分钟换算），允许 SPA/API 合理突发。
- 匿名与爬虫配额收紧，批量滥用通过滑动窗口加价而非直接封禁，减少误杀。
