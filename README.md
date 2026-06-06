# Teleproxy

HTTP(S)/TCP proxy через Yandex Telemost (SFU WebRTC). Позволяет обходить файрволы, используя DataChannel как туннель между клиентом и сервером.

## Как это работает

```
Клиент (SOCKS5) ←→ Yandex SFU ←→ Сервер (исходит запросы в интернет)
```

Оба конца подключаются к одной конференции Telemost. Yandex SFU ретранслирует DataChannel между ними.

**Сервер** (на VPS) создаёт конференцию и держит её вечно (автопереподключение при обрыве).  
**Клиент** (на ПК) просто подключается по ссылке — никакой авторизации не нужно.

## Требования

- Go 1.21+
- **Сервер:** Linux, Yandex UID + Session_id (для создания конференции)
- **Клиент:** Windows/Linux/macOS, только ссылка на конференцию

## Установка

```bash
git clone https://github.com/Th3K3rn3l/teleproxy.git
cd teleproxy
go build ./cmd/server
go build ./cmd/client
```

## Использование

### 1. Сервер (VPS) — создаёт конференцию

На удалённой машине:

```bash
./server -mode=create -uid=1234567890 -session="3:168..."
```

- `-uid` — `yandexuid` из cookie
- `-session` — `Session_id` из cookie (берётся в браузере DevTools → Application → Cookies → yandex.ru)

Сервер создаст конференцию, выведет URL и останется в ней навсегда:

```
=== CONFERENCE URL ===
https://telemost.yandex.ru/j/abc123def
======================
```

### 2. Клиент (ПК) — подключается по ссылке

На вашем рабочем компьютере (без всяких cookie):

```bash
./client -url=https://telemost.yandex.ru/j/abc123def
```

Клиент подключится к конференции и поднимет SOCKS5 прокси на `127.0.0.1:1080`.

### 3. Настроить браузер

**Firefox:** Настройки → Сеть → Настройка соединения → Ручная конфигурация прокси → SOCKS5 → `127.0.0.1:1080`.  
**Chrome:** `chrome --proxy-server=socks5://127.0.0.1:1080`

Готово. Весь трафик браузера пойдёт через VPS.

## Параметры

### Сервер

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `-mode` | `create` | `create` или `join` |
| `-uid` | — | Yandex UID (для `-mode=create`) |
| `-session` | — | Session_id cookie (для `-mode=create`) |
| `-name` | `ProxyServer` | Имя в конференции |
| `-url` | — | URL конференции (для `-mode=join`) |

### Клиент

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `-url` | — | Ссылка на конференцию от сервера |
| `-name` | `ProxyClient` | Имя в конференции |
| `-socks` | `127.0.0.1:1080` | Адрес SOCKS5 прокси |

## Как получить Session_id

1. Откройте https://passport.yandex.ru в браузере, войдите в аккаунт
2. DevTools (F12) → Application → Cookies → `yandex.ru`
3. Найдите `Session_id` — длинная строка вида `3:168...`
4. Найдите `yandexuid` — число, это ваш UID

Эти данные нужны только серверу для создания конференции. Клиенту — ничего.

## Режим join для сервера

Если сервер перезагрузился и конференция ещё жива (кто-то в ней остаётся), можно подключиться к существующей:

```bash
./server -mode=join -url=https://telemost.yandex.ru/j/abc123def
```

Сборка для Android

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
gomobile bind -target=android -o teleproxy.aar github.com/teleproxy
```

## Ограничения

- UDP в SOCKS5 — частично (только CONNECT + DNS через TCP)
- STUN только (`stun.rtc.yandex.net:3478`), TURN не используется
- Если Yandex SFU не принимает подключение без credentials, серверу нужно раздавать их клиентам через отдельный канал
