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

Распакуйте → запустите программу → подключите расширение из папки `extension` (см. `RELEASE.txt` в архиве).  
Для видео нужен **ffmpeg** той же разрядности, что и программа (расширение подскажет ссылку, если его нет).

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

## Быстрый старт (Windows)

1. Собрать приложение:
   ```powershell
   .\scripts\setup-go.ps1    # один раз, если нет Go
   .\scripts\build.ps1
   .\scripts\setup-ffmpeg.ps1
   ```
2. Запустить `dist\GetCourseDownloader\GetCourseDownloader.exe`
3. Chrome → `chrome://extensions` → **Загрузить распакованное** → папка `extension`
4. На уроке: **Подключиться** → **+ В очередь** или **Скачать сейчас** (видео в плеере должно хотя бы раз проиграться)

Подробнее: [docs/INSTALL-windows.md](docs/INSTALL-windows.md)

---

## Быстрый старт (macOS)

1. Установить зависимости:
   ```bash
   brew install go ffmpeg
   ```
2. Собрать и запустить:
   ```bash
   chmod +x scripts/build-mac.sh
   ./scripts/build-mac.sh
   cd dist/GetCourseDownloader
   ./GetCourseDownloader
   ```
3. Расширение — как на Windows, папка `extension` из корня репозитория.

Подробнее: [docs/INSTALL-macos.md](docs/INSTALL-macos.md)

---

## Очередь и прогресс (расширение v1.5+)

- Загрузки идут **в фоне** — popup можно закрыть.
- **+ В очередь** — добавить урок с текущей вкладки.
- Статус и прогресс сохраняются; на иконке расширения: `…` (идёт работа) или цифра (ожидают).
- Опции: PDF страницы, `lesson.txt`, видео.

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
