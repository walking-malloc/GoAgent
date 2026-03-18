# GoAgent RAG 流程说明（从入库到问答）

本文档描述本仓库 `GoAgent/` 的 **RAG（Retrieval-Augmented Generation）完整链路**：文档入库 → 分块与向量化 → 向量/关键词检索 → RRF 融合 → Prompt 组装 → LLM 生成 → 返回结果。

> 代码以 GoAgent 当前实现为准：检索侧 **仅做 RRF 融合**（已去掉重排），可按需扩展重排模块。

---

## 0. 流程图（Mermaid）

### 0.1 文档入库（Ingestion Pipeline）

```mermaid
flowchart TD
  A[用户上传文档\nPOST /api/v1/documents] --> B[DocumentHandler\n读取文件/识别类型/保存本地]
  B --> C[DocumentService.UploadDocument\n创建 Document(Pending)]
  C --> D[goroutine: processDocument\nDocument=Processing]

  D --> E[Tika 解析\nparser.GetTikaParser().Parse]
  E --> F[文本清洗\nsanitizeText / UTF-8 校验]
  F --> G[ChunkService.ChunkTextStreaming\n按段落 + overlap 分块]
  G --> H[MySQL: document_chunks\n批量 CreateBatch\nVectorStatus=Pending]

  H --> I[EmbeddingService\nDashScope Embedding]
  I --> J[Milvus: InsertVectors\nid/vector/doc_id/kb_id/content]
  J --> K[Document/Chunk 状态更新\nCompleted/Failed]

  subgraph 外部服务
    T[Tika Server]
    DS[DashScope Embedding API]
    MV[Milvus]
    DB[(MySQL)]
  end
  E -.调用.-> T
  I -.调用.-> DS
  J -.写入.-> MV
  H -.写入.-> DB
```

### 0.2 RAG 问答（Chat）

```mermaid
flowchart TD
  A[用户提问\nPOST /api/v1/chat] --> B[ChatHandler]
  B --> C[ChatService.Chat\n生成/透传 conversation_id]

  C --> D{QueryRewriteService?\n(可选)}
  D -->|是| D1[基于最近 Q/A 重写问题]
  D -->|否| E[进入选库]
  D1 --> E[进入选库]

  E --> F[IntentService.Classify\n自动选择 KBID]
  F --> G[KBRepo.FindByID\n获取 collectionName]

  G --> H{MemoryService?\n(可选)}
  H -->|是| H1[SearchRelevantMemory\n补充 memory contexts]
  H -->|否| I[RetrievalService.Retrieve]
  H1 --> I[RetrievalService.Retrieve]

  I --> I1[EmbeddingService.EmbedText\nquery 向量化]
  I1 --> I2[Milvus SearchVectors\n向量召回]
  I --> I3[MySQL SearchByKeyword\n关键词召回]
  I2 --> I4[RRF 融合 rrfMerge]
  I3 --> I4[RRF 融合 rrfMerge]
  I4 --> J[取 TopK chunks\n返回给 ChatService]

  J --> K[BuildRAGPrompt\n拼接 contexts]
  K --> L[LLMService.Chat\nDeepSeek /chat/completions]
  L --> M[返回 answer + contexts + source_docs]

  subgraph 外部服务
    DS[DashScope Embedding API]
    MV[Milvus]
    DB[(MySQL)]
    LLM[DeepSeek LLM API]
  end
  I1 -.调用.-> DS
  I2 -.查询.-> MV
  I3 -.查询.-> DB
  L -.调用.-> LLM
```

## 1. 核心组件（你需要知道的“谁负责什么”）

- **API 层**：`internal/api/handler/*`
  - 文档上传：`DocumentHandler.UploadDocument`
  - 问答接口：`ChatHandler.Chat`
- **Service 层**：`internal/service/*`
  - 入库流水线：`DocumentService`
  - 检索：`RetrievalService`
  - RAG 问答编排：`ChatService`
  -（可选增强）问题重写：`QueryRewriteService`
  -（可选增强）会话记忆：`ConversationMemoryService`
  -（可选增强）意图识别选库：`IntentService`
- **Repository 层**：`internal/repository/*`
  - 文档/分块/知识库/用户等 CRUD
- **外部/基础设施**
  - MySQL：文档、分块、知识库等结构化数据
  - Milvus：向量存储与向量检索
  - Redis：Token 黑名单、（可扩展缓存等）
  - Tika：文档解析（当前 `DocumentService` 直接依赖 Tika）
  - DashScope：Embedding
  - DeepSeek：LLM

---

## 2. 文档入库流程（Ingestion Pipeline）

### 2.1 入口：上传文档

- **接口**：`POST /api/v1/documents`（multipart/form-data）
- **Handler**：`internal/api/handler/document_handler.go`
  - 读取上传文件 → 推断文件类型 → 保存到本地 → 调用 `DocumentService.UploadDocument(...)`

### 2.2 保存与创建文档记录

- **Service**：`internal/service/document_service.go`
- **方法**：`UploadDocument(...)`
  - 校验知识库存在（KBID）
  - 创建 `Document` 记录，状态 `Pending`
  - 启动 goroutine 执行 `processDocument(docID, kb.CollectionName)`

### 2.3 解析（Parse）

- **方法**：`processDocument(...)`
  - 组装文件路径（上传目录 + 相对路径）
  - 使用 **Tika 解析器**：`parser.GetTikaParser().Parse(fullFilePath)`
  - 对解析出的文本做 UTF-8 清洗（`sanitizeText`）
  - 若为空则标记文档失败

