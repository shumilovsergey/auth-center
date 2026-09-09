# auth-center

![baner](/auth-center/tools/baner.webp)

Централизованный сервис аутентификации. Приложение перенаправляет пользователя на auth-center, тот проверяет личность и возвращает одноразовый код, приложение меняет код на данные пользователя через server-to-server вызов.

Auth-center не хранит сессии и не имеет базы данных — только верифицирует личность.


## Компоненты

- [auth-center](auth-center/README.md) — сервер аутентификации, к нему подключаются все приложения
- [auth-client](auth-client/README.md) — демо-клиент и референсная реализация подключения
- [auth-proxy](auth-proxy/README.md) — прокси общего назначения (Grafana-алерты, GitHub). Telegram-вход через него больше не идёт
- [auth-miniapp](auth-miniapp/README.md) — Telegram-вход через Mini App


