-- 清理无效的文档分块数据（字符编码错误的数据）
USE `ragent`;

-- 查看有问题的分块（如果有的话）
-- 注意：这个查询可能会很慢，因为需要检查每个content字段
-- SELECT id, doc_id, chunk_index, LEFT(content, 50) as content_preview 
-- FROM t_document_chunk 
-- WHERE content IS NOT NULL 
-- LIMIT 10;

-- 如果确实有编码错误的数据，可以删除这些分块
-- DELETE FROM t_document_chunk WHERE id IN (
--     SELECT id FROM (
--         SELECT id FROM t_document_chunk 
--         WHERE content NOT REGEXP '^[[:print:][:space:]]*$'
--     ) AS temp
-- );

-- 或者更安全的方式：只删除最近失败的文档的分块
-- DELETE FROM t_document_chunk 
-- WHERE doc_id IN (
--     SELECT id FROM t_document 
--     WHERE status = 'failed' 
--     AND error_message LIKE '%1366%'
-- );

-- 显示最近失败的文档
SELECT id, name, status, error_message, create_time 
FROM t_document 
WHERE status = 'failed' 
ORDER BY create_time DESC 
LIMIT 10;
