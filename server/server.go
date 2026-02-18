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
	
	// PvP данные
	pvpQueue  []*PvPPlayer
	pvpMatches map[string]*PvPMatch
	pvpMutex  sync.RWMutex
	queueMutex sync.Mutex
	matchCounter int
	registeredNicks map[string]bool
	nickMutex sync.Mutex
}

type PvPPlayer struct {
	Name     string
	HP       int
	MaxHP    int
	Strength int
}

type PvPMatch struct {
	ID        string
	Player1   *PvPPlayer
	Player2   *PvPPlayer
	Player1HP int
	Player2HP int
	Round     int
	Move1     *MoveData
	Move2     *MoveData
	Result    string
	mutex     sync.RWMutex
	// Новые поля для хранения результатов
	ResultForPlayer1 string
	ResultForPlayer2 string
	Chat      []string     // Эфемерный чат
	chatMutex sync.Mutex
}

type MoveData struct {
	Attack int
	Block  int
}

func NewChatServer() *ChatServer {
	return &ChatServer{
		registeredNicks: make(map[string]bool),
		history:    make([]string, 0),
		logCh:      make(chan string, 20),
		pvpQueue:   make([]*PvPPlayer, 0),
		pvpMatches: make(map[string]*PvPMatch),
		matchCounter: 0,
	}
}

func (s *ChatServer) Start(port string) {
	go s.printLogs()
	go s.readServerInput()

	// Чат
	http.HandleFunc("/", s.handleRequests)
	http.HandleFunc("/check-nick", s.handleCheckNick)
	http.HandleFunc("/register-nick", s.handleRegisterNick)
	http.HandleFunc("/pvp/chat", s.HandlePvPChat)
	http.HandleFunc("/pvp/chat/history", s.HandlePvPChatHistory)
	
	// PvP
	http.HandleFunc("/pvp/join", s.handlePvPJoin)
	http.HandleFunc("/pvp/status", s.handlePvPStatus)
	http.HandleFunc("/pvp/battle", s.handlePvPBattle)
	http.HandleFunc("/pvp/move", s.handlePvPMove)

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

// ========== PvP ОБРАБОТЧИКИ ==========

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

	// Формат: Имя|HP|MaxHP|Сила
	parts := strings.Split(string(body), "|")
	if len(parts) < 4 {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	hp, _ := strconv.Atoi(parts[1])
	maxHP, _ := strconv.Atoi(parts[2])
	strength, _ := strconv.Atoi(parts[3])

	player := &PvPPlayer{
		Name:     parts[0],
		HP:       hp,
		MaxHP:    maxHP,
		Strength: strength,
	}

	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()

	// Если есть игрок в очереди, создаем матч
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

		// Отвечаем текущему игроку
		response := fmt.Sprintf("match:%s|%s|%d|%d|%d", 
			matchID, player1.Name, player1.HP, player1.MaxHP, player1.Strength)
		fmt.Fprint(w, response)
		
		s.logCh <- fmt.Sprintf("PvP: Создан матч %s: %s vs %s", matchID, player1.Name, player.Name)
	} else {
		// Добавляем в очередь
		s.pvpQueue = append(s.pvpQueue, player)
		fmt.Fprint(w, "queued")
		s.logCh <- fmt.Sprintf("PvP: %s в очереди", player.Name)
	}
}

