// Package support 提供多个 Usecase 共用的应用服务。
//
// 其中 OutboxPublisher 将已提交的本地事件投递到外部消息系统；授权默认值安装器保证
// 系统预置角色和权限可以安全地重复初始化。
package support
