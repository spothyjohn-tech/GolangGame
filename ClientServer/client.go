package client

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type ChatClient struct {
	serverURL  string
	username   string
	httpClient *http.Client
	displayCh  chan string
}

func NewChatClient(server string) *ChatClient {
	return &ChatClient{
		serverURL: server,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		displayCh: make(chan string, 10),
	}
}

func (c *ChatClient) Start() {
	go c.listenForMessages()
	go c.pollServer()

	c.askUsername()
	c.readInput()
}

func (c *ChatClient) listenForMessages() {
	for msg := range c.displayCh {
		fmt.Println(msg)
	}
}

func (c *ChatClient) pollServer() {
	lastCount := 0

	for {
		resp, err := c.httpClient.Get(c.serverURL)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		lines := strings.Split(strings.TrimSpace(string(body)), "\n")

		if len(lines) > lastCount && lines[0] != "" {
			for i := lastCount; i < len(lines); i++ {
				c.displayCh <- lines[i]
			}
			lastCount = len(lines)
		}

		time.Sleep(2 * time.Second)
	}
}

func (c *ChatClient) askUsername() {
	fmt.Print("Подключено. Введите ваше имя: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	c.username = scanner.Text()
}

func (c *ChatClient) readInput() {
	fmt.Println("Введите сообщение:")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		fullMessage := fmt.Sprintf("[%s]: %s", c.username, text)

		_, err := c.httpClient.Post(
			c.serverURL,
			"text/plain",
			strings.NewReader(fullMessage),
		)

		if err != nil {
			fmt.Println("Ошибка отправки:", err)
		}
	}
}
