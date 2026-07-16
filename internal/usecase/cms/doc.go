// Package cms 编排 CMS 管理和公开内容查询用例。
//
// 写用例负责保持 locale、分类树、标签、文章翻译、slug、重定向和审计记录的一致性。
// 需要多表写入或写入审计事件的操作必须在 TxManager 的同一事务中完成。
package cms
