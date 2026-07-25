# tob-ratelimit

面向 ToB 的 Go 分层限流库：分布式令牌桶、Redis Lua 原子操作、本地降级与 HTTP 中间件。

## 用途与 Target 判定

服务端限流库（`net/http` 中间件 + Redis），非消费级 App。

- 移动端：不适用
- 智能硬件：不适用

## 目标平台

| 平台 | 需要? | 方式 |
|------|-------|------|
| 移动端 | 不适用 | — |
| 智能硬件 | 不适用 | — |

## 技术栈信号

- 检测到：go

## 命令

```bash
cd active/tob-ratelimit
GOPROXY=https://goproxy.cn,direct go test ./...
go run ./examples/basic   # 需本地 Redis
```

## 助手约束

- Loop：先定可验证 Done 与停止条件；每切片过验证环；失败回灌；详见 `docs/LOOP_ENGINEERING.md`
- Harness：遵循根 `AGENTS.md`「Harness Engineering」；Guide=本文件，Sensor=`go test ./...`；同类失误二次须沉淀规则或测试（棘轮）。详见 `docs/HARNESS_ENGINEERING.md`
- 分层：`Types → Config → Repo → Service → Runtime → UI`；`internal/` 不得反向依赖上层；对外 API 仅经 `pkg/ratelimit`
- 改动保持聚焦；禁止静默失败

## 验证环 / Done

```bash
GOPROXY=https://goproxy.cn,direct go test ./...
```

**Done 示例**：`go test ./...` 全绿；示例 `examples/basic` 在 Redis 可用时可启动。
