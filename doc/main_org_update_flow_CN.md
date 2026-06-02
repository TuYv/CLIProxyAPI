# `main_org` 更新到 `main` 流程

> 适用场景：`CLIProxyAPI_org` 是上游源码仓，`CLIProxyAPI` 是带本地新功能的仓；先把上游源码同步到 `CLIProxyAPI/main_org`，再把 `main_org` 合并到 `CLIProxyAPI/main`。

## 仓库与分支约定

| 名称 | 说明 |
| --- | --- |
| `CLIProxyAPI_org/main` | 上游源码主分支 |
| `CLIProxyAPI/main_org` | 本地仓中用于跟踪上游源码的分支 |
| `CLIProxyAPI/main` | 本地新功能主分支，需要保留本地功能并吸收上游更新 |
| `refs/remotes/source/main` | 临时拉取上游源码后在本地仓中的引用 |

## 1. 合并前检查

先确认两个仓库路径、分支和工作区状态，避免在有未提交改动时开始合并。

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI status --short --branch
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI branch --all --verbose --no-abbrev

git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI_org status --short --branch
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI_org branch --all --verbose --no-abbrev
```

建议同时确认 `main_org` 是否是上游 `main` 的祖先；如果是，可以优先使用快进合并。

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI_org merge-base --is-ancestor <CLIProxyAPI/main_org旧提交> main
```

## 2. 把上游源码同步到 `main_org`

在本地新功能仓 `CLIProxyAPI` 内拉取上游源码仓 `CLIProxyAPI_org/main`，并快进 `main_org`。

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI fetch \
  /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI_org \
  main:refs/remotes/source/main

git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI checkout main_org
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI merge --ff-only refs/remotes/source/main
```

同步后检查：

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI status --short --branch
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI rev-parse main_org refs/remotes/source/main
```

两者提交应一致。

## 3. 把 `main_org` 合并到 `main`

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI checkout main
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI merge main_org
```

如果出现冲突，进入冲突解决流程。

## 4. 冲突解决原则

### 4.1 总原则

1. 保留上游源码的新能力。
2. 保留本地 `main` 已新增的功能。
3. 对同一结构体、同一日志/用量链路中的字段，不二选一，优先合并字段。
4. 解决冲突后不要只依赖肉眼检查，必须执行冲突标记检查和 `git diff --check`。

### 4.2 本次合并中需要重点保留的本地功能

- 账号级 API Key。
- 账号用量元数据：`AccountID`、`AccountName`、`APIKeyID`、`APIKeyName`。
- 用量事件持久化：SQLite usage event store。
- 用量队列中账号维度统计与事件记录。

### 4.3 本次合并中需要重点保留的上游功能

- `reasoning_effort` / `service_tier` 用量记录。
- TTFT / latency 相关用量字段。
- Home 控制面、集群订阅、日志转发、证书等新增能力。
- Codex、xAI、Antigravity、signature、translator 等上游更新。

### 4.4 常见冲突处理策略

#### `sdk/cliproxy/usage/manager.go`

`Record` 需要同时保留本地账号字段和上游字段：

```go
Provider    string
Model       string
Alias       string
APIKey      string
AccountID   string
AccountName string
APIKeyID    string
APIKeyName  string
AuthID      string
AuthIndex   string
AuthType    string
Source      string
ReasoningEffort string
ServiceTier     string
ResponseHeaders http.Header
```

异步入队前需要克隆 `ResponseHeaders`，避免调用方后续修改 header map 影响队列消费者。

#### `internal/runtime/executor/helps/usage_helpers.go`

需要同时保留：

- 账号元数据注入：从 `sdk/access.Result` 中读取账号信息。
- 上游新增的 `reasoning_effort`、`service_tier`、TTFT、response headers 逻辑。

注意：`SetTranslatedReasoningEffort()` 不应在 payload 缺少 `service_tier` 时覆盖上下文中的既有值。

#### `internal/redisqueue/plugin.go`

队列 payload 需要同时包含：

- provider/model/alias/endpoint/auth_type/api_key/request_id
- account_id/account_name/api_key_id/api_key_name
- reasoning_effort/service_tier
- response_headers

#### `cmd/server/main.go`

- 保留用量 SQLite store 初始化。
- Home 配置模式下不要无条件覆盖 Home 下发的端口；仅当 `parsed.Port == 0` 时设置默认端口。

#### `internal/home/client.go`

- 订阅 `config` 和 `cluster` 两个频道。
- 空闲超时不应直接判定断链；应先尝试 `pubsub.Ping()`。
- 单次 keepalive 失败不应直接 failover，应复用连续失败阈值逻辑。

#### OAuth 会话

完成 OAuth 登录时只完成当前 `state`：

```go
CompleteOAuthSession(state)
```

不要对同一 provider 下所有 pending session 做批量完成，否则并发登录会互相影响。

#### Home 请求日志转发

推送到 Home 前必须脱敏 headers，至少覆盖：

- `Authorization`
- `api-key` / `apikey`
- `token`
- `secret`
- `cookie`

#### zstd 请求体日志

请求日志中如果解压 `Content-Encoding: zstd`，必须限制解压后的最大字节数，避免小压缩包导致内存放大。

## 5. 冲突标记与空白检查

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI grep -n '<<<<<<<\|>>>>>>>' || true
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI diff --check
```

要求：

- 不得存在 `<<<<<<<` 或 `>>>>>>>`。
- `git diff --check` 不应输出错误。

## 6. 格式化、构建与测试

正常环境下应执行：

```bash
cd /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI

gofmt -w <本次手工解决冲突和新增测试涉及的 Go 文件>
go test ./...
go build -o test-output ./cmd/server && rm test-output
```

如果当前环境没有 `go` / `gofmt`，需要如实记录：

```text
gofmt 未执行：command not found: gofmt
go test ./... 未执行：command not found: go
```

## 7. 代码审查

合并冲突解决后，至少做一次 Go 代码审查和一次通用代码审查，重点关注：

- 是否错误丢弃任一分支功能。
- 是否残留冲突标记。
- 用量统计字段是否完整。
- 异步队列是否持有稳定快照。
- Home 订阅是否会因正常空闲频繁重连。
- OAuth 并发登录是否互相影响。
- 日志转发是否泄露敏感 header。
- 压缩请求体日志是否有内存放大风险。

## 8. 提交合并结果

确认冲突解决、检查通过、审查高风险问题已处理后再提交。

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI add <变更文件>
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI commit \
  -m "chore: 合并 main_org 到 main" \
  -m "合并上游 main_org 更新，并保留账号 API Key 与用量统计相关功能。" \
  -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

提交后检查：

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI status --short --branch
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI log --oneline --decorate --graph -5
```

## 9. 推送

除非明确要求，不自动推送。需要推送时再执行：

```bash
git -C /Users/zz/my/data/code/githubtool/cc-trans/CLIProxyAPI push origin main
```

## 10. 本次实际结果记录模板

```text
main_org 同步到：<上游最新提交>
main 合并提交：<merge commit>
工作区状态：clean / not clean
格式化：已执行 / 未执行，原因：<原因>
测试：已执行 / 未执行，原因：<原因>
构建：已执行 / 未执行，原因：<原因>
是否推送：是 / 否
遗留风险：<如 Home 证书引导协议安全性需后续确认>
```
