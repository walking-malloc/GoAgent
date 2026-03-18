# GoAgent（RAG 问答服务）

GoAgent 是一个基于 Go 实现的 **RAG（Retrieval-Augmented Generation）知识库问答服务**，包含：

- 文档入库：上传 → 解析 → 分块 → 向量化 → 写入 Milvus（并将分块内容写入 MySQL）
- 问答：问题重写（可选，基于最近对话）→ 意图识别自动选库 → 向量召回 + 关键词召回 → RRF 融合 → 组装 Prompt → 调用大模型生成答案

> 更详细的流程说明见：`docs/RAG_FLOW.md`。

---

## 目录结构（高频关注）

- `cmd/server/`：服务启动入口
- `internal/api/handler/`：HTTP API Handler
- `internal/service/`：核心业务（Chat / Retrieval / Ingestion / Intent / Memory / Rewrite）
- `internal/repository/`：MySQL 数据访问（文档、分块、知识库等）
- `internal/pkg/ai/`：Embedding / LLM /（可选）Rerank 封装
- `internal/pkg/milvus/`：Milvus 向量写入与检索
- `configs/`：配置文件
- `frontend/`：演示前端（Vite）

---

## 核心流程（对齐当前代码实现）

### 1）文档入库（Ingestion）

1. 用户上传文档（HTTP 接口）
2. 解析为纯文本（如 PDF/Word → text）
3. 分块：按段落切分；段落过长再按固定长度 + overlap 切分
4. 分块写入 MySQL：`document_chunks`（`vector_status=Pending`）
5. 对分块做 Embedding
6. 向量 + 元信息写入 Milvus collection

### 2）问答（Chat / RAG）

1. 接收用户问题 `question`（可带 `conversation_id`、`top_k`）
2. 若未传 `conversation_id`，后端生成一个（用于串联多轮）
3. **（可选）问题重写**：取该会话最近 N 轮 Q/A 文本，生成更适合检索的“检索问题”
4. **意图识别选库**：用“检索问题”自动选择 KB，并获取其对应 Milvus collectionName
5. **（可选）会话记忆**：从记忆中检索若干相关片段，作为额外 contexts
6. 检索（两路召回 + 融合）：
   - 向量召回：Embedding(query) → Milvus Search
   - 关键词召回：MySQL `content LIKE %query%`
   - RRF 融合：按排名融合两路结果，取 TopK chunks
7. 组 Prompt 并调用 LLM 生成答案
   - 注意：检索用“检索问题（可能被重写）”，但回答时仍使用“用户原始问题”以贴近用户问法
8. 写回：将本轮 Q/A 写入会话记忆（若启用），并追加到进程内的会话历史（供下一轮重写用）

---

## 环境依赖

你需要以下依赖（按实际部署可选）：

- **MySQL**：存储知识库、文档、分块等结构化数据
- **Milvus**：向量存储与向量检索
- **Embedding 服务**：当前实现对接 DashScope（用于 query / chunk 向量化）
- **LLM 服务**：当前实现对接 DeepSeek（用于生成回答）
- **Tika（可选/推荐）**：用于文档解析（PDF/Word 等）

---

## 配置

配置文件见：`configs/config.yaml`（以该文件内容为准）。通常你需要关注：

- **Embedding**：API Key / BaseURL / Model / Dimension
- **LLM**：API Key / BaseURL / Model
- **MySQL**：连接信息
- **Milvus**：连接信息与 collection 相关配置
- **Tika**：地址与超时时间

> 如果出现 embedding 维度不匹配，优先检查 `ai.embedding.dimension` 与实际模型输出是否一致。

---

## 启动方式

### 方式 A：Docker Compose（推荐）

仓库内提供 `docker-compose.yml`，用于一键拉起依赖与服务（以文件内容为准）。

在 `GoAgent/` 目录下执行：

```bash
docker compose up -d
```

如需查看日志：

```bash
docker compose logs -f
```

### 方式 B：本地运行（开发）

在 `GoAgent/` 目录下：

```bash
go mod download
go run ./cmd/server
```

> 本地运行前请确保：MySQL/Milvus/Tika/Embedding/LLM 等外部依赖在配置中可访问。

---

## 常用接口（概览）

> 具体路由与参数以 `internal/api/handler/` 为准。

- **上传文档**：`POST /api/v1/documents`
  - 用于将文档写入指定知识库并触发分块/向量化/入库
- **问答**：`POST /api/v1/chat`
  - 入参：`question`、`top_k`、`conversation_id`（可选）
  - 出参：`answer`、`contexts`、`source_docs`、`conversation_id`

---

## 前端（演示）

`frontend/` 为演示前端（Vite）。是否启用、如何访问以你当前部署方式为准（常见是后端提供静态页或独立启动前端）。

---

## 常见问题（Troubleshooting）

- **检索结果为空**
  - 确认文档已入库完成（chunk 已写入 MySQL、向量已写入 Milvus）
  - 确认意图识别是否选到了正确知识库
  - 确认 Milvus collectionName 与知识库绑定正确
- **Embedding 维度报错**
  - 检查配置中的 embedding dimension 与实际模型维度一致
- **文档解析失败/为空**
  - 检查 Tika 是否可用、超时配置是否合理

---

## 参考/笔记

- 飞书笔记：`https://my.feishu.cn/wiki/PfOIwq33RiccTjkKAODc3EaMnuh?from=from_copylink`
