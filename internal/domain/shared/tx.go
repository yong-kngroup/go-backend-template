package shared

import "context"

// TxManager 为 Usecase 提供跨 Repository 写入的事务边界。
//
// Do 调用 fn 时传入的 context 携带当前事务。回调内的所有 Repository 调用必须使用
// 该 context；fn 返回错误时整个事务回滚，返回 nil 时提交。
type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
