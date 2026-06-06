# Teleproxy

HTTP(S)/TCP proxy через Yandex Telemost (SFU WebRTC). Позволяет обходить файрволы, используя DataChannel как туннель между клиентом и сервером.

## Как это работает

```
Клиент (SOCKS5 127.0.0.1:1080) ←→ Yandex SFU (goloom.strm.yandex.net) ←→ Сервер (исходит запросы в интернет)
```

Оба конца подключаются к одной и той же конференции Telemost. Yandex SFU ретранслирует DataChannel между ними.

## Требования

- Go 1.21+
- Yandex UID (из cookies yandex.ru, `yandexuid` или `Session_id`)
- Сервер — Linux с доступом в интернет
- Клиент — Windows, Linux или Android

## Установка

```bash
git clone <repo> teleproxy
cd teleproxy
go mod tidy
go build ./cmd/server
go build ./cmd/client
```

## Использование

### 1. Получить Yandex UID

Откройте https://passport.yandex.ru в браузере, откройте DevTools → Console и выполните:

```js
document.cookie.split('; ').find(c => c.startsWith('yandexuid=')).split('=')[1]
```

Или найдите `yandexuid` в cookies любого запроса на yandex.ru.

### 2. Запустить сервер (на удалённой Linux-машине)

```bash
./server -uid=<ВАШ_YANDEX_UID> -name=ProxyServer
```

Сервер создаст конференцию и выведет URL вида:
```
https://telemost.yandex.ru/j/1234567890
```

Этот URL нужно передать клиенту (через чат, Telegram, email и т.д.).

### 3. Запустить клиент (на вашем ПК)

```bash
./client -uid=<ВАШ_YANDEX_UID> -mode=join -url=https://telemost.yandex.ru/j/1234567890
```

Клиент подключится к конференции, поднимет SOCKS5 прокси на `127.0.0.1:1080`.

### 4. Настроить браузер

В Firefox: Настройки → Сеть → Настройка соединения → Ручная конфигурация прокси → SOCKS5 → `127.0.0.1:1080`.
В Chrome: использовать `--proxy-server=socks5://127.0.0.1:1080` при запуске.

## Параметры командной строки

### Сервер

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `-uid` | — | Yandex UID (обязательно) |
| `-name` | `ProxyServer` | Отображаемое имя в конференции |
| `-url` | — | URL конференции для join-режима |
| `-listen` | — | Указать этот флаг для явного создания конференции |

Если не указан ни `-url`, ни `-listen`, сервер создаёт конференцию автоматически.

### Клиент

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `-uid` | — | Yandex UID (обязательно) |
| `-name` | `ProxyClient` | Отображаемое имя в конференции |
| `-mode` | `create` | `create` или `join` |
| `-url` | — | URL конференции (для `-mode=join`) |
| `-socks` | `127.0.0.1:1080` | Адрес SOCKS5 сервера |

## Режимы запуска

### Клиент создаёт конференцию, сервер подключается

```bash
# На клиенте (Windows):
client -uid=12345 -mode=create
# → выведет CONFERENCE URL, скопировать и отправить серверу

# На сервере (Linux):
server -uid=12345 -url=https://telemost.yandex.ru/j/ABC123
```

### Сервер создаёт конференцию, клиент подключается

```bash
# На сервере (Linux):
server -uid=12345
# → выведет CONFERENCE URL, скопировать и отправить клиенту

# На клиенте (Windows):
client -uid=12345 -mode=join -url=https://telemost.yandex.ru/j/ABC123
```

## Сборка для Android

Установите Android NDK и gomobile:

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
gomobile bind -target=android -o teleproxy.aar github.com/teleproxy
```

## Ограничения

- UDP в SOCKS5 поддерживается частично (только связка CONNECT + DNS-резолв через TCP)
- ICE restart и автоматическое переподключение не реализованы
- STUN только (`stun.rtc.yandex.net:3478`), TURN не используется
- DataChannel закрывается при выходе из конференции — сессию нужно перезапускать
