// Package mq 将项目的消息发布、消费、重试和死信语义适配到 Kafka。
//
// 消费采用至少一次语义。每条消息先按 consumer group 和 message key 领取消费记录，
// 再执行业务处理；只有处理结果已持久化，或重试/DLQ 消息已成功写入后，才提交原始
// Kafka offset。消费者因此可能收到重复消息，上层业务处理必须保持幂等。
package mq
