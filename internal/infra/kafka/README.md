# Kafka Infrastructure

`internal/infra/kafka` is organized by Kafka capability rather than as a
single facade package:

- `event`: the project's sole event envelope and DLQ event context.
- `config`: connection, consumer, retry, and DLQ options.
- `producer`: event publishing and the outbox adapter.
- `consumer`: consumption, idempotency state, retries, and offset commits.
- `dlq`: dead-letter inspection and replay.
- `health`: broker reachability checks.
- `internal/client`: shared `kafka-go` reader, writer, headers, and tracing
  plumbing; it is unavailable to callers outside this Kafka integration.

Application composition roots import the capability they need. Kafka-go's
native reader, writer, and message types remain inside the implementation;
the only project-defined message envelope is `event.Event`.

`consumer.ConsumptionStore` remains a narrow dependency so the Kafka
consumer can persist idempotency and processing state without depending on
the PostgreSQL implementation in `platform/messaging`.
