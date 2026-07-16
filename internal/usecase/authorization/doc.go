// Package authorization 编排角色、权限和用户角色关系的管理用例。
//
// 写操作会先确保默认授权数据存在；涉及角色及其权限集合的复合写入由用例建立事务边界。
// HTTP 层应先认证用户，再调用本包提供的管理员访问或细粒度权限判断。
package authorization
