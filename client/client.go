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
	running    bool
}

func NewChatClient(server string) *ChatClient {
	return &ChatClient{
		serverURL: server,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		running: false,
	}
}

// Start - запускает чат клиент (БЛОКИРУЮЩИЙ ВЫЗОВ)
func (c *ChatClient) Start() {
	c.running = true
	
	// Запрашиваем имя
	fmt.Print("Введите ваше имя для чата: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	c.username = strings.TrimSpace(scanner.Text())
	if c.username == "" {
		c.username = "Аноним"
	}
	
	fmt.Printf("\n✅ Добро пожаловать в чат, %s!\n", c.username)
	fmt.Println("📝 Просто вводите сообщения и нажимайте Enter")
	fmt.Println("🚪 Для выхода введите '/back'")
	fmt.Println("📜 История чата:\n")
	
	// Канал для новых сообщений
	msgCh := make(chan string, 50)
	
	// Запускаем горутину для получения сообщений
	go c.receiveMessages(msgCh)
	
	// Запускаем горутину для вывода сообщений
	go c.displayMessages(msgCh)
	
	// Основной цикл ввода сообщений
	scanner = bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		
		// Проверка на выход
		if text == "/back" {
			fmt.Println("Выход из чата...")
			break
		}
		
		// Отправляем сообщение если не пустое
		if text != "" {
			c.sendMessage(text)
		}
	}
	
	c.running = false
	time.Sleep(500 * time.Millisecond) // Даем время на вывод последних сообщений
}

// receiveMessages - постоянно получает новые сообщения
func (c *ChatClient) receiveMessages(msgCh chan<- string) {
	lastCount := 0
	
	for c.running {
		// Получаем историю сообщений
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
		
		// Разбираем сообщения
		content := strings.TrimSpace(string(body))
		if content == "" {
			time.Sleep(2 * time.Second)
			continue
		}
		
		lines := strings.Split(content, "\n")
		
		// Отправляем только новые сообщения
		if len(lines) > lastCount {
			for i := lastCount; i < len(lines); i++ {
				if lines[i] != "" {
					msgCh <- lines[i]
				}
			}
			lastCount = len(lines)
		}
		
		time.Sleep(2 * time.Second)
	}
}

// displayMessages - выводит сообщения в консоль
func (c *ChatClient) displayMessages(msgCh <-chan string) {
	for msg := range msgCh {
		if !c.running {
			return
		}
		// Очищаем текущую строку ввода и выводим сообщение
		fmt.Print("\r\033[K") // Очистить строку
		fmt.Println(msg)
		fmt.Print("> ") // Приглашение для ввода
	}
}

// sendMessage - отправляет сообщение на сервер
func (c *ChatClient) sendMessage(text string) {
	fullMessage := fmt.Sprintf("[%s]: %s", c.username, text)
	
	_, err := c.httpClient.Post(
		c.serverURL,
		"text/plain",
		strings.NewReader(fullMessage),
	)
	
	if err != nil {
		fmt.Println("\r⚠️ Ошибка отправки сообщения")
		fmt.Print("> ")
	}
}
