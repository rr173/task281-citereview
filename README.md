# 判例引证适用范围复核台 (task281-citereview)

法律研究者复核判例引证的事实前提、适用范围与后续限制关系的后端服务。核心能力：
导入判决段与事实要素、解析限制语（地域/时间/主体/事项）、比较引证双方的前提集合以检测
"范围过宽"误引、记录区分裁决，并将研究图版本化（草稿→共享→冻结→替代）后发布。

## 业务闭环

1. 导入判例材料，切分为判决段（幂等摘要）。
2. 抽取事实要素，解析限制语。
3. 建立后案→原案的引证关系（拒绝自引与引证环）。
4. 范围检查：原案限制语未被后案采纳 → 标记"范围过宽"。
5. 研究者补充区分理由并裁决，冻结并发布研究图版本。

## 状态机

- 研究批次：整理中 → 待分析 → 待裁决 → 已发布 → 封存
- 判决段：待解析 → 有效 / 限定 / 排除
- 引证关系：候选 → 适用 / 范围过宽 / 区分 / 确认
- 研究图版本：草稿 → 共享 → 冻结 → 替代

## 标准命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/citereview --addr :8080 --db citereview.db   # 启动服务
go run ./cmd/citereview --smoke-test                     # 自检（不启动长驻服务）
```

## 模块责任

- `internal/material`：材料导入与判决段切分、幂等摘要。
- `internal/element`：事实要素抽取。
- `internal/scope`：限制语解析与适用范围比较。
- `internal/decision`：区分裁决与状态回写。
- `internal/graph`：研究图版本化、快照与聚合视图。
- `internal/store`：SQLite 持久化（建表迁移 + CRUD）。
- `internal/httpapi`：HTTP 层，路由前缀 `/api`。

## 持久化

SQLite（modernc.org/sqlite，CGO 无关，离线可构建）。保存段落、要素、限制语、引证边、
裁决与版本快照；重启后恢复未完成比较，冻结版本保留材料版本。组件版本见 `component-versions.json`。
