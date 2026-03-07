-- 修复文档分块表的字符集问题
USE `ragent`;

-- 确保数据库字符集
ALTER DATABASE `ragent` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 确保表字符集
ALTER TABLE `t_document_chunk` CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 确保 content 字段使用 utf8mb4
ALTER TABLE `t_document_chunk` 
MODIFY COLUMN `content` TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '分块内容';

-- 检查字符集设置
SELECT 
    TABLE_NAME,
    COLUMN_NAME,
    CHARACTER_SET_NAME,
    COLLATION_NAME
FROM 
    INFORMATION_SCHEMA.COLUMNS
WHERE 
    TABLE_SCHEMA = 'ragent'
    AND TABLE_NAME = 't_document_chunk'
    AND COLUMN_NAME = 'content';
