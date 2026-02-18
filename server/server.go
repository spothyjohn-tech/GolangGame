package server

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type ChatServer struct {
	history []string
	mutex   sync.RWMutex
	logCh   chan string

	pvpQueue     []*PvPPlayer
	pvpMatches   map[string]*PvPMatch
	pvpMutex     sync.RWMutex
	queueMutex   sync.Mutex
	matchCounter int

	pvpChats  map[string][]string
	chatMutex sync.RWMutex
	registeredNames map[string]bool
}

type PvPPlayer struct {
	Name     string
	HP       int
	MaxHP    int
	Strength int
}

type PvPMatch struct {
	ID               string
	Player1          *PvPPlayer
	Player2          *PvPPlayer
	Player1HP        int
	Player2HP        int
	Round            int
	Move1            *MoveData
	Move2            *MoveData
	Result           string
	mutex            sync.RWMutex
	ResultForPlayer1 string
	ResultForPlayer2 string
}

type MoveData struct {
	Attack int
	Block  int
}

func NewChatServer() *ChatServer {
	return &ChatServer{
		history:      make([]string, 0),
		logCh:        make(chan string, 20),
		pvpQueue:     make([]*PvPPlayer, 0),
		pvpMatches:   make(map[string]*PvPMatch),
		pvpChats:     make(map[string][]string),
		matchCounter: 0,
		registeredNames: make(map[string]bool),
	}
}

