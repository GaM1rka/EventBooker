# EventBooker

EventBooker — сервис бронирования мест на мероприятия с дедлайном подтверждения. Если пользователь не подтверждает бронь за заданное время, worker автоматически отменяет ее и публикует email-уведомление в notification-service через Kafka.

## Состав

- `backend/services/event-service` — Go API и worker отмены просроченных броней.
- `backend/services/notification-service` — Go Kafka consumer + SMTP отправка, перенесен из Weather-Alert-Service.
- `frontend` — простой статический веб-интерфейс для пользователя и администратора.
- `swagger/openapi.yaml` — OpenAPI 3.0 спецификация.
- `docker-compose.yml` — Postgres, Kafka, API, worker, notification-service и frontend.

## Запуск

```bash
cp .env.example .env
docker compose up --build
```

После запуска:

- frontend: `http://localhost:3000`
- API: `http://localhost:8080`
- Swagger файл: `swagger/openapi.yaml`

Если Docker установлен через snap и `docker compose` падает с `snap-confine has elevated permissions`, нужно починить snap/apparmor окружение или использовать обычную установку Docker Engine.

## Быстрая проверка API

Создать мероприятие с TTL брони 2 минуты:

```bash
curl -X POST http://localhost:8080/events \
  -H 'Content-Type: application/json' \
  -d '{"title":"Go workshop","eventDate":"2026-06-01T19:00","capacity":2,"bookingTtlMinutes":2,"requiresPayment":true}'
```

Посмотреть мероприятие:

```bash
curl http://localhost:8080/events/1
```

Забронировать место:

```bash
curl -X POST http://localhost:8080/events/1/book \
  -H 'Content-Type: application/json' \
  -d '{"userName":"Ivan","userEmail":"ivan@example.com"}'
```

Подтвердить бронь:

```bash
curl -X POST http://localhost:8080/events/1/confirm \
  -H 'Content-Type: application/json' \
  -d '{"userEmail":"ivan@example.com"}'
```

## Безопасность данных

Бронирование и подтверждение выполняются в транзакциях. При бронировании строка мероприятия блокируется через `FOR UPDATE`, просроченные pending-брони для мероприятия сначала отменяются, затем считается занятость. Worker отменяет просрочку батчами через `FOR UPDATE SKIP LOCKED`, поэтому несколько worker-процессов не должны обрабатывать одну и ту же бронь одновременно.
