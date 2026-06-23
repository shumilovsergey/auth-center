# auth-center

Централизованный сервис аутентификации. Приложение перенаправляет пользователя на auth-center, тот проверяет личность и возвращает одноразовый код, приложение меняет код на данные пользователя через server-to-server вызов.

```
Пользователь → фронтенд app → auth-center → (Telegram / Solana / Google / ...)
                                       ↓
                              одноразовый code
                                       ↓
                       бэкенд app→ /exchange → данные пользователя
```

Auth-center не хранит сессии и не имеет базы данных — только верифицирует личность.

---

## Подготовка сервера
Скопировать сервисные файлы из `bin/example.*.service` в `/etc/systemd/system/`, заполнить все переменные окружения, затем:

```bash
systemctl daemon-reload
systemctl enable --now auth-center
systemctl enable --now auth-client
```

---

## Компоненты

- [auth-center](auth-center/README.md) — сервер аутентификации, к нему подключаются все приложения
- [auth-client](auth-client/README.md) — демо-клиент и референсная реализация подключения
- [auth-proxy](auth-proxy/README.md) — прокси для Telegram webhook, если VPS с auth-center не имеет доступа к серверам Telegram

---

## Dev flow

Локальная разработка — `docker-compose -f dev-compose.yml up` (горячая перезагрузка через `go run`).

Продакшн-бинарь собирается через `docker-compose -f prod-compose.yml run --rm release` и коммитится в git. На сервере нет сборки — только `git pull` и рестарт сервиса.
