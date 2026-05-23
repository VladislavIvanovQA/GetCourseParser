# Установка на Windows

## Требования

- Windows 10/11
- Google Chrome или Microsoft Edge
- Для сборки: Go (или `scripts/setup-go.ps1` скачает portable SDK в `tools/`)

## 1. Клонирование

```powershell
git clone https://github.com/VladislavIvanovQA/GetCourseScripts.git
cd GetCourseScripts
```

## 2. Установка

**Готовый архив:** [Releases](https://github.com/VladislavIvanovQA/GetCourseScripts/releases) → `GetCourseDownloader-windows-amd64.zip`.

**Сборка из исходников:**

```powershell
.\scripts\setup-go.ps1      # один раз, если Go не установлен
.\scripts\build.ps1
.\scripts\setup-ffmpeg.ps1  # скачает ffmpeg.exe в dist
```

## 3. Запуск

Запустите:

```
dist\GetCourseDownloader\GetCourseDownloader.exe
```

Не коммитьте `config.json` из этой папки — в нём токен доступа к API.

## 4. Расширение

1. `chrome://extensions` → Режим разработчика
2. Загрузить распакованное → папка `extension`

## 5. Использование

См. раздел «Очередь и прогресс» в [README.md](../README.md).
