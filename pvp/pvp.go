package pvp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"game/player"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"sync"
)

type PvPClient struct {
	serverURL     string
	httpClient    *http.Client
	matchID       string
	playerName    string
	running       bool
	lastTurnOwner string
	inputCh       chan string
	done          chan struct{}
	chatOpen      bool
	chatLastCount int
	chatMu        sync.Mutex
}

func NewPvPClient(serverURL string) *PvPClient {
	serverURL = strings.TrimRight(serverURL, "/")
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	return &PvPClient{
		serverURL:  serverURL,
		httpClient: client,
		inputCh:    make(chan string),
		done:       make(chan struct{}),
	}
}

func (c *PvPClient) Play(p *player.Player) string {
	c.playerName = p.Name
	c.running = true
	fmt.Println("\n=== ПОИСК PvP СОПЕРНИКА ===")

	data := fmt.Sprintf("%s|%d|%d|%d", p.Name, p.HP, p.GetMaxHP(), p.GetStrength())
	resp, err := c.httpClient.Post(fmt.Sprintf("%s/pvp/join", c.serverURL), "text/plain", strings.NewReader(data))
	if err != nil {
		fmt.Println("❌ Ошибка подключения к PvP-серверу:", err)
		return "error"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	response := string(body)

	if strings.HasPrefix(response, "queued") {
		fmt.Println("⏳ Ожидание противника... (Enter для отмены)")
		cancelCh := make(chan bool)
		go c.waitForCancel(cancelCh)

		matchFound := false
		for !matchFound && c.running {
			select {
			case <-cancelCh:
				fmt.Println("\n❌ Поиск отменен")
				return "cancelled"
			default:
				matchID, opponent := c.checkMatchStatus()
				if matchID != "" {
					c.matchID = matchID
					fmt.Printf("\n✅ ПРОТИВНИК НАЙДЕН!\n%s\n", opponent)
					matchFound = true
				} else {
					time.Sleep(1 * time.Second)
				}
			}
		}
	} else if strings.HasPrefix(response, "match:") {
		parts := strings.Split(response, ":")
		if len(parts) == 2 {
			matchParts := strings.Split(parts[1], "|")
			if len(matchParts) >= 5 {
				c.matchID = matchParts[0]
				fmt.Printf("\n✅ ПРОТИВНИК НАЙДЕН!\n")
				fmt.Printf("👤 Имя: %s\n❤️ Здоровье: %s/%s\n⚔️ Сила: %s\n",
					matchParts[1], matchParts[2], matchParts[3], matchParts[4])
			}
		}
	}

	result := c.startBattle(p)

	// c.matchID = ""        // сбрасываем ТОЛЬКО после выхода
	// c.lastTurnOwner = ""  // заодно
	c.running = false

	return result
}

func (c *PvPClient) waitForCancel(cancelCh chan<- bool) {
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	cancelCh <- true
	c.running = false
}

func (c *PvPClient) checkMatchStatus() (string, string) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/pvp/status?player=%s", c.serverURL, c.playerName))
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	status := string(body)

	if strings.HasPrefix(status, "match:") {
		parts := strings.Split(status, ":")
		if len(parts) == 2 {
			matchParts := strings.Split(parts[1], "|")
			if len(matchParts) >= 2 {
				info := fmt.Sprintf("%s (❤️ %s/%s, ⚔️ %s)",
					matchParts[1], matchParts[2], matchParts[3], matchParts[4])
				return matchParts[0], info
			}
		}
	}
	return "", ""
}

