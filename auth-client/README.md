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

## Telegram link preview (OG-теги)

Когда auth-center отправляет пользователю сообщение с URL приложения, Telegram автоматически показывает превью страницы — иконку и название. Чтобы это работало, нужно добавить три meta-тега в `<head>` HTML-страницы приложения.

### Что добавить в HTML

```html
<meta property="og:type"  content="website" />
<meta property="og:title" content="ИМЯ ПРИЛОЖЕНИЯ" />
<meta property="og:image" content="https://ДОМЕН/icon.png" />
```

### Как сделать иконку

Иконка — PNG 512×512, тёмный фон с закруглёнными углами, символ в цвете `#c4b5fd` (нeon-purple, соответствует стилю auth-center).

Генерируется скриптом на Python (требует `Pillow`):

```python
from PIL import Image, ImageDraw

SIZE = 512
img = Image.new('RGBA', (SIZE, SIZE), (0, 0, 0, 0))
draw = ImageDraw.Draw(img)

S = SIZE / 32  # масштаб от SVG 32x32

# фон #111120 с радиусом 8
draw.rounded_rectangle([0, 0, SIZE-1, SIZE-1], radius=int(8*S), fill=(17, 17, 32, 255))

neon = (196, 181, 253, 255)  # #c4b5fd
lw = max(2, int(2 * S))

# здесь рисуем символ — пример: иконка ключа (как в auth-client)
cx, cy, cr = 11*S, 16*S, 5*S
draw.ellipse([cx-cr, cy-cr, cx+cr, cy+cr], outline=neon, width=lw)
draw.line([(16*S, 16*S), (27*S, 16*S)], fill=neon, width=lw)
draw.line([(22*S, 16*S), (22*S, 21*S)], fill=neon, width=lw)
draw.line([(26*S, 16*S), (26*S, 19*S)], fill=neon, width=lw)

img.save('web/icon.png')
```

Символ меняется под каждое приложение — главное сохранить фон и цвет.

### Где хранить иконку

Положить `icon.png` в `web/` рядом с `index.html` — она встроится в бинарь через `//go:embed web` и будет доступна по `/icon.png`.

### URL для og:image

После деплоя иконка доступна по адресу `https://ДОМЕН/icon.png`. Можно также использовать raw-ссылку из GitHub (если репозиторий публичный):

```
https://raw.githubusercontent.com/USER/REPO/main/APP/go/build/web/icon.png
```

---

## Сервисный файл

Шаблон: `go/bin/example.auth-client.service`

Скопировать в `/etc/systemd/system/auth-client.service`, заполнить все переменные.
