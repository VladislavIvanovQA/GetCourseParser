# GetCourse Downloader

Инструмент для **личного архивирования** уроков [GetCourse](https://getcourse.ru/): вложения, текст, PDF страницы, видео (HLS).  
Состоит из **расширения Chrome** (сбор данных и очередь) и **локального приложения** (скачивание через `ffmpeg`).

> **Важно:** используйте только для материалов, на которые у вас есть право доступа. Не публикуйте `config.json` и не коммитьте папку `downloads/` — там могут быть личные ссылки и куки.

---

## Скачать готовую сборку

Полный список релизов: **[Releases](https://github.com/VladislavIvanovQA/GetCourseScripts/releases)**.

Ниже — **прямые ссылки на артефакты** последнего релиза (`/releases/latest/download/…`).  
Скачайте архив под свою систему (не нужно знать «amd64» — смотрите колонку «Кому»):

| Кому | Скачать |
|------|---------|
| Windows 10/11, обычный ПК (Intel/AMD 64-bit) | [GetCourseDownloader-windows-x64.zip](https://github.com/VladislavIvanovQA/GetCourseScripts/releases/latest/download/GetCourseDownloader-windows-x64.zip) |
| Windows на ARM (Surface Pro X и др.) | [GetCourseDownloader-windows-arm64.zip](https://github.com/VladislavIvanovQA/GetCourseScripts/releases/latest/download/GetCourseDownloader-windows-arm64.zip) |
| Старый Windows 32-bit | [GetCourseDownloader-windows-x86.zip](https://github.com/VladislavIvanovQA/GetCourseScripts/releases/latest/download/GetCourseDownloader-windows-x86.zip) |
| macOS 11+ (Intel и Apple Silicon — один файл) | [GetCourseDownloader-macos.zip](https://github.com/VladislavIvanovQA/GetCourseScripts/releases/latest/download/GetCourseDownloader-macos.zip) |
| Linux 64-bit (Ubuntu, Fedora, …) | [GetCourseDownloader-linux-x64.zip](https://github.com/VladislavIvanovQA/GetCourseScripts/releases/latest/download/GetCourseDownloader-linux-x64.zip) |
| Linux ARM64 (Raspberry Pi, некоторые VPS) | [GetCourseDownloader-linux-arm64.zip](https://github.com/VladislavIvanovQA/GetCourseScripts/releases/latest/download/GetCourseDownloader-linux-arm64.zip) |

Для видео нужен **ffmpeg** той же разрядности, что и программа (расширение подскажет ссылку, если его нет).  
Дальше — **[инструкция по установке и использованию](#инструкция-установка-и-использование)**.

<details>
<summary>Почему несколько файлов, а не один «на всех»?</summary>

У процессоров разная **архитектура** — одна программа не запустится на «чужом» CPU. Поэтому в CI собираются отдельные бинарники:

- **x64** — почти все современные ПК и ноутбуки Windows/Linux;
- **arm64** — Mac с Apple Silicon, Windows на ARM, многие одноплатники;
- **x86** — редкие старые 32-bit Windows;
- **macOS** — один ZIP с «универсальным» бинарником (Intel + Apple Silicon внутри).

Раньше в названии было `amd64` — это то же самое, что **x64** для Windows/Linux.
</details>

---

## Инструкция: установка и использование

Нужны **две части**: локальная программа (скачивает файлы) и **расширение Chrome** (собирает страницу, куки и ссылки на видео). Работают вместе через `http://127.0.0.1:18765` на вашем компьютере.

### Шаг 1. Распакуйте и запустите программу

1. Скачайте ZIP из таблицы выше и распакуйте в любую папку, например `C:\Tools\GetCourseDownloader\`.
2. Запустите:
   - **Windows:** `GetCourseDownloader.exe`
   - **macOS / Linux:** `./GetCourseDownloader` (в терминале из этой папки)
3. Оставьте окно/процесс **запущенным** на время скачивания. В консоли будет адрес API и токен — это нормально.
4. При первом запуске рядом появится `config.json` (токен доступа) и папка `downloads/` — **не публикуйте** `config.json`.

### Шаг 2. ffmpeg (только если нужны видео)

Без ffmpeg скачаются **вложения, текст и PDF**, но не HLS-видео.

| ОС | Что сделать |
|----|-------------|
| **Windows** | Положите `ffmpeg.exe` **в ту же папку**, что и `GetCourseDownloader.exe`. Ссылку на скачивание покажет расширение, или: `.\scripts\setup-ffmpeg.ps1` из репозитория. |
| **macOS** | `brew install ffmpeg` или бинарник `ffmpeg` рядом с программой. |
| **Linux** | `sudo apt install ffmpeg` (или аналог) / положите `ffmpeg` рядом с программой. |

Разрядность ffmpeg должна совпадать с программой (x64, arm64 и т.д.).

### Шаг 3. Установите расширение Chrome

Подойдёт **Google Chrome** или **Microsoft Edge** (Chromium).

1. Откройте страницу расширений:
   - Chrome: `chrome://extensions`
   - Edge: `edge://extensions`
2. Включите **«Режим разработчика»** (переключатель справа вверху).
3. Нажмите **«Загрузить распакованное расширение»** (Load unpacked).
4. Укажите папку **`extension`**:
   - из распакованного ZIP (`…/GetCourseDownloader/extension`), **или**
   - из клона репозитория (`GetCourseScripts/extension`).
5. Закрепите иконку расширения на панели (необязательно, но удобно).

После обновления релиза: на `chrome://extensions` нажмите **«Обновить»** у карточки расширения (или перезагрузите распакованное).

### Шаг 4. Первое подключение к программе

1. Убедитесь, что **GetCourseDownloader запущен**.
2. В браузере **войдите в GetCourse** под своим аккаунтом (как обычно).
3. Нажмите на **иконку расширения** → откроется окно «GetCourse Downloader».
4. Нажмите **«Подключиться»**.
   - Порт по умолчанию: `18765` (менять не нужно, если не меняли в `config.json`).
   - Токен подставится автоматически из программы.
5. Если всё ок — появится сообщение «Подключено». Если ffmpeg нет — оранжевый блок с инструкцией и кнопкой **«Скачать ffmpeg»**.

### Шаг 5. Скачать урок

1. Откройте **страницу урока** GetCourse, например:  
   `https://ваша-школа.getcourse.ru/pl/teach/control/lesson/view?id=…`
2. **Включите видео в плеере** на 5–10 секунд (пауза тоже подойдёт) — иначе ссылки HLS могут не перехватиться.
3. Снова откройте расширение на этой вкладке.
4. Отметьте при необходимости:
   - **PDF страницы** — сохранить страницу как PDF;
   - **Текст lesson.txt** — текст урока.
5. Нажмите:
   - **«+ В очередь»** — добавить урок и продолжить смотреть другие; **или**
   - **«Скачать сейчас»** — то же самое, сразу в работу.
6. Окно расширения **можно закрыть** — загрузка идёт в фоне. На иконке:
   - `…` — идёт скачивание;
   - цифра — сколько уроков ждут в очереди.
7. Прогресс виден в popup (список задач) или снова откройте иконку расширения.

**Где лежат файлы:** папка `downloads/` рядом с программой, внутри — подпапка с названием урока (файлы, `lesson.txt`, PDF, видео `.mp4`).

### Частые вопросы

| Проблема | Решение |
|----------|---------|
| «Не удалось подключиться» | Запущен ли `GetCourseDownloader`? Не блокирует ли порт 18765 файрвол? |
| Видео 0 | Включите плеер до скачивания; установите ffmpeg; нажмите «Проверить снова» в расширении. |
| Лишние видео / не те ролики | Откройте нужный урок заново, не переключайтесь между уроками в одной вкладке без перезагрузки. |
| Только YouTube/Vimeo на странице | Встроенный GC-плеер не используется — автоматически не скачается. |
| Очередь зависла | Перезапустите программу и расширение; вкладку с уроком оставьте открытой. |

Подробности по ОС: [Windows](docs/INSTALL-windows.md) · [macOS](docs/INSTALL-macos.md)

---

## Совместимость с GetCourse

GetCourse — **единая платформа** (SaaS): у каждой школы свой домен (`school.getcourse.ru` или свой сайт на инфраструктуре GC), но типовая разметка и API похожи.

Парсер ориентирован на стандартные паттерны:

| Что | Паттерн |
|-----|---------|
| Страница урока | `/pl/teach/control/lesson/view?id=…` |
| Файлы | `/pl/fileservice/…/file/download/…` |
| Видео (часто) | `gceuproxy.com` / Kinescope, master playlist `…/api/playlist/master/…` |
| Плеер | `*.gcfiles.net` |

**Обычно работает** на большинстве школ GetCourse с классическим интерфейсом урока.

**Может не сработать или работать частично:**

- урок только с YouTube/Vimeo без GC-плеера (видео не перехватится автоматически);
- нестандартная вёрстка / старые шаблоны без `/pl/`;
- нет доступа к уроку (не куплен курс, истёк доступ);
- жёсткий DRM или нестандартный CDN;
- вы не включили видео в плеере до скачивания (для HLS нужны сетевые запросы).

---

## Структура проекта

```
├── extension/          # Расширение Chrome (MV3)
├── desktop/            # Локальный сервер на Go
├── scripts/            # Сборка (Windows / macOS)
├── page_and_video_downloader.sh   # Альтернатива: только bash (любая ОС)
└── dist/               # Сюда попадает сборка (в git не коммитится)
```

---

## Сборка из исходников (разработчикам)

**Windows:**

```powershell
.\scripts\setup-go.ps1    # один раз, если нет Go
.\scripts\build.ps1
.\scripts\setup-ffmpeg.ps1
# dist\GetCourseDownloader\GetCourseDownloader.exe
```

**macOS:**

```bash
brew install go ffmpeg
chmod +x scripts/build-mac.sh && ./scripts/build-mac.sh
cd dist/GetCourseDownloader && ./GetCourseDownloader
```

Расширение — папка `extension` в корне репозитория или в `dist`. Дальше — [инструкция](#инструкция-установка-и-использование).

---

## Сборка релиза (maintainers)

Тег `v*` запускает [GitHub Actions](.github/workflows/release.yml): Windows (x64, ARM, x86), macOS universal, Linux (x64, ARM64) → Release.

```bash
git tag v1.0.0
git push origin v1.0.0
```

Локально: `scripts/build.ps1` / `scripts/build-mac.sh`, затем `scripts/package-release.ps1` или `package-release.sh`.

---

## Публикация на GitHub — что не коммитить

В `.gitignore` уже исключено:

- `dist/` (сборки, `config.json` с токеном, скачанные уроки)
- `downloads/`
- `tools/` (локальный SDK Go для Windows)
- `config.json` в любой папке

Перед первым push проверьте:

```bash
git status
# не должно быть: downloads, config.json, *.exe с токенами, lesson.txt с URL школы
```

Если форкнете репозиторий под своим именем, при желании смените module path:

```bash
cd desktop
go mod edit -module=github.com/YOUR_USER/YOUR_REPO
# и обновите import paths в .go файлах
```

---

## Альтернатива: только bash

```bash
chmod +x page_and_video_downloader.sh
./page_and_video_downloader.sh interactive
```

Нужны: `python3`, `curl`, `ffmpeg`, bash.

---

## API (localhost)

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/health` | Проверка |
| GET | `/api/pair` | Токен для расширения |
| POST | `/api/lesson` | Загрузка (`async: true` → опрос `/api/job?id=`) |
| GET | `/api/job?id=` | Прогресс фоновой задачи |

---

## Лицензия

MIT — см. [LICENSE](LICENSE). Автор не связан с GetCourse. Использование на свой риск и в рамках правил школы/платформы.