func (s *ChatServer) Start(port string) {
	go s.printLogs()
	go s.readServerInput()

	http.HandleFunc("/", s.handleRequests)
	http.HandleFunc("/pvp/join", s.handlePvPJoin)
	http.HandleFunc("/pvp/status", s.handlePvPStatus)
	http.HandleFunc("/pvp/battle", s.handlePvPBattle)
	http.HandleFunc("/pvp/move", s.handlePvPMove)
	http.HandleFunc("/pvp/chat", s.handlePvPChat)

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
		message := strings.TrimSpace(string(body))
		if message != "" {
			s.addMessage(message)
			s.logCh <- "Клиент: " + message
		}
		fmt.Fprint(w, "получено")

	case http.MethodGet:
		history := s.getHistory()
		for _, msg := range history {
			if msg != "" {
				fmt.Fprintln(w, msg)
			}
		}

	default:
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

func (s *ChatServer) handlePvPJoin(w http.ResponseWriter, r *http.Request) {
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	parts := strings.Split(string(body), "|")
	if len(parts) < 4 {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	name := parts[0]
	hp, _ := strconv.Atoi(parts[1])
	maxHP, _ := strconv.Atoi(parts[2])
	strength, _ := strconv.Atoi(parts[3])

	// Проверка уникальности имени
	s.chatMutex.Lock()
	if s.registeredNames[name] {
		s.chatMutex.Unlock()
		http.Error(w, "name_taken", http.StatusConflict)
		return
	}
	s.registeredNames[name] = true
	s.chatMutex.Unlock()

	
	// 2. Проверяем очередь
	s.queueMutex.Lock()
	for _, p := range s.pvpQueue {
		if p.Name == name {
			s.queueMutex.Unlock()
			http.Error(w, "already in queue", http.StatusConflict)
			return
		}
	}
	s.queueMutex.Unlock()


	// 3. Проверяем активные матчи
	s.pvpMutex.RLock()
	for _, match := range s.pvpMatches {
		if (match.Player1.Name == name || match.Player2.Name == name) && match.Result == "" {
			s.pvpMutex.RUnlock()
			http.Error(w, "already in match", http.StatusConflict)
			return
		}
	}
	s.pvpMutex.RUnlock()

	// 4. Только теперь регистрируем имя
	s.chatMutex.Lock()
	s.registeredNames[name] = true
	s.chatMutex.Unlock()

	player := &PvPPlayer{
		Name:     name,
		HP:       hp,
		MaxHP:    maxHP,
		Strength: strength,
	}

	if len(s.pvpQueue) > 0 {
		player1 := s.pvpQueue[0]
		s.pvpQueue = s.pvpQueue[1:]

		s.matchCounter++
		matchID := fmt.Sprintf("match_%d", s.matchCounter)

		match := &PvPMatch{
			ID:        matchID,
			Player1:   player1,
			Player2:   player,
			Player1HP: player1.HP,
			Player2HP: player.HP,
			Round:     1,
		}

		s.pvpMutex.Lock()
		s.pvpMatches[matchID] = match
		s.pvpMutex.Unlock()

		response := fmt.Sprintf("match:%s|%s|%d|%d|%d",
			matchID, player1.Name, player1.HP, player1.MaxHP, player1.Strength)
		fmt.Fprint(w, response)

		s.logCh <- fmt.Sprintf("PvP: Матч %s: %s vs %s", matchID, player1.Name, player.Name)
	} else {
		s.pvpQueue = append(s.pvpQueue, player)
		fmt.Fprint(w, "queued")
		s.logCh <- fmt.Sprintf("PvP: %s в очереди", player.Name)
	}
}

func (s *ChatServer) handlePvPStatus(w http.ResponseWriter, r *http.Request) {
	playerName := r.URL.Query().Get("player")

	s.pvpMutex.RLock()
	for _, match := range s.pvpMatches {
		if (match.Player1.Name == playerName || match.Player2.Name == playerName) && match.Result == "" {
			var opponent *PvPPlayer
			if match.Player1.Name == playerName {
				opponent = match.Player2
			} else {
				opponent = match.Player1
			}

			response := fmt.Sprintf("match:%s|%s|%d|%d|%d",
				match.ID, opponent.Name, opponent.HP, opponent.MaxHP, opponent.Strength)
			s.pvpMutex.RUnlock()
			fmt.Fprint(w, response)
			return
		}
	}
	s.pvpMutex.RUnlock()

	fmt.Fprint(w, "waiting")
}

func (s *ChatServer) handlePvPBattle(w http.ResponseWriter, r *http.Request) {
	matchID := r.URL.Query().Get("matchId")
	playerName := r.URL.Query().Get("player")

	s.pvpMutex.RLock()
	match, exists := s.pvpMatches[matchID]
	s.pvpMutex.RUnlock()

	if !exists {
		http.Error(w, "Match not found", http.StatusNotFound)
		return
	}

	match.mutex.Lock()
	defer match.mutex.Unlock()

	// Проверка завершения
	if match.Player1HP <= 0 || match.Player2HP <= 0 {
		if match.Player1HP <= 0 && match.Player2HP <= 0 {
			fmt.Fprint(w, "finished:draw")
		} else if match.Player1HP <= 0 {
			if match.Player1.Name == playerName {
				fmt.Fprint(w, "finished:loss")
			} else {
				fmt.Fprint(w, "finished:win")
			}
		} else {
			if match.Player2.Name == playerName {
				fmt.Fprint(w, "finished:loss")
			} else {
				fmt.Fprint(w, "finished:win")
			}
		}
		return
	}

	delete(s.pvpChats, matchID)

	s.chatMutex.Lock()
	delete(s.registeredNames, match.Player1.Name)
	delete(s.registeredNames, match.Player2.Name)
	s.chatMutex.Unlock()

	s.pvpMutex.Lock()
	delete(s.pvpMatches, matchID)
	s.pvpMutex.Unlock()

	// Расчет раунда
	if match.Move1 != nil && match.Move2 != nil && match.ResultForPlayer1 == "" && match.ResultForPlayer2 == "" {
		oldPlayer1HP := match.Player1HP
		oldPlayer2HP := match.Player2HP

		damageToPlayer1 := calculatePvPDamage(match.Player2.Strength, match.Move2.Attack, match.Move1.Block)
		damageToPlayer2 := calculatePvPDamage(match.Player1.Strength, match.Move1.Attack, match.Move2.Block)

		match.Player1HP -= damageToPlayer1
		if match.Player1HP < 0 {
			match.Player1HP = 0
		}
		match.Player2HP -= damageToPlayer2
		if match.Player2HP < 0 {
			match.Player2HP = 0
		}

		s.logCh <- fmt.Sprintf("PvP Раунд %d: %s нанес %d, %s нанес %d",
			match.Round, match.Player1.Name, damageToPlayer2, match.Player2.Name, damageToPlayer1)

		match.ResultForPlayer1 = fmt.Sprintf("round_result:%d|%d|%d|%d|%d|%d|%d",
			match.Round, damageToPlayer2, oldPlayer1HP, match.Player1HP, damageToPlayer1, oldPlayer2HP, match.Player2HP)
		match.ResultForPlayer2 = fmt.Sprintf("round_result:%d|%d|%d|%d|%d|%d|%d",
			match.Round, damageToPlayer1, oldPlayer2HP, match.Player2HP, damageToPlayer2, oldPlayer1HP, match.Player1HP)
	}

	// Отправка результата
	if match.Player1.Name == playerName && match.ResultForPlayer1 != "" {
		fmt.Fprint(w, match.ResultForPlayer1)
		match.ResultForPlayer1 = ""
	} else if match.Player2.Name == playerName && match.ResultForPlayer2 != "" {
		fmt.Fprint(w, match.ResultForPlayer2)
		match.ResultForPlayer2 = ""
	}

	// Сброс ходов
	if match.ResultForPlayer1 == "" && match.ResultForPlayer2 == "" && match.Move1 != nil && match.Move2 != nil {
		match.Move1 = nil
		match.Move2 = nil
		match.Round++
		return
	}

	// Определение текущего хода
	if match.Move1 == nil || match.Move2 == nil {
		currentTurn := match.Player1.Name
		if match.Move1 != nil {
			currentTurn = match.Player2.Name
		}
		fmt.Fprintf(w, "wait_turn:%s", currentTurn)
		return
	}
}

func (s *ChatServer) handlePvPMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	parts := strings.Split(string(body), "|")
	if len(parts) < 4 {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	matchID := parts[0]
	playerName := parts[1]
	attack, _ := strconv.Atoi(parts[2])
	block, _ := strconv.Atoi(parts[3])

	s.pvpMutex.RLock()
	match, exists := s.pvpMatches[matchID]
	s.pvpMutex.RUnlock()

	if !exists {
		http.Error(w, "Match not found", http.StatusNotFound)
		return
	}

	match.mutex.Lock()
	defer match.mutex.Unlock()

	if playerName == match.Player1.Name {
		if match.Move1 != nil {
			http.Error(w, "Move already submitted", http.StatusBadRequest)
			return
		}
		match.Move1 = &MoveData{Attack: attack, Block: block}
		s.logCh <- fmt.Sprintf("PvP: %s сделал ход", playerName)
	} else if playerName == match.Player2.Name {
		if match.Move2 != nil {
			http.Error(w, "Move already submitted", http.StatusBadRequest)
			return
		}
		match.Move2 = &MoveData{Attack: attack, Block: block}
		s.logCh <- fmt.Sprintf("PvP: %s сделал ход", playerName)
	} else {
		http.Error(w, "Invalid player", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *ChatServer) handlePvPChat(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		data := string(body)
		if !strings.HasPrefix(data, "chat:") {
			http.Error(w, "Invalid format", http.StatusBadRequest)
			return
		}

		parts := strings.SplitN(data[5:], "|", 3)
		if len(parts) != 3 {
			http.Error(w, "Invalid data", http.StatusBadRequest)
			return
		}

		matchID := parts[0]
		playerName := parts[1]
		message := parts[2]

		s.pvpMutex.RLock()
		match, exists := s.pvpMatches[matchID]
		s.pvpMutex.RUnlock()

		if !exists {
			http.Error(w, "Match not found", http.StatusNotFound)
			return
		}

		if match.Player1.Name != playerName && match.Player2.Name != playerName {
			http.Error(w, "Not in match", http.StatusForbidden)
			return
		}

		s.chatMutex.Lock()
		s.pvpChats[matchID] = append(s.pvpChats[matchID], fmt.Sprintf("%s: %s", playerName, message))
		s.chatMutex.Unlock()

		s.logCh <- fmt.Sprintf("[Чат %s] %s: %s", matchID, playerName, message)
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		matchID := r.URL.Query().Get("matchId")
		playerName := r.URL.Query().Get("player")
		lastStr := r.URL.Query().Get("last")
		last, _ := strconv.Atoi(lastStr)

		s.pvpMutex.RLock()
		match, exists := s.pvpMatches[matchID]
		s.pvpMutex.RUnlock()

		if !exists {
			http.Error(w, "Match not found", http.StatusNotFound)
			return
		}

		if match.Player1.Name != playerName && match.Player2.Name != playerName {
			http.Error(w, "Not in match", http.StatusForbidden)
			return
		}

		s.chatMutex.RLock()
		chat, ok := s.pvpChats[matchID]
		s.chatMutex.RUnlock()

		if !ok || last >= len(chat) {
			fmt.Fprint(w, "")
			return
		}

		for i := last; i < len(chat); i++ {
			fmt.Fprintln(w, chat[i])
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func calculatePvPDamage(strength, attack, block int) int {
	damage := strength + 5

	switch attack {
	case 0:
		damage = int(float64(damage) * 1.3)
	case 2:
		damage = int(float64(damage) * 0.8)
	}

	if attack == block {
		damage = damage / 2
	}

	if damage < 5 {
		damage = 5
	}

	return damage
}
