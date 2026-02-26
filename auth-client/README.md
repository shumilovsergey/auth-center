← [telegram-auth](../README.md)

# auth-client

Демо-клиент и референсная реализация подключения к auth-center. Показывает полный цикл аутентификации: редирект → получение code → обмен на данные пользователя → сессия.

---

## Переменные окружения

| Переменная | Обязательно | Описание |
|---|:---:|---|
| `PORT` | | Порт сервера (по умолчанию `8890`) |
| `AUTH_URL` | ★ | Публичный URL auth-center — куда перенаправляется браузер пользователя |
| `AUTH_INTERNAL` | ★ | Внутренний URL auth-center для server-to-server вызова `/exchange` (может совпадать с `AUTH_URL`, если сервисы на разных машинах) |
| `APP_URL` | ★ | Публичный URL этого приложения — auth-center редиректит сюда после аутентификации |
| `APP_TOKEN` | ★ | Секрет для `/exchange` — должен совпадать с одним из значений `APP_TOKENS` в auth-center |
| `SECRET_KEY` | ★ | Секрет для подписи cookie-сессий, произвольная строка |

### Важно

`AUTH_INTERNAL` отличается от `AUTH_URL` при деплое на один сервер:
- `AUTH_URL=https://auth-center.example.com` — для браузера
- `AUTH_INTERNAL=http://localhost:8886` — для внутреннего вызова без DNS и TLS (если на одном сервере клиент и сервер)

`APP_TOKEN` никогда не попадает в браузер — только в server-to-server запросе к `/exchange`.

---

## Сервисный файл

Шаблон: `go/bin/example.auth-client.service`

Скопировать в `/etc/systemd/system/auth-client.service`, заполнить все переменные.
