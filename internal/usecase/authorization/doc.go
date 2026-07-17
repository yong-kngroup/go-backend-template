// Package authorization 编排角色、权限和用户角色关系的管理用例。
//
// 默认授权数据由服务启动阶段的 bootstrap 用例安装；本包假设初始化已经完成。涉及角色
// 及其权限集合的复合写入由用例建立事务边界。HTTP 层应先认证用户，再调用本包提供的
// 管理员访问或细粒度权限判断。
package authorization
