// Package cms 实现 CMS 领域端口的 PostgreSQL 查询和实体映射。
//
// Repository 只表达持久化读写，不决定发布、slug 重定向或审计等业务流程；这些规则由
// usecase 层在事务中编排。所有方法均应使用 context 中已有的事务连接。
package cms