func (c *PvPClient) startBattle(p *player.Player) string {
	c.done = make(chan struct{})
	c.chatLastCount = 0
	c.startChatListener()
	c.startInputListener()

	fmt.Println("\n=== БОЙ НАЧИНАЕТСЯ ===")
	var isMyTurn bool

	for c.running {
		resp, err := c.httpClient.Get(fmt.Sprintf("%s/pvp/battle?matchId=%s&player=%s",
			c.serverURL, c.matchID, c.playerName))
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			fmt.Println("\n⚠️ Матч больше не существует. Бой завершён.")
			c.running = false
			close(c.done)
			return "error"
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		status := string(body)

		if strings.HasPrefix(status, "finished:") {
			c.running = false
			close(c.done)

			result := "error"
			parts := strings.Split(status, ":")
			if len(parts) == 2 {
				result = parts[1]
			}

			c.matchID = ""
			c.lastTurnOwner = ""

			return result
		}

		if p.HP <= 0 {
			c.running = false
			close(c.done)
			fmt.Println("\n💀 Вы погибли. Бой завершен.")
			return "loss"
		}

		if strings.HasPrefix(status, "round_result:") {
			c.printRoundResult(status, p)
			time.Sleep(300 * time.Millisecond)
			continue
		}

		if strings.HasPrefix(status, "wait_turn:") {
			parts := strings.Split(status, ":")
			if len(parts) == 2 {
				turnPlayer := parts[1]
				isMyTurn = turnPlayer == c.playerName
				if c.lastTurnOwner != turnPlayer {
					c.lastTurnOwner = turnPlayer
					if isMyTurn {
						fmt.Println("\n⚔️ ВАШ ХОД!")
						fmt.Println("1 — Открыть чат")
						fmt.Println("2 — Атака")
						fmt.Println("3 — Использовать предмет")
						fmt.Println("4 — Инвентарь")
						fmt.Print("> ")
					} else {
						fmt.Printf("\n⏳ Ожидание хода %s...\n", turnPlayer)
						fmt.Println("1 — Открыть чат")
						fmt.Println("4 — Инвентарь")
						fmt.Print("> ")
					}
				}
			}
		}

		select {
		case input := <-c.inputCh:
			if input == "/exit" {
				c.StopBattle()
				return "exit"
			}
			c.handleInput(input, isMyTurn, p)
		default:
		}

		time.Sleep(200 * time.Millisecond)
	}

	return "error"
}

func (c *PvPClient) printRoundResult(status string, p *player.Player) {
	parts := strings.Split(status, ":")
	if len(parts) != 2 {
		return
	}
	data := strings.Split(parts[1], "|")
	if len(data) < 7 {
		return
	}

	round, _ := strconv.Atoi(data[0])
	yourDamage, _ := strconv.Atoi(data[1])
	yourHPBefore, _ := strconv.Atoi(data[2])
	yourHPAfter, _ := strconv.Atoi(data[3])
	damageToYou, _ := strconv.Atoi(data[4])
	opponentHPBefore, _ := strconv.Atoi(data[5])
	opponentHPAfter, _ := strconv.Atoi(data[6])

	fmt.Printf("\n=== РЕЗУЛЬТАТ РАУНДА %d ===\n", round)
	fmt.Println("═══════════════════════════")
	fmt.Printf("💥 ВЫ нанесли: %d урона\n", yourDamage)
	fmt.Printf("💔 ВАМ нанесли: %d урона\n", damageToYou)
	fmt.Println("───────────────────────────")
	fmt.Printf("❤️ ВАШЕ здоровье: %d → %d\n", yourHPBefore, yourHPAfter)
	fmt.Printf("❤️ Здоровье ПРОТИВНИКА: %d → %d\n", opponentHPBefore, opponentHPAfter)
	fmt.Println("═══════════════════════════")

	p.HP = yourHPAfter
}

func (c *PvPClient) chooseHit() int {
	fmt.Println("\nКуда атаковать?")
	fmt.Println("1 — Голова\n2 — Тело\n3 — Ноги")
	for {
		select {
		case <-c.done:
			return -1
		case input := <-c.inputCh:
			if choice, err := strconv.Atoi(input); err == nil && choice >= 1 && choice <= 3 {
				return choice - 1
			}
			fmt.Println("❌ Введите 1-3")
		}
	}
}

func (c *PvPClient) chooseBlock() int {
	fmt.Println("\nЧто защищать?")
	fmt.Println("1 — Голова\n2 — Тело\n3 — Ноги")
	for {
		select {
		case <-c.done:
			return -1
		case input := <-c.inputCh:
			if choice, err := strconv.Atoi(input); err == nil && choice >= 1 && choice <= 3 {
				return choice - 1
			}
			fmt.Println("❌ Введите 1-3")
		}
	}
}

func (c *PvPClient) sendChat(playerName, message string) {
	endpoint := fmt.Sprintf("%s/pvp/chat?matchID=%s&player=%s&msg=%s",
		c.serverURL, c.matchID, url.QueryEscape(playerName), url.QueryEscape(message))
	c.httpClient.Get(endpoint)
}

