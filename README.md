# Telegram persistent conversation bot (Go)

Простой Telegram‑бот, повторяющий логику [`persistentconversationbot.py`](https://github.com/python-telegram-bot/python-telegram-bot/blob/master/examples/persistentconversationbot.py): ведёт диалог с пользователем, собирает данные по темам и сохраняет состояние на диск, чтобы не потерять историю между перезапусками.

## Требования
- Go 1.22+ (для локального запуска/тестов)
- Docker (для контейнерного запуска)

## Конфигурация
- `TELEGRAM_TOKEN` — токен бота, обязателен.
- `STATE_FILE` — путь к файлу состояния (по умолчанию `data/state.json`).
- `BOT_DEBUG` — `1`, чтобы включить подробные логи библиотеки.

## Локальный запуск
```bash
go run ./...
```
Бот читает токен из `TELEGRAM_TOKEN`, состояние пишет в `STATE_FILE` или `data/state.json`.

## Запуск в Docker
```bash
docker build -t go-conversation-bot .
docker run --rm \
  -e TELEGRAM_TOKEN=YOUR_TOKEN \
  -v $(pwd)/data:/data \
  go-conversation-bot
```
Том `/data` хранит `state.json`, чтобы диалог не терялся между рестартами.

## Автотесты
```bash
go test ./...
```
`conversation_test.go` проверяет основной сценарий (старт → выбор темы → ввод → завершение) и сохранение/загрузку пользовательского состояния с кастомной категорией.

## Как устроено
- `main.go` — инициализация бота (токен, путь состояния, debug), создание клавиатуры, цикл обработки апдейтов, отправка ответов.
- `conversation.go` — логика диалога и персистентность:
  - `conversationManager` управляет состояниями чатов, сохраняет их в `fileStore` после каждого шага.
  - Поддерживаются команды `/start`, `/show_data`, `/cancel`/`/stop` и кнопка `Done`.
  - Стадии: выбор категории (`CHOOSING`), ввод произвольной категории (`TYPING_CHOICE`), ввод значения (`TYPING_REPLY`).
  - Персистентность — простой JSON‑файл (`fileStore`), атомарная запись через временный файл.
- `Dockerfile` — многостадийная сборка (CGO выключен), финальный образ на Alpine с небортовым пользователем и томом `/data`.