func (s *ChatServer) handlePvPStatus(w http.ResponseWriter, r *http.Request) {
	playerName := r.URL.Query().Get("player")
	
	// Проверяем, есть ли матч для игрока
	s.pvpMutex.RLock()
	for _, match := range s.pvpMatches {
		if (match.Player1.Name == playerName || match.Player2.Name == playerName) && match.Result == "" {
			// Определяем противника
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

	// Проверка завершения боя
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

	// Если оба хода сделаны и результаты ещё не отправлены, рассчитываем урон
	if match.Move1 != nil && match.Move2 != nil && (match.ResultForPlayer1 == "" && match.ResultForPlayer2 == "") {
		oldPlayer1HP := match.Player1HP
		oldPlayer2HP := match.Player2HP

		// Расчет урона
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

		s.logCh <- fmt.Sprintf("PvP Раунд %d: %s нанес %d (%d→%d), %s нанес %d (%d→%d)",
			match.Round,
			match.Player1.Name, damageToPlayer2, oldPlayer1HP, match.Player1HP,
			match.Player2.Name, damageToPlayer1, oldPlayer2HP, match.Player2HP,
		)

		// Формируем результаты для обоих игроков
		match.ResultForPlayer1 = fmt.Sprintf("round_result:%d|%d|%d|%d|%d|%d|%d",
			match.Round, damageToPlayer2, oldPlayer1HP, match.Player1HP, damageToPlayer1, oldPlayer2HP, match.Player2HP)
		match.ResultForPlayer2 = fmt.Sprintf("round_result:%d|%d|%d|%d|%d|%d|%d",
			match.Round, damageToPlayer1, oldPlayer2HP, match.Player2HP, damageToPlayer2, oldPlayer1HP, match.Player1HP)
	}

	// Отправка результата текущему игроку
	if match.Player1.Name == playerName && match.ResultForPlayer1 != "" {
		fmt.Fprint(w, match.ResultForPlayer1)
		match.ResultForPlayer1 = ""
	} else if match.Player2.Name == playerName && match.ResultForPlayer2 != "" {
		fmt.Fprint(w, match.ResultForPlayer2)
		match.ResultForPlayer2 = ""
	}

	// Сбрасываем ходы и увеличиваем раунд, если оба игрока получили результаты
	if match.ResultForPlayer1 == "" && match.ResultForPlayer2 == "" && match.Move1 != nil && match.Move2 != nil {
		match.Move1 = nil
		match.Move2 = nil
		match.Round++
		s.logCh <- fmt.Sprintf("PvP: Раунд %d готов, ждём новые ходы", match.Round)
		return
	}

	// Определяем, чей ход сейчас, если ходы не завершены
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

	// Формат: matchID|playerName|attack|block
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

	// Сохраняем ход
	if playerName == match.Player1.Name {
		if match.Move1 != nil {
			http.Error(w, "Move already submitted", http.StatusBadRequest)
			return
		}
		match.Move1 = &MoveData{Attack: attack, Block: block}
		s.logCh <- fmt.Sprintf("PvP: %s сделал ход (атака: %d, блок: %d)", playerName, attack, block)
	} else if playerName == match.Player2.Name {
		if match.Move2 != nil {
			http.Error(w, "Move already submitted", http.StatusBadRequest)
			return
		}
		match.Move2 = &MoveData{Attack: attack, Block: block}
		s.logCh <- fmt.Sprintf("PvP: %s сделал ход (атака: %d, блок: %d)", playerName, attack, block)
	} else {
		http.Error(w, "Invalid player", http.StatusBadRequest)
		return
	}

	// Если оба хода получены, просто логируем это
	if match.Move1 != nil && match.Move2 != nil {
		s.logCh <- fmt.Sprintf("PvP: Оба игрока сделали ход в раунде %d матча %s", match.Round, match.ID)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *ChatServer) processPvPRound(match *PvPMatch) {
    // Сохраняем здоровье ДО для логирования
    oldPlayer1HP := match.Player1HP
    oldPlayer2HP := match.Player2HP
    
    // Расчет урона для игрока 1 (от атаки игрока 2)
    damage1 := calculatePvPDamage(match.Player2.Strength, match.Move2.Attack, match.Move1.Block)
    match.Player1HP -= damage1
    if match.Player1HP < 0 {
        match.Player1HP = 0
    }

    // Расчет урона для игрока 2 (от атаки игрока 1)
    damage2 := calculatePvPDamage(match.Player1.Strength, match.Move1.Attack, match.Move2.Block)
    match.Player2HP -= damage2
    if match.Player2HP < 0 {
        match.Player2HP = 0
    }

    // Логирование с отображением изменения здоровья
    s.logCh <- fmt.Sprintf("PvP Раунд %d: %s нанес %d (%d❤️ → %d❤️), %s нанес %d (%d❤️ → %d❤️)", 
        match.Round, 
        match.Player1.Name, damage2, oldPlayer1HP, match.Player1HP,
        match.Player2.Name, damage1, oldPlayer2HP, match.Player2HP)

    // Сбрасываем ходы для следующего раунда
    match.Move1 = nil
    match.Move2 = nil
    match.Round++
}

func calculatePvPDamage(strength, attack, block int) int {
	// База: сила + случайный разброс
	damage := strength + 5
	
	// Модификаторы атаки
	switch attack {
	case 0: // Head
		damage = int(float64(damage) * 1.3)
	case 2: // Legs
		damage = int(float64(damage) * 0.8)
	}
	
	// Блок
	if attack == block {
		damage = damage / 2
	}
	
	// Минимальный урон
	if damage < 5 {
		damage = 5
	}
	
	return damage
}

func (s *ChatServer) HandlePvPChat(w http.ResponseWriter, r *http.Request) {
	matchID := r.URL.Query().Get("matchID")
	player := r.URL.Query().Get("player")
	msg := r.URL.Query().Get("msg")

	s.pvpMutex.RLock()
	match, ok := s.pvpMatches[matchID]
	s.pvpMutex.RUnlock()

	if ok {
		match.chatMutex.Lock()
		match.Chat = append(match.Chat, fmt.Sprintf("[%s]: %s", player, msg))
		// Ограничиваем историю чата для производительности
		if len(match.Chat) > 10 {
			match.Chat = match.Chat[1:]
		}
		match.chatMutex.Unlock()
		w.Write([]byte("ok"))
	}
}
func (s *ChatServer) handleCheckNick(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(r.URL.Query().Get("name"))

	s.nickMutex.Lock()
	defer s.nickMutex.Unlock()

	if s.registeredNicks[name] {
		fmt.Fprint(w, "exists")
		return
	}
	fmt.Fprint(w, "ok")
}

func (s *ChatServer) HandlePvPChatHistory(w http.ResponseWriter, r *http.Request) {
	matchID := r.URL.Query().Get("matchID")

	s.pvpMutex.RLock()
	match, ok := s.pvpMatches[matchID]
	s.pvpMutex.RUnlock()

	if !ok {
		return
	}

	match.chatMutex.Lock()
	defer match.chatMutex.Unlock()

	for _, msg := range match.Chat {
		fmt.Fprintln(w, msg)
	}
}

func (s *ChatServer) handleRegisterNick(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	name := strings.ToLower(strings.TrimSpace(string(body)))

	s.nickMutex.Lock()
	s.registeredNicks[name] = true
	s.nickMutex.Unlock()

	fmt.Fprint(w, "registered")
}
