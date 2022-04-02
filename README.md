=== coub-archive

Программа для загрузки коубов. Способна массово загрузить коубы:
  - Залайканные тобой;
  - Из твоей ленты;
  - Из заданного канала;
  - По заданному тегу.

Коубы (опционально) загружаются в IPFS -- так можно будет легко поделиться сохранёнными шедеврами с сообществом.

**Видео `.mp4` и аудио `.mp3` сохраняются отдельно.**

==== Инструкция

1. Заходим на [страницу с релизами](https://github.com/tekhnus/coub-archive/releases); находим по названию подходящую версию для своего компьютера ("darwin" значит Mac, "arm64" значит процессор M1, "amd64" значит процессор Intel или AMD); скачиваем; распаковываем программу из архива.

*Следующие шаги необходимы только если нужно скачивать лайкнутые коубы или коубы из ленты.*

2. Открываем браузер, заходим на coub.com, заходим в свой аккаунт.
3. Пользуясь консолью разработчика, копируем команду cURL для домена `coub.com`. Как это сделать, читай здесь: https://everything.curl.dev/usingcurl/copyas
4. Создаём непосредственно в своей домашней папке файл с названием `coub-curl.txt` и сохраняем скопированный текст в него.

Внимание: в `coub-curl.txt` находится авторизационная информация, необходимая для скачивания. Не показывайте её никому.

Коубы будут сохраняться в папку `coubs` в домашней директории (либо, опционально, в IPFS-хранилище; для этого на компьютере должна быть запущена IPFS-нода).

=== english version

This is a coub downloader. It can download coubs:
  - Liked by you;
  - From your feed;
  - From the given channel;
  - With a given tag.

The coubs are (optionally) shared via IPFS, so that you can later share them with the community.

**Each coub is saved as a pair of `.mp4` (silent video) and `.mp3` (audio) files.**

==== How to use

1. Go to the [releases page](https://github.com/tekhnus/coub-archive/releases); find the version suitable for your computer ("darwin" means Mac, "arm64" means M1 CPU, "amd64" means Intel or AMD CPU), download the archive and **extract** the program from it.

*The following steps are only required only if you need to download the coubs from your feed or liked coubs*

2. Open your browser, navigate to `coub.com`, log into your account.
3. Copy the cURL command for the `coub.com` domain. To do it follow the instructions here: https://everything.curl.dev/usingcurl/copyas
4. Create a file named `coub-curl.txt` right in your home directory and save the copied text into it.



Warning: `coub-curl.txt` file contains your authentication cookies, which are neeeded for downloading. Do not share its content with other people!

The coubs will be saved in `coubs` folder in your home directory (or into IPFS storage; in this case, an IPFS node should be running on your computer).