### 2.4 分块（Chunking）

- **分块服务**：`internal/service/chunk_service.go`
- **特点**：
  - 按段落切分（空行分隔），段落内换行合并为空格
  - 段落过长：按固定 `chunkSize` + `chunkOverlap` 切分
  - 过滤过短段落（`minChunkSize`）
  - 流式回调 `ChunkTextStreaming(text, callback)`，避免一次性构造全部分块导致内存压力

### 2.5 分块落库（MySQL）

- 在 `processDocument(...)` 中：
  - 以批量方式 `CreateBatch` 写入 `document_chunks`
  - 每个 chunk 初始 `VectorStatus=Pending`

### 2.6 向量化（Embedding）并写入 Milvus

- **Embedding**：`internal/pkg/ai/embedding.go`
  - `EmbeddingService.EmbedTexts(texts []string)` 调 DashScope Embedding API
- **向量写入**：`internal/pkg/milvus/vector.go`
  - `VectorManager.InsertVectors(collectionName, vectors, ids, metadatas)`
  - 字段包含：
    - `id`（chunk 记录 ID）
    - `vector`（embedding 向量）
    - `chunk_id`（chunk index）
    - `doc_id` / `kb_id` / `content`

> 说明：Embedding 维度不匹配时会在日志提示你更新 `config.yaml` 的 `ai.embedding.dimension`。

---

## 3. 问答流程（RAG Chat）

### 3.1 入口：问答接口

- **接口**：`POST /api/v1/chat`
- **Handler**：`internal/api/handler/chat_handler.go`
  - 入参：`question`, `top_k`, `conversation_id`（KBID 字段目前 handler 入参有，但 service 侧走“意图识别选库”）
  - 调用：`ChatService.Chat(...)`

### 3.2 问题预处理（可选增强）

在 `internal/service/chat_service.go`：

- **会话 ID 生成**：如果没传 `conversation_id`，后端生成 ULID
- **问题重写（可选）**：`QueryRewriteService.RewriteQuestion(...)`
  - 使用最近若干轮 “Q/A” 对进行重写，让检索 query 更聚焦

### 3.3 选择知识库（Intent → KB）

- **意图识别**：`IntentService.Classify(question, topN, threshold)`
  - 选出最匹配的 KBID
- **查询知识库信息**：`KnowledgeBaseRepository.FindByID(kbID)`
  - 获取 `collectionName`（Milvus collection）

### 3.4 会话记忆检索（可选增强）

- `ConversationMemoryService.SearchRelevantMemory(...)`
  - 找出与当前问题相关的历史片段，作为额外上下文拼到 contexts 前面

### 3.5 检索（Retrieval）

核心在 `internal/service/retrieval_service.go`：

1. **向量召回**
   - `EmbeddingService.EmbedText(query)` 得到 query vector
   - `VectorManager.SearchVectors(collectionName, queryVector, topK, kbID)`
2. **关键词召回（MySQL LIKE）**
   - `DocumentChunkRepository.SearchByKeyword(kbID, query, kwTopK)`
3. **RRF 融合（Reciprocal Rank Fusion）**
   - `rrfMerge(vectorChunks, keywordChunks, maxCandidates)`
   - 输出候选列表后 **直接截断 TopK 返回**（当前实现不再做 rerank）

> 为什么要 RRF：当向量召回和关键词召回各自有优势时，RRF 用“排名”而不是“分数尺度”融合，能降低不同检索通道分数不可比的问题。

### 3.6 Prompt 组装并调用 LLM

- **Prompt**：`internal/pkg/ai/llm.go`
  - `BuildRAGPrompt(question, contexts)`：
    - system：约束“只基于文档内容回答，找不到就诚实说明”
    - user：拼接 contexts + 原始问题
- **LLM**：`LLMService.Chat(...)`
  - 调 DeepSeek `POST {base_url}/chat/completions`

### 3.7 返回结果

`ChatResponse`（service 层）包含：

- `answer`：模型输出
- `contexts`：参与回答的文档片段（含会话记忆 + 检索片段）
- `source_docs`：来源 doc_id 列表（去重）
- `conversation_id`：会话 ID

---

## 4. 运行与访问（常用）

- **服务启动入口**：`cmd/server/main.go`
- **前端静态页（演示）**：`GET /app/`（目录 `GoAgent/frontend/`）

---

## 5. 可配置项（和 RAG 直接相关）

见 `GoAgent/configs/config.yaml`（或 `config.example.yaml`）：

- **Embedding**
  - `ai.dashscope.api_key/base_url`
  - `ai.embedding.model/dimension`
- **LLM**
  - `ai.deepseek.api_key/base_url/model`
- **Milvus**
  - `milvus.host/port/...`
- **Tika**
  - `tika.enabled/host/port/timeout`

---

## 6. 常见扩展点（面试可讲）

- **重排（Rerank）**：在 RRF 后加入 Cross-Encoder / LLM rerank，并提供失败降级策略
- **引用溯源**：contexts 增加 chunk 元信息（文档名、页码、段落索引），回答中输出引用
- **增量更新与删除**：文档更新后重算 embedding、同步删除 Milvus 向量与 MySQL chunk
- **评测**：离线指标（Recall@K/MRR/nDCG）+ 线上反馈闭环

