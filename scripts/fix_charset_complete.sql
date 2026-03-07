-- 完整修复字符集问题
-- 确保数据库使用 utf8mb4
ALTER DATABASE `ragent` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE `ragent`;

-- 检查并修复文档分块表
-- 先转换整个表
ALTER TABLE `t_document_chunk` 
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 确保 content 列使用正确的字符集
ALTER TABLE `t_document_chunk` 
  MODIFY COLUMN `content` TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '分块内容';

-- 检查并修复文档表
ALTER TABLE `t_document` 
  CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 确保 error_message 列使用正确的字符集
ALTER TABLE `t_document` 
  MODIFY COLUMN `error_message` TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '错误信息';

-- 确保 name 列也使用正确的字符集
ALTER TABLE `t_document` 
  MODIFY COLUMN `name` VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '文档名称';

ALTER TABLE `t_document` 
  MODIFY COLUMN `file_name` VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '原始文件名';

-- 验证字符集设置
SELECT 
    TABLE_SCHEMA,
    TABLE_NAME,
    TABLE_COLLATION
FROM 
    information_schema.TABLES
WHERE 
    TABLE_SCHEMA = 'ragent' 
    AND TABLE_NAME IN ('t_document', 't_document_chunk');

SELECT 
    TABLE_SCHEMA,
    TABLE_NAME,
    COLUMN_NAME,
    CHARACTER_SET_NAME,
    COLLATION_NAME
FROM 
    information_schema.COLUMNS
WHERE 
    TABLE_SCHEMA = 'ragent' 
    AND TABLE_NAME IN ('t_document', 't_document_chunk')
    AND COLUMN_NAME IN ('content', 'error_message', 'name', 'file_name');
