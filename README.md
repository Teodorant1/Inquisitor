**Golang based testing framework for creating secure PDFs that prompt inject Chatgpt to stop it from helping exam cheaters**

-It has 2 main functions:
1. sending N number of simultaneous requests of an image prompt to ChatGPT to check the response, namely if it will reject it.
2. Scanning a PDF with exam questions to extract them and create a new PDF with diagonally overlaid text, specifically designed to trigger AI academic integrity alarms which will force them to refuse to answer. 

**INSTRUCTIONS FOR HOW TO RUN IN VS CODE**

1. Install Golang Runtime https://go.dev/doc/install
2. Install the Go extension for VS code https://marketplace.visualstudio.com/items?itemName=golang.go
3. Go to https://platform.openai.com/ and acquire an API key for ChatGPT , and also set the spending limits to 10 and 5 dollars, the very minimum, so you don't bankrupt yourself by accident.
4. Create a .env in the root and add the API key with the name OPENAI_API_KEY in it and add other values from .env.example
5. Make sure poppler-windows is installed and the bin is added to path , the one i used successfully is https://github.com/oschwartz10612/poppler-windows/releases/tag/v25.12.0-0 (if you are on Macintosh you can install Poppler via homebrew via terminal command)
6. Type "GO RUN ." in terminal to make the program run and start the server
7. Make sure you also have the frontend running, available at https://github.com/Teodorant1/inquisitor-requiem

<img width="1024" height="1024" alt="image" src="https://github.com/user-attachments/assets/686ee7c3-3ffa-4d1a-8370-41aa23556ee5" />

