# BENZHI 评测说明

基于 Go 实现的判例引证适用范围复核 Web 项目，一款后端服务，完成判决段导入与限制语解析、引证适用范围冲突检测（范围过宽判定）与区分裁决、以及研究图版本冻结发布。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/citereview --addr :8080 --db citereview.db
```

## 自检（不启动长驻服务）

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/citereview --smoke-test
```

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
```

## HTTP API（/api 前缀，JSON）

- 批次：POST/GET `/api/batches`，GET `/api/batches/{id}`，POST `/api/batches/{id}/status`
- 材料：POST `/api/batches/{id}/import`，GET `/api/batches/{id}/segments`
- 判决段：GET `/api/segments/{id}`，POST `/api/segments/{id}/elements`，POST `/api/segments/{id}/limitations`，GET `/api/segments/{id}/elements`，GET `/api/segments/{id}/limitations`
- 引证：POST `/api/batches/{id}/citations`，GET `/api/batches/{id}/citations`，GET `/api/citations/{id}`，POST `/api/citations/{id}/check`，POST `/api/citations/{id}/decide`
- 版本：POST `/api/batches/{id}/versions`，GET `/api/batches/{id}/versions`，POST `/api/versions/{id}/freeze`，POST `/api/versions/{id}/share`，POST `/api/versions/{id}/supersede`
- 视图：GET `/api/batches/{id}/graph`
- 元数据：GET `/api/stats`，GET `/api/health`
- 复核页：启动后访问 `GET /`（传入 `?batch=<id>` 查看引证图）

## 持久化

使用 SQLite（modernc.org/sqlite，CGO 无关）持久化判决段、事实要素、限制语、引证关系、裁决与研究图版本；`--smoke-test` 关闭并重开同一数据库验证重启恢复。Docker 镜像说明见 `build_benzhi_docker.sh`。

## Docker 双架构

```bash
bash build_benzhi_docker.sh <镜像名> linux/amd64
bash build_benzhi_docker.sh <镜像名> linux/arm64
docker run --rm <镜像名> --smoke-test   # 判据：exit 0
```
