// Package authorization 定义角色、权限和用户授权关系的领域模型。
//
// 默认权限与超级管理员角色是系统启动所需的基线数据；初始化必须幂等，且业务代码通过
// 权限码进行授权，不依赖角色名称作为权限判断依据。
package authorization
