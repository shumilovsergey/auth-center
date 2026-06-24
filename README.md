# auth-center

![baner](/auth-center/tools/baner.webp)

Централизованный сервис аутентификации. Приложение перенаправляет пользователя на auth-center, тот проверяет личность и возвращает одноразовый код, приложение меняет код на данные пользователя через server-to-server вызов.

Auth-center не хранит сессии и не имеет базы данных — только верифицирует личность.


## Компоненты

- [auth-center](auth-center/README.md) — сервер аутентификации, к нему подключаются все приложения
- [auth-client](auth-client/README.md) — демо-клиент и референсная реализация подключения
- [auth-proxy](auth-proxy/README.md) — прокси для Telegram webhook, если VPS с auth-center не имеет доступа к серверам Telegram