func (c *PvPClient) openPvPChat() {
	c.chatMu.Lock()
	c.chatOpen = true
	c.chatMu.Unlock()

	defer func() {
		c.chatMu.Lock()
		c.chatOpen = false
		c.chatMu.Unlock()
		fmt.Println("\n=== ЧАТ ЗАКРЫТ ===")
	}()

	fmt.Println("\n=== PvP ЧАТ ===")
	fmt.Println("Введите /back для возврата")

	// Показываем историю
	resp, err := c.httpClient.Get(fmt.Sprintf(
		"%s/pvp/chat/history?matchID=%s",
		c.serverURL, c.matchID,
	))
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		raw := strings.TrimSpace(string(body))
		if raw != "" {
			fmt.Println("\n--- ИСТОРИЯ ЧАТА ---")
			fmt.Println(raw)

			lines := strings.Split(raw, "\n")

			c.chatMu.Lock()
			c.chatLastCount = len(lines)
			c.chatMu.Unlock()
		}
	}

	fmt.Print("> ")

	for c.running {
		input := <-c.inputCh

		switch input {
		case "/back":
			return

		case "/exit":
			c.StopBattle()
			return

		default:
			if strings.TrimSpace(input) != "" {
				c.sendChat(c.playerName, input)
			}
		}

		fmt.Print("> ")
	}
}

func (c *PvPClient) useItemInBattle(p *player.Player) {
	if len(p.Inventory) == 0 {
		fmt.Println("Инвентарь пуст.")
		return
	}

	p.ShowInventory()
	fmt.Print("Введите номер предмета: ")
	input := <-c.inputCh
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(p.Inventory) {
		fmt.Println("❌ Неверный номер!")
		return
	}
	p.UseItem(idx - 1)
}

func (c *PvPClient) startInputListener() {
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for c.running {
			text, err := reader.ReadString('\n')
			if err != nil {
				continue
			}
			c.inputCh <- strings.TrimSpace(text)
		}
	}()
}

func (c *PvPClient) handleInput(input string, isMyTurn bool, p *player.Player) {
	if !c.running {
		fmt.Println("❌ Бой завершён, нельзя вводить команды")
		return
	}

	switch input {
	case "1":
		c.openPvPChat()
	case "2":
		if !isMyTurn {
			fmt.Println("❌ Сейчас не ваш ход!")
			return
		}
		attack := c.chooseHit()
		if attack == -1 {
			return
		}
		block := c.chooseBlock()
		if block == -1 {
			return
		}
		moveData := fmt.Sprintf("%s|%s|%d|%d", c.matchID, c.playerName, attack, block)
		c.httpClient.Post(fmt.Sprintf("%s/pvp/move", c.serverURL), "text/plain", strings.NewReader(moveData))
	case "3":
		if !isMyTurn {
			fmt.Println("❌ Сейчас не ваш ход!")
			return
		}
		c.useItemInBattle(p)
	case "4":
		p.ShowInventory()
	default:
		fmt.Println("❌ Неверная команда")
	}
}

func (c *PvPClient) startChatListener() {
	go func() {
		for c.running {
			resp, err := c.httpClient.Get(fmt.Sprintf(
				"%s/pvp/chat/history?matchID=%s",
				c.serverURL, c.matchID,
			))
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			raw := strings.TrimSpace(string(body))
			if raw == "" {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			lines := strings.Split(raw, "\n")

			c.chatMu.Lock()

			if len(lines) > c.chatLastCount {
				for i := c.chatLastCount; i < len(lines); i++ {
					line := strings.TrimSpace(lines[i])
					if line == "" {
						continue
					}

					if c.chatOpen {
						fmt.Printf("\r\033[K%s\n> ", line)
					} else {
						fmt.Printf("\n💬 %s\n> ", line)
					}
				}
				c.chatLastCount = len(lines)
			}

			c.chatMu.Unlock()

			time.Sleep(500 * time.Millisecond)
		}
	}()
}

func (c *PvPClient) StopBattle() {
	if c.running {
		c.running = false
		close(c.done)
		fmt.Println("\n❌ Вы покинули бой PvP.")
	}
}
