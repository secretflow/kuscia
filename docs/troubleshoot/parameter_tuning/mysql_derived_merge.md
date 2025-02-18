# MySQL（数据库） optimizer_switch 参数优化

## 问题描述

Kuscia 的 SQL 语句可能会存在触发 MySQL 的 Bug，导致 MySQL 直接崩溃。

## 解决方案

1. 将数据库的 optimizer_switch 的优化项 derived_merge 修改为 false。针对不同数据库实例该配置的默认值不同，建议使用前检查该配置是否关闭。永久性修改，需要编辑 MySQL 配置文件（通常是 my.cnf 或 my.ini），然后重启 MySQL 服务。

   ```bash
   optimizer_switch='derived_merge=false';
   ```

2. Kuscia 部署时，建议使用单独的数据库实例，避免与其他业务数据库实例共享优化配置。

## 原因分析

该 Bug 的主要原因是 MySQL 在处理 ORDER BY 子句时，将用于 ORDER BY 操作的选择列表中的子查询通过别名引用解析，但在合并查询块时，会错误地删除这些尚在使用中的子查询。这个问题的根本原因在于 MySQL 在删除未使用的合并派生列时没有正确识别出这些列仍在 ORDER BY 子句中被使用。

MySQL Bug 报告如下：

```bash
Bug #34377854: Bug that can directly crashes MySQL

If order by references a subquery in select list through an alias, and if the query block having the order by gets merged,then while deleting the unused merged derived columns, the subquery gets deleted. This is not correct as it is still being used in order by. To determine if a merged derived column is used or not, "derived_used" was being checked. "derived_used" is not set for this case because resolving for order by does not make a call to fix_fields() when it is resolved against alias in the select list. Since we now have reference counting for items, we can use it to check if a derived column is used or not. So as part of the solution we do the following now.

1. Increment reference count when order by references the select expression.
2. Check the reference count before removing a merged derived table column.
3. Remove the use of "derived_used".
```

触发 MySQL Bug 的 SQL 示例如下：

```sql
SELECT
    (SELECT MAX(rkv.id) AS id FROM kine AS rkv),
    COUNT(c.theid)
FROM (
    SELECT *
    FROM (
        SELECT
            (SELECT MAX(rkv.id) AS id FROM kine AS rkv),
            (SELECT MAX(crkv.prev_revision) AS prev_revision
             FROM kine AS crkv
             WHERE crkv.name = 'compact_rev_key'),
            kv.id AS theid,
            kv.name,
            kv.created,
            kv.deleted,
            kv.create_revision,
            kv.prev_revision,
            kv.lease,
            kv.value,
            kv.old_value
        FROM kine AS kv
        JOIN (
            SELECT MAX(mkv.id) AS id
            FROM kine AS mkv
            WHERE mkv.name LIKE '/registry/apiextensions.k8s.io/customresourcedefinitions/%'
            GROUP BY mkv.name
        ) AS maxkv ON maxkv.id = kv.id
        WHERE kv.deleted = 0 OR 0
    ) AS lkv
    ORDER BY lkv.theid ASC
) c;
```

## 参数介绍

在 MySQL 中，optimizer_switch 是一个系统变量，用于控制查询优化器的行为。通过调整这个参数，您可以开启或关闭某些特定的优化策略，以适应不同的查询场景和性能需求。optimizer_switch 的值是一个由多个标志组成的字符串，每个标志都对应着一种优化策略，常见的优化项如下：

- `index_merge`: 控制是否启用索引合并优化。
- `index_merge_union`: 在 index_merge 下进一步指定是否允许使用 UNION 操作来合并索引扫描结果。
- `index_merge_sort_union`: 允许在执行 UNION 时先排序再合并索引扫描的结果。
- `index_merge_intersection`: 启用基于交集的索引合并。
- `engine_condition_pushdown`: 是否将条件推送到存储引擎层处理，这通常可以提高查询效率。
- `derived_merge`: 对于派生表（子查询）尝试将其与外部查询合并。
- `firstmatch`: 当有多个等价的索引可用时，只选择第一个找到的而不是评估所有可能的索引来决定最佳者。
- `loosescan`: 放宽对于某些类型查询的扫描规则，可能会牺牲准确性但提升速度。
- `materialization`: 使用临时表来存储中间结果，特别是在复杂的连接中。
- `in_to_exists`: 将 IN 子句转换为 EXISTS 子句，有时能提供更好的性能。
- `semijoin`, `loose_scan`, `duplicate_weedout`: 这些都是关于半连接的不同实现方式，用于优化包含子查询的查询语句。
