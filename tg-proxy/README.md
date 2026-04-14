← [auth-center](../README.md)

# tg-proxy

Минимальный прокси-сервис. Принимает Telegram webhook и пересылает его на auth-center.

Нужен когда VPS с auth-center не имеет доступа к серверам Telegram.

```
Telegram → VPS-2 (tg-proxy) → VPS-1 (auth-center /webhook)
```

---

## Переменные окружения

| Переменная | Обязательно | Описание |
|---|:---:|---|
| `AUTH_CENTER_URL` | ★ | Базовый URL auth-center, например `https://auth-center.sh-development.ru` |
| `PORT` | | Порт сервера (по умолчанию `8080`) |

---

## Локальная разработка

```bash
cd tg-proxy/go
cp .env.example .env   # заполнить AUTH_CENTER_URL
docker-compose up proxy
```

---

## Сборка продакшн-бинаря (linux/amd64)

```bash
cd tg-proxy/go
docker-compose run --rm release
```

Бинарь окажется в `bin/tg-proxy`. Скопировать на VPS-2.

---

## Деплой на VPS-2

1. Скопировать бинарь на сервер
2. Скопировать `go/bin/example.tg-proxy.service` в `/etc/systemd/system/tg-proxy.service`
3. Заполнить `ExecStart`, `WorkingDirectory`, `AUTH_CENTER_URL`
4. Запустить:

```bash
systemctl daemon-reload
systemctl enable --now tg-proxy
```

5. Настроить nginx — SSL-терминация и проксирование на `127.0.0.1:8080`

---

## Перенаправить webhook Telegram

После деплоя переключить webhook на VPS-2. `WEBHOOK_SECRET` остаётся прежним — auth-center его по-прежнему проверяет.

```bash
curl -X POST "https://api.telegram.org/bot<BOT_TOKEN>/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://tg-proxy.your-domain.com/webhook","secret_token":"<WEBHOOK_SECRET>"}'
```

Проверить:

```bash
curl "https://api.telegram.org/bot<BOT_TOKEN>/getWebhookInfo"
```

---

## Эндпоинты

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/webhook` | Пересылает тело и заголовок `X-Telegram-Bot-Api-Secret-Token` на auth-center |
| `GET` | `/health` | Возвращает `200 OK` — для мониторинга |
