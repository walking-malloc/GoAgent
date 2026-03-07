-- Active: 1772879589376@@127.0.0.1@3307@ragent
-- 初始化数据库脚本
-- 如果数据库不存在则创建
CREATE DATABASE IF NOT EXISTS `ragent`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE `ragent`;

-- 创建用户表
CREATE TABLE IF NOT EXISTS `t_user`
(
    `id`          char(26)     NOT NULL COMMENT '主键ID（ULID）',
    `username`    varchar(64)  NOT NULL COMMENT '用户名，唯一',
    `password`    varchar(128) NOT NULL COMMENT '密码',
    `role`        varchar(32)  NOT NULL COMMENT '角色：admin/user',
    `avatar`      varchar(128) DEFAULT NULL COMMENT '用户头像',
    `create_time` datetime     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time` datetime     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`  datetime     DEFAULT NULL COMMENT '删除时间（软删除）',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_username` (`username`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

-- 插入默认管理员用户（用户名：admin，密码：admin123）
-- 注意：ID 使用 ULID，这里使用固定值用于初始化（ULID 格式：26个字符，Base32 编码）
-- 实际创建时，GORM 的 BeforeCreate hook 会自动生成 ULID
-- 这里使用固定的 ULID 值：01HZEXM5QZ8K9N3V4R6S7T8U9W0（示例值，实际应该使用 ulid.Make() 生成）
INSERT INTO `t_user` (`id`, `username`, `password`, `role`, `avatar`, `create_time`, `update_time`, `deleted_at`)
VALUES ('01HZEXM5QZ8K9N3V4R6S7T8U9W', 'admin', 'admin123', 'admin', NULL, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE `update_time` = NOW();

-- 插入默认普通用户（用户名：user，密码：user123）
INSERT INTO `t_user` (`id`, `username`, `password`, `role`, `avatar`, `create_time`, `update_time`, `deleted_at`)
VALUES ('01HZEXM5QZ8K9N3V4R6S7T8U9X', 'user', 'user123', 'user', NULL, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE `update_time` = NOW();

-- 创建知识库表
CREATE TABLE IF NOT EXISTS `t_knowledge_base`
(
    `id`             char(26)     NOT NULL COMMENT '主键ID（ULID）',
    `name`           varchar(128) NOT NULL COMMENT '知识库名称',
    `embedding_model` varchar(64)  DEFAULT NULL COMMENT '嵌入模型标识，如：qwen3-embedding:8b-fp16',
    `collection_name` varchar(128) NOT NULL COMMENT 'Milvus Collection 名称（创建后禁止修改）',
    `created_by`     varchar(64)  DEFAULT NULL COMMENT '创建人',
    `updated_by`     varchar(64)  DEFAULT NULL COMMENT '修改人',
    `create_time`    datetime     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`    datetime     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`     datetime     DEFAULT NULL COMMENT '删除时间（软删除）',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_collection_name` (`collection_name`, `deleted_at`),
    KEY `idx_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识库表';

-- 创建文档表
CREATE TABLE IF NOT EXISTS `t_document`
(
    `id`             char(26)     NOT NULL COMMENT '主键ID（ULID）',
    `kb_id`          char(26)     NOT NULL COMMENT '知识库ID',
    `name`           varchar(255) NOT NULL COMMENT '文档名称',
    `file_name`      varchar(255) NOT NULL COMMENT '原始文件名',
    `file_path`      varchar(512) NOT NULL COMMENT '文件存储路径',
    `file_type`      varchar(32)  NOT NULL COMMENT '文件类型：pdf/docx/md/txt等',
    `file_size`      bigint       NOT NULL COMMENT '文件大小（字节）',
    `status`         varchar(32)  NOT NULL DEFAULT 'pending' COMMENT '状态：pending/processing/completed/failed',
    `chunk_count`    int          DEFAULT 0 COMMENT '分块数量',
    `error_message`  text         DEFAULT NULL COMMENT '错误信息',
    `created_by`     varchar(64)  DEFAULT NULL COMMENT '创建人',
    `create_time`    datetime     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`    datetime     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`     datetime     DEFAULT NULL COMMENT '删除时间（软删除）',
    PRIMARY KEY (`id`),
    KEY `idx_kb_id` (`kb_id`),
    KEY `idx_status` (`status`),
    KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文档表';

-- 创建文档分块表
CREATE TABLE IF NOT EXISTS `t_document_chunk`
(
    `id`             char(26)     NOT NULL COMMENT '主键ID（ULID）',
    `doc_id`         char(26)     NOT NULL COMMENT '文档ID',
    `kb_id`          char(26)     NOT NULL COMMENT '知识库ID',
    `chunk_index`    int          NOT NULL COMMENT '分块序号（从0开始）',
    `content`        text         NOT NULL COMMENT '分块内容',
    `content_length` int          NOT NULL COMMENT '内容长度（字符数）',
    `vector_status`  varchar(32)  NOT NULL DEFAULT 'pending' COMMENT '向量化状态：pending/completed/failed',
    `create_time`    datetime     DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`    datetime     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`     datetime     DEFAULT NULL COMMENT '删除时间（软删除）',
    PRIMARY KEY (`id`),
    KEY `idx_doc_id` (`doc_id`),
    KEY `idx_kb_id` (`kb_id`),
    KEY `idx_vector_status` (`vector_status`),
    UNIQUE KEY `uk_doc_chunk` (`doc_id`, `chunk_index`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文档分块表';