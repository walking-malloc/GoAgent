-- Active: 1772879589376@@127.0.0.1@3307@ragent
-- 修复字符集问题
-- 确保数据库使用 utf8mb4
ALTER DATABASE `ragent` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE `ragent`;

-- 修复文档分块表的字符集
ALTER TABLE `t_document_chunk` 
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 确保 content 列使用正确的字符集
ALTER TABLE `t_document_chunk` 
  MODIFY COLUMN `content` TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '分块内容';

-- 修复文档表的字符集
ALTER TABLE `t_document` 
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 确保 error_message 列使用正确的字符集
ALTER TABLE `t_document` 
  MODIFY COLUMN `error_message` TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '错误信息';
