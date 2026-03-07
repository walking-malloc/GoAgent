# Ragent Go 版本

这是 Ragent 项目的 Golang 重构版本。

## 项目结构

```
golang/
├── cmd/
│   └── server/          # 应用入口
├── internal/
│   ├── api/            # API 层（Handler）
│   ├── service/        # 服务层
│   ├── repository/     # 数据访问层
│   ├── model/          # 数据模型
│   ├── config/         # 配置
│   └── pkg/           # 内部包
│       ├── milvus/     # Milvus 封装
│       ├── ai/         # AI API 封装
│       └── parser/     # 文档解析
├── pkg/                # 可复用的包
├── configs/            # 配置文件
├── scripts/            # 脚本
└── docs/               # 文档
```

## 快速开始

### 1. 安装依赖

```bash
go mod download
```

### 2. 配置

复制 `configs/config.example.yaml` 为 `configs/config.yaml` 并修改配置。

### 3. 运行

```bash
go run cmd/server/main.go
```

## 开发计划

详见 `重构计划.md`
