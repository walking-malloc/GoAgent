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
