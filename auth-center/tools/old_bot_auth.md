[<- Назад](/auth-center/README.md)

# Telegram-вход через бота (снято с эксплуатации)

Так Telegram-ветка auth-center работала до перехода на [auth-miniapp](/auth-miniapp/README.md). Код удалён; документ существует, чтобы схему можно было восстановить или объяснить, не поднимая историю git.

Заменено на мини-апп после того, как тот отработал на живых пользователях: подпись `initData` сходится, `start_param` покрыт этой подписью, вход проходит целиком. Мини-аппу не нужен ни webhook, ни исходящий доступ к Telegram — а значит и `auth-proxy` для этой ветки.

## Как это работало

```
вкладка ──POST /qr-session──> auth-center           (сессия, токен, QR)
   │
   │  пользователь открывает https://t.me/<bot>?start=<токен>
   ▼
Telegram ──> бот получает "/start <токен>"
   │
   │  апдейт доставляется webhook'ом
   ▼
POST /webhook ──> auth-center ищет сессию по токену и заполняет её
   │
   ├──> sendTG(): "You are authenticated!" в чат, со ссылкой на redirect
   ▼
вкладка ──GET /poll/{токен}──> code ──> redirect?code=…
```

Первая половина (`/qr-session`, сессии, TTL) и вторая (`/poll`, одноразовый код, `/exchange`) не менялись при переходе — заменён только средний участок, способ доставки личности.

## Что для этого требовалось

| Переменная | Зачем |
|---|---|
| `BOT_TOKEN` | вызовы Telegram API из `sendTG` |
| `BOT_USERNAME` | сборка ссылки `https://t.me/<bot>?start=<токен>` |
| `WEBHOOK_SECRET` | сверка заголовка `X-Telegram-Bot-Api-Secret-Token` на входящем апдейте |
| `TELEGRAM_API_URL` | база Telegram API; подменялась на `https://<auth-proxy>/tg-api`, когда VPS не видел Telegram |

Плюс зарегистрированный у Telegram webhook:

```bash
curl -F "url=https://auth-center.sh-development.ru/webhook" \
     -F "secret_token=<WEBHOOK_SECRET>" \
     https://api.telegram.org/bot<BOT_TOKEN>/setWebhook
```

`BOT_TOKEN` и `BOT_USERNAME` живы и сейчас — первый выводит общий секрет с auth-miniapp, второй собирает ссылку на мини-апп. Ушли `WEBHOOK_SECRET` и `TELEGRAM_API_URL`.

## Ключевые места удалённого кода

**`POST /webhook`** — принимал апдейт, проверял секрет заголовком, доставал `/start <токен>` из `message.text` и заполнял сессию:

```go
if strings.HasPrefix(text, "/start ") {
    tok := strings.TrimSpace(strings.TrimPrefix(text, "/start "))

    sessionsMu.Lock()
    sess, ok := sessions[tok]
    if ok && sess.Status == "pending" && time.Since(sess.CreatedAt) <= sessionTTL {
        user := map[string]any{
            "id": from.ID, "first_name": from.FirstName,
            "last_name": from.LastName, "username": from.Username,
        }
        sess.Status = "authenticated"
        sess.User = user
        redirect := sess.Redirect
        if redirect != "" {
            sess.Code = newCode(user, "telegram")
        }
        sessionsMu.Unlock()
        go sendTG(from.ID, "You are authenticated!", redirect)
    }
}
```

Этот блок целиком перешёл в `POST /miniapp/auth` (`build/miniapp.go`) — там та же логика, но на входе проверенная `initData` вместо тела апдейта.

**`sendTG()`** — слал пользователю в чат подтверждение со ссылкой на приложение:

```go
apiURL := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIURL, botToken)
payload := map[string]any{"chat_id": chatID, "text": text}
if redirectURL != "" {
    payload["text"] = text + "\n\n" + redirectURL
    payload["link_preview_options"] = map[string]any{
        "url": redirectURL, "show_above_text": true,
    }
}
```

Аналога у мини-аппа нет и не появилось: он сам показывает результат и кнопку возврата, пока пользователь на него смотрит.

## Чем мини-апп отличается по существу

| | бот | мини-апп |
|---|---|---|
| Доставка личности | входящий webhook | `POST /miniapp/auth` от auth-miniapp |
| Чем доказана личность | доверием к Telegram, доставившему апдейт | HMAC-подпись `initData` по `BOT_TOKEN` |
| Исходящий доступ к Telegram | нужен (`sendTG`) | не нужен |
| Публичный webhook | нужен | не нужен |
| `auth-proxy` для Telegram | нужен, если VPS не видит Telegram | не нужен |
| Что видит пользователь после входа | сообщение в чате | страница мини-аппа с кнопкой «назад» |

## Что осталось от старой схемы

`auth-proxy` продолжает работать — у него есть другие задачи (Grafana-алерты, GitHub-прокси). Маршруты `POST /webhook` и `POST /tg-api/{path...}` в нём никто больше не зовёт; удалять их отдельное решение, к auth-center оно отношения не имеет.

Откат к схеме с ботом — это `git revert` соответствующего коммита плюс регистрация webhook заново. Переменной окружения для отката больше нет: `MINIAPP_SHORT_NAME` теперь обязателен, потому что другой дороги в Telegram-ветке не осталось.
