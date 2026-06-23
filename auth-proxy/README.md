← [auth-center](../README.md)

# auth-proxy

Минимальный прокси-сервис между Telegram и auth-center.

Нужен когда VPS с auth-center не имеет доступа к серверам Telegram.

```
Telegram      →  VPS-2 (auth-proxy /webhook)   →  VPS-1 (auth-center)
VPS-1 (auth)  →  VPS-2 (auth-proxy /tg-api/*)  →  Telegram API
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
cd auth-proxy
cp .env.example .env   # заполнить AUTH_CENTER_URL
docker-compose -f dev-compose.yml up proxy
```

---

## Сборка продакшн-бинаря (linux/amd64)

```bash
cd auth-proxy
docker-compose -f prod-compose.yml run --rm release
```

Бинарь окажется в `bin/auth-proxy`. Скопировать на VPS-2.

---

## Деплой на VPS-2

1. Скопировать бинарь на сервер
2. Скопировать `bin/example.auth-proxy.service` в `/etc/systemd/system/auth-proxy.service`
3. Заполнить `ExecStart`, `WorkingDirectory`, `AUTH_CENTER_URL`
4. Запустить:

```bash
systemctl daemon-reload
systemctl enable --now auth-proxy
```

5. Настроить nginx — SSL-терминация и проксирование на `127.0.0.1:8080`

---

## Перенаправить webhook Telegram

После деплоя переключить webhook на VPS-2. `WEBHOOK_SECRET` остаётся прежним — auth-center его по-прежнему проверяет.

```bash
curl -X POST "https://api.telegram.org/bot<BOT_TOKEN>/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://auth-proxy.your-domain.com/webhook","secret_token":"<WEBHOOK_SECRET>"}'
```

Проверить:

```bash
curl "https://api.telegram.org/bot<BOT_TOKEN>/getWebhookInfo"
```

---

## Проверка работы прокси

Убедиться, что `/tg-api` проксирует запросы к Telegram:

```bash
curl -X POST "https://<proxy-domain>/tg-api/bot<BOT_TOKEN>/getMe"
```

Успешный ответ — JSON от Telegram с данными бота:

```json
{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"MyBot","username":"mybot"}}
```

Если вернулось `{"ok":false,...}` — прокси работает, но токен неверный.  
Если `502 Bad Gateway` — прокси не может достучаться до `api.telegram.org`.

Проверить `/health`:

```bash
curl "https://<proxy-domain>/health"
```

---

## Эндпоинты

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/webhook` | Пересылает тело и заголовок `X-Telegram-Bot-Api-Secret-Token` на auth-center |
| `POST` | `/tg-api/{path...}` | Пересылает запросы к Telegram API от auth-center. Путь подставляется как есть: `/tg-api/botTOKEN/sendMessage` → `https://api.telegram.org/botTOKEN/sendMessage` |
| `GET` | `/health` | Возвращает `200 OK` — для мониторинга |
