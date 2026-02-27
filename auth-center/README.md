← [telegram-auth](../README.md)

# auth-center

Сервер аутентификации. Принимает пользователя, проверяет личность через Telegram / Solana / Google/.. , выдаёт одноразовый код приложению.

Stateless — нет базы данных, нет хранения сессий между запросами.

---

## Переменные окружения

| Переменная | Обязательно | Описание |
|---|:---:|---|
| `PORT` | | Порт сервера (по умолчанию `8886`) |
| `BOT_TOKEN` | ★ | Токен Telegram-бота из [@BotFather](https://t.me/BotFather) |
| `BOT_USERNAME` | ★ | Username бота без `@` |
| `WEBHOOK_SECRET` | | Секрет для проверки Telegram webhook (задаётся при регистрации webhook) |
| `APP_TOKENS` | ★ | Секреты приложений через запятую — кто может вызывать `/exchange` |
| `DIRECT_REDIRECT` | | Куда редиректить пользователя если он открыл auth-center напрямую без `?redirect=` |
| `GOOGLE_CLIENT_ID` | | Client ID из Google Cloud Console |
| `GOOGLE_CLIENT_SECRET` | | Client Secret из Google Cloud Console |
| `GOOGLE_CALLBACK_URL` | | Полный URL callback'а, должен совпадать с настройкой в Google Cloud (`https://your-domain/google/callback`) |

### Где получить токены

**Telegram**
1. [@BotFather](https://t.me/BotFather) → `/newbot` → скопировать `BOT_TOKEN`
2. Username бота → `BOT_USERNAME`
3. Придумать `WEBHOOK_SECRET` — произвольная строка
4. Зарегистрировать webhook (выполнить один раз после деплоя):

```bash
curl -X POST "https://api.telegram.org/bot<BOT_TOKEN>/setWebhook" -H "Content-Type: application/json" -d '{"url":"https://your-auth-center-domain/webhook","secret_token":"<WEBHOOK_SECRET>"}'
```

**Google**
1. [Google Cloud Console](https://console.cloud.google.com/) → APIs & Services → Credentials → Create OAuth 2.0 Client ID
2. Тип: Web application
3. Authorized redirect URIs: `https://your-auth-center-domain/google/callback`
4. Скопировать Client ID и Client Secret

**APP_TOKENS**
Придумать произвольные строки — по одной на каждое приложение, которое будет использовать auth-center. Те же строки прописать как `APP_TOKEN` в настройках каждого приложения.

---

## Сервисный файл

Шаблон: `go/bin/example.auth-center.service`

Скопировать в `/etc/systemd/system/auth-center.service`, заполнить все переменные.

---

## Подключение приложения

### 1. Отправить пользователя на auth-center

```
https://your-auth-center-domain/?redirect=https://yourapp.com/callback
```

### 2. Принять code

После аутентификации пользователь вернётся на:

```
https://yourapp.com/callback?code=<one-time-code>
```

### 3. Обменять code на данные пользователя

Только с бэкенда, не из браузера:

```http
POST https://your-auth-center-domain/exchange
Content-Type: application/json

{ "code": "<one-time-code>", "app_token": "<твой APP_TOKEN>" }
```

Ответ:

```json
{
  "ok": true,
  "method": "telegram",
  "user": { "id": 123456789, "first_name": "Ivan", "username": "ivan" }
}
```

```json
{
  "ok": true,
  "method": "solana",
  "user": { "id": "5ZX8wKF..." }
}
```

```json
{
  "ok": true,
  "method": "google",
  "user": { "id": "1170...", "email": "ivan@gmail.com", "name": "Ivan" }
}
```

`code` одноразовый, живёт 60 секунд. После успешного `/exchange` удаляется.

### 4. Создать сессию в своём приложении

Auth-center не помнит пользователя — это задача приложения.

```python
session["user_id"] = data["user"]["id"]
session["method"]  = data["method"]
```

`user.id` — постоянный уникальный идентификатор (Telegram ID, Solana pubkey, Google sub). Используй как primary key.
