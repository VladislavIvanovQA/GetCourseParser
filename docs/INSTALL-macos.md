# Установка на macOS

## Требования

- macOS 11+
- [Homebrew](https://brew.sh/) (рекомендуется)
- Google Chrome или Microsoft Edge
- Аккаунт GetCourse с доступом к нужным урокам

## 1. Зависимости

```bash
brew install go ffmpeg
```

Проверка:

```bash
go version
ffmpeg -version
```

## 2. Клонирование

```bash
git clone https://github.com/VladislavIvanovQA/GetCourseScripts.git
cd GetCourseScripts
```

Замените URL на свой репозиторий после публикации на GitHub.

## 3. Установка

**Готовый архив:** [GetCourseDownloader-macos.zip](https://github.com/VladislavIvanovQA/GetCourseScripts/releases/latest/download/GetCourseDownloader-macos.zip) (Intel + Apple Silicon).

**Сборка из исходников:**

```bash
chmod +x scripts/build-mac.sh
./scripts/build-mac.sh
```

Будет создано:

```
dist/GetCourseDownloader/
  GetCourseDownloader    # исполняемый файл
  extension/             # копия для удобства (можно использовать из корня repo)
  downloads/             # сюда сохраняются уроки
```

`ffmpeg` берётся из `brew` (PATH). При желании можно положить бинарник `ffmpeg` рядом с `GetCourseDownloader`.

## 4. Первый запуск

```bash
cd dist/GetCourseDownloader
chmod +x GetCourseDownloader
./GetCourseDownloader
```

В терминале появятся:

- порт API (по умолчанию `18765`);
- **токен** — понадобится расширению;
- путь к папке `downloads`.

Рядом создаётся `config.json` — **не публикуйте его** (там секретный токен).

Остановка: `Ctrl+C`.

### Автозапуск (опционально)

Создайте `.plist` для `launchd` или просто держите Terminal/iTerm с запущенным процессом во время скачивания.

## 5. Расширение Chrome

1. Откройте `chrome://extensions`
2. Включите **Режим разработчика**
3. **Загрузить распакованное расширение**
4. Укажите папку **`extension`** в корне репозитория (не `dist/.../extension`, если не хотите дублировать — подойдёт любая копия)

## 6. Использование

Полная пошаговая инструкция (расширение, очередь, ffmpeg, типичные ошибки):  
[README → Инструкция: установка и использование](../README.md#инструкция-установка-и-использование)

Результат скачивания: `dist/GetCourseDownloader/downloads/<название урока>/`

- `lesson.pdf` — страница с вёрсткой (если включено)
- `lesson.txt`, `links.txt`
- вложения
- `video_01.mp4`, …

## 7. Обновление

```bash
git pull
./scripts/build-mac.sh
```

Перезапустите `GetCourseDownloader` и обновите расширение (кнопка ↻ на `chrome://extensions`).

## Устранение неполадок

| Проблема | Решение |
|----------|---------|
| Не подключается | Запущен ли `./GetCourseDownloader`? Порт 18765 не занят? |
| Нет видео | Включите плеер на странице; проверьте `ffmpeg -version` |
| PDF не создаётся | Chrome может показать «отладка» — разрешите; перезапустите вкладку |
| Файл без расширения | Обновите до последней версии (имя + ext из URL) |

## Только bash (без Go и расширения)

```bash
./page_and_video_downloader.sh interactive
```

Интерактивный режим: вставка HTML, ручной ввод cookie/referer, URL видео из DevTools.
