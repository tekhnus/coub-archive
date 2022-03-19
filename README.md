A program to download all coubs liked by you.

Each coub is saved into a separate directory as a pair of `.mp4` (silent video) and `.mp3` (audio) files.

How to use
----------
1. Go to the "Releases" page; find the version suitable for your computer ("darwin" means Mac, "arm64" means M1 processor), download the archive and extract it the `coub-arhive` program.
2. Open your browser, navigate to `coub.com`, log into your account.
3. Copy the curl command string for the `coub.com` page into the clipboard. To do it just follow the instructions here: https://everything.curl.dev/usingcurl/copyas
4. Save the copied text into a file named `coub-curl.txt` in the same directory where you placed the `coub-archive` program.
5. Open the terminal, drag and drop the `coub-archive` from the file manager into the terminal and press Enter.

WARNING: `coub-curl.txt` file will contain your authentication cookies.
Do not share its content with other people!

