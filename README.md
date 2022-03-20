=== coub-archive

Программа для загрузки коубов. Способна массово загрузить коубы:
  - Залайканные тобой;
  - Из твоей ленты;
  - Из заданного канала;
  - По заданному тегу.

Коубы (опционально) загружаются в IPFS -- так можно будет легко поделиться сохранёнными шедеврами с сообществом.

**Видео `.mp4` и аудио `.mp3` сохраняются отдельно.**

==== Инструкция

1. Заходим на [страницу с релизами](https://github.com/tekhnus/coub-archive/releases); находим по названию подходящую версию для своего компьютера ("darwin" значит Mac, "arm64" значит процессор M1, "amd64" значит процессор Intel или AMD); скачиваем; распаковываем исполняемый файл `coub-archive`

*Следующие шаги необходимы только если нужно скачивать лайкнутые коубы или коубы из ленты.*

2. Открываем браузер, заходим на coub.com, заходим в свой аккаунт.
3. Пользуясь консолью разработчика, копируем в буфер обмена командную строку curl для соединения `coub.com`. Как это сделать, читай здесь: https://everything.curl.dev/usingcurl/copyas
4. Создаём в той же папке, где находится программа `coub-archive`, файл с названием `coub-curl.txt` и сохраняем скопированный текст в него.

Внимание: в `coub-curl.txt` находится авторизационная информация, необходимая для скачивания. Не показывайте её никому.

=== english version

This is a coub downloader. It can download coubs:
  - Liked by you;
  - From your feed;
  - From the given channel;
  - With a given tag.

The coubs are (optionally) shared via IPFS, so that you can later share them with the community.

**Each coub is saved as a pair of `.mp4` (silent video) and `.mp3` (audio) files.**

==== How to use

1. Go to the [releases page](https://github.com/tekhnus/coub-archive/releases); find the version suitable for your computer ("darwin" means Mac, "arm64" means M1 CPU, "amd64" means Intel or AMD CPU), download the archive and extract it the `coub-arhive` program.

*The following steps are only required only if you need to download the coubs from your feed or liked coubs*

2. Open your browser, navigate to `coub.com`, log into your account.
3. Copy the curl command string for the `coub.com` page into the clipboard. To do it just follow the instructions here: https://everything.curl.dev/usingcurl/copyas
4. Create a file named `coub-curl.txt` in the same directory where you placed the `coub-archive` program and save the copied text into it.



Warning: `coub-curl.txt` file contains your authentication cookies, which are neeeded for downloading. Do not share its content with other people!

