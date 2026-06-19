# bas-remote-go

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/bablosoft/bas-remote-go)](https://goreportcard.com/report/github.com/bablosoft/bas-remote-go)

Go-порт библиотеки [bas-remote-python](https://github.com/bablosoft/bas-remote-python) — клиент для удалённого управления функциями [BrowserAutomationStudio (BAS)](https://bablosoft.com/shop/BrowserAutomationStudio) из Go-приложений.

> Полная документация по возможностям BAS, созданию пользовательских скриптов и архитектуре протокола доступна в [оригинальном репозитории](https://github.com/bablosoft/bas-remote-python).

---

## Содержание

- [Установка](#установка)
- [Быстрый старт](#быстрый-старт)
- [API](#api)
- [Архитектура](#архитектура)
- [Тестовый проект](#тестовый-проект)
- [Лицензия](#лицензия)

---

## Установка

```bash
go get github.com/bablosoft/bas-remote-go
```

Требования: **Go 1.21+**, **Windows** (движок BAS работает только под Windows).

---

## Быстрый старт

Пример выполняет поиск в Google через BAS-функцию `GoogleSearch` из тестового скрипта `TestRemoteControl`.

```go
package main

import (
	"fmt"
	"log"
	"time"

	basremote "github.com/bablosoft/bas-remote-go"
)

func main() {
	opts := &basremote.Options{
		ScriptName: "TestRemoteControl",
	}

	client, err := basremote.New(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Запуск клиента — загружает движок BAS и подключается по WebSocket.
	if err := client.Start(60 * time.Second); err != nil {
		log.Fatal(err)
	}

	// Запуск BAS-функции и ожидание результата.
	fn, err := client.RunFunction("GoogleSearch", map[string]interface{}{
		"Query": "cats",
	})
	if err != nil {
		log.Fatal(err)
	}

	result := <-fn.Result()
	if result.Err != nil {
		log.Fatal(result.Err)
	}

	fmt.Println("Результат:", string(result.Value))
}
```

---

## API

### `basremote.New(opts *Options) (*BasRemoteClient, error)`

Создаёт клиент. Принимает `*Options`:

| Поле | Тип | Описание |
|---|---|---|
| `ScriptName` | `string` | **Обязательно.** Имя BAS-скрипта. |
| `Login` | `string` | Логин аккаунта BAS с доступом к скрипту. |
| `Password` | `string` | Пароль аккаунта BAS. |
| `WorkingDir` | `string` | Папка для хранения движка. По умолчанию `<cwd>/data`. |

---

### `client.Start(timeout time.Duration) error`

Скачивает движок BAS (если нужно), запускает его и устанавливает WebSocket-соединение. Блокирует до завершения handshake или истечения `timeout`.

---

### `client.RunFunction(name string, params map[string]interface{}) (*BasFunction, error)`

Запускает BAS-функцию в отдельном потоке. Результат читается из канала:

```go
fn, err := client.RunFunction("Add", map[string]interface{}{"X": 5, "Y": 3})
if err != nil {
    log.Fatal(err)
}

res := <-fn.Result()
if res.Err != nil {
    log.Fatal(res.Err)
}
fmt.Println("Сумма:", string(res.Value)) // "8"
```

Метод `fn.Stop()` немедленно останавливает выполнение.

---

### `client.CreateThread() *BasThread`

Создаёт переиспользуемый поток BAS. В отличие от `RunFunction`, поток не останавливается между вызовами — эффективнее для серии задач.

```go
thread := client.CreateThread()
defer thread.Stop()

thread.RunFunction("SetProxy", map[string]interface{}{
    "Proxy":    "127.0.0.1:8080",
    "IsSocks5": false,
})
res := <-thread.Result()

thread.RunFunction("CheckIp", nil)
ip := <-thread.Result()
fmt.Println("IP:", string(ip.Value))
```

---

### `client.Send / client.SendAsync`

Низкоуровневые методы для отправки произвольных сообщений протокола BAS:

```go
// Отправить без ожидания ответа, получить ID сообщения.
id, err := client.Send("my_type", map[string]interface{}{"key": "val"}, false)

// Отправить и дождаться ответа.
raw, err := client.SendAsync("get_global_variable", map[string]interface{}{"name": "myVar"})
```

---

### `client.Close() error`

Закрывает WebSocket-соединение и завершает процесс движка BAS.

---

## Архитектура

Библиотека повторяет архитектуру [bas-remote-python](https://github.com/bablosoft/bas-remote-python), адаптированную под идиомы Go:

```
BasRemoteClient
├── engineService   — скачивает zip, распаковывает, запускает FastExecuteScript.exe
├── socketService   — WebSocket-клиент с буферизацией по сепаратору ---Message--End---
├── BasFunction     — одноразовый вызов (goroutine: start_thread → run_task → stop_thread)
└── BasThread       — многоразовый поток (lazy start_thread, atomic isRunning)
```

**Протокол WebSocket:**

- Сообщения сериализуются в JSON и разделяются строкой `---Message--End---`.
- Handshake: `initialize` → `accept_resources` → `thread_start` (готово) / `message` (ошибка аутентификации).
- Async-запросы отслеживаются по `id` через `sync.Map[int]chan json.RawMessage`.

**Вместо asyncio** используются goroutine + channel:

| Python | Go |
|---|---|
| `asyncio.Future` | `chan runResult` |
| `asyncio.create_task` | `go func()` |
| `await` | `<-ch` |
| `AsyncIOEventEmitter` | внутренний `map[string][]func` + `sync.RWMutex` |

Движок BAS скачивается автоматически в `WorkingDir/engine/<version>/` при первом запуске и переиспользуется в дальнейшем.

---

## Тестовый проект

Скрипт **TestRemoteControl** доступен для тестирования без BAS-подписки. Он предоставляет следующие функции:

| Функция | Параметры | Возвращает |
|---|---|---|
| `Add` | `X int, Y int` | Сумму X + Y |
| `SetProxy` | `Proxy string, IsSocks5 bool` | — |
| `CheckIp` | — | Внешний IP-адрес |
| `GoogleSearch` | `Query string` | Список URL результатов |

```go
// Пример: сложение
fn, _ := client.RunFunction("Add", map[string]interface{}{"X": 10, "Y": 20})
res := <-fn.Result()
fmt.Println(string(res.Value)) // 30

// Пример: проверка IP
fn2, _ := client.RunFunction("CheckIp", nil)
ip := <-fn2.Result()
fmt.Println(string(ip.Value)) // "1.2.3.4"
```

---

## Лицензия

[MIT](LICENSE) — приложения, использующие эту библиотеку, можно распространять коммерчески. Подписка BAS Premium требуется только разработчикам, создающим собственные скрипты, но не конечным пользователям.

---

<p align="center">
  Go-порт <a href="https://github.com/bablosoft/bas-remote-python">bablosoft/bas-remote-python</a> •
  <a href="https://bablosoft.com/shop/BrowserAutomationStudio">BAS</a> •
  <a href="https://bablosoft.com">bablosoft.com</a>
</p>
