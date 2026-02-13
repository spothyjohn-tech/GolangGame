package server

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

type ChatServer struct {
	history []string
	mutex   sync.RWMutex
	logCh   chan string
}

func NewChatServer() *ChatServer {
	return &ChatServer{
		history: make([]string, 0),
		logCh:   make(chan string, 20),
	}
}

func (s *ChatServer) Start(port string) {
	go s.printLogs()
	go s.readServerInput()

	http.HandleFunc("/", s.handleRequests)

	s.logCh <- "Сервер запущен на порту " + port
	http.ListenAndServe(":"+port, nil)
}

func (s *ChatServer) printLogs() {
	for msg := range s.logCh {
		fmt.Println(msg)
	}
}

func (s *ChatServer) readServerInput() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		text := scanner.Text()

		s.addMessage(text)
		s.logCh <- "Вы: " + text
	}
}

func (s *ChatServer) addMessage(msg string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.history = append(s.history, msg)
}

func (s *ChatServer) getHistory() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	copyHistory := make([]string, len(s.history))
	copy(copyHistory, s.history)
	return copyHistory
}

func (s *ChatServer) handleRequests(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Ошибка чтения", http.StatusBadRequest)
			return
		}

		message := string(body)
		s.addMessage(message)
		s.logCh <- "Клиент: " + message

		fmt.Fprint(w, "получено")

	case http.MethodGet:
		history := s.getHistory()
		for _, msg := range history {
			fmt.Fprintln(w, msg)
		}

	default:
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
	}
}
