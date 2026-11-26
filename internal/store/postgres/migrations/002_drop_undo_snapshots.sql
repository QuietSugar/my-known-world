-- 002: 撤销功能下线，操作表收敛为审计日志。
--
-- 撤销的实现方式是「每次写操作存两份整图快照」，撤销时整图覆盖回去。数据量
-- 一大就不可持续：131 节点时单份快照 84–397 KB，历史表已占 5.9 MB，而且改
-- 一台机器的 IP 也要重写整张图并序列化两次。同时「只能撤销最新一条」的限制
-- 让功能在同步频繁时几乎不可用。
--
-- 改为：不再提供撤销，改用数据库备份兜底；业务删除改为逻辑删除（lifecycle
-- 置 retired）。graph_mutation_operations 保留下来当审计日志——谁在什么时候
-- 做了什么仍然可见，但不再存快照。
--
-- 注意：本文件不得包含 BEGIN/COMMIT，Migrate 已在外层开启事务。

-- 三个快照列承载了绝大部分体积，且只服务撤销。
ALTER TABLE graph_mutation_operations DROP COLUMN IF EXISTS before_state;
ALTER TABLE graph_mutation_operations DROP COLUMN IF EXISTS after_state;
ALTER TABLE graph_mutation_operations DROP COLUMN IF EXISTS after_hash;

-- 撤销记录表只服务于已下线的撤销功能。
DROP TABLE IF EXISTS graph_operation_undos;
