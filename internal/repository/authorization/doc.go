// Package authorization 实现角色、权限和用户授权关系的 PostgreSQL 持久化。
//
// Repository 保证初始化操作可重复执行，并由 Usecase 负责将多项关系变更包裹在同一
// 事务中。
package authorization
