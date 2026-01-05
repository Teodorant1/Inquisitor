**Golang based testing framework for creating secure PDFs that prompt inject Chatgpt to stop it from helping exam cheaters**

-It has 2 main functions:
1. sending N number of simultaneous requests of an image prompt to ChatGPT to check the response, namely if it will reject it.
2. Scanning a PDF with exam questions to extract them and create a new PDF with diagonally overlaid text, specifically designed to trigger AI academic integrity alarms which will force them to refuse to answer. (testing of this PDF isn't currently automated, for now I take screenshots of the PDF or print it out and take pictures of it and then add those pictures to the root folder)

**INSTRUCTIONS FOR HOW TO RUN IN VS CODE**

1. Install Golang Runtime https://go.dev/doc/install
2. Install the Go extension for VS code https://marketplace.visualstudio.com/items?itemName=golang.go
3. Go to https://platform.openai.com/ and acquire an API key for ChatGPT , and also set the spending limits to 10 and 5 dollars, the very minimum, so you don't bankrupt yourself by accident.
4. Create a .env in the root and add the API key with the name OPENAI_API_KEY in it
5. Type "GO RUN ." in terminal to make the program run and read the results in terminal.


![unnamed](https://github.com/user-attachments/assets/cbeabfba-18f3-40b5-af12-a11cd1f4fd8d)
