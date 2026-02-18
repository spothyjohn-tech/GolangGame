package pvp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"game/player"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type PvPClient struct {
	serverURL    string
	httpClient   *http.Client
	matchID      string
	playerName   string
	running      bool
	inChat       bool
	lastMsgCount int
	menuShown    bool
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
	}
}

func ValidateName(name string) bool {
	if len(name) == 0 {
		return false
	}
	match, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", name)
	return match
}

func (c *PvPClient) Play(p *player.Player) string {
	if !ValidateName(p.Name) {
		fmt.Println("\n❌ Для PvP нужно английское имя (только буквы, цифры и _)")
		fmt.Println("Ваше текущее имя:", p.Name)
		fmt.Print("Введите имя для PvP (или Enter для отмены): ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			return "cancelled"
		}

		if !ValidateName(input) {
			fmt.Println("❌ Недопустимое имя")
			return "cancelled"
		}

		p.Name = input
	}

	c.playerName = p.Name
	c.running = true
	c.lastMsgCount = 0
	c.menuShown = false

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

	if strings.Contains(response, "already in queue") || strings.Contains(response, "already in match") {
		fmt.Println("❌ Игрок с таким ником уже в очереди или в бою")
		return "cancelled"
	}

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
				fmt.Printf("👤 %s\n❤️ %s/%s\n⚔️ %s\n",
					matchParts[1], matchParts[2], matchParts[3], matchParts[4])
			}
		}
	}

	go c.receiveChatMessages()
	return c.startBattle(p)
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
	fmt.Println("\n=== БОЙ ===")
	if !c.menuShown {
		fmt.Println("1 - атака, 2 - чат")
		c.menuShown = true
	}

	for c.running {
		resp, err := c.httpClient.Get(fmt.Sprintf("%s/pvp/battle?matchId=%s&player=%s", c.serverURL, c.matchID, c.playerName))
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		status := string(body)

		if strings.HasPrefix(status, "finished:") {
			c.running = false
			parts := strings.Split(status, ":")
			if len(parts) == 2 {
				switch parts[1] {
				case "win":
					fmt.Println("\n🎉 ПОБЕДА")
					return "win"
				case "loss":
					fmt.Println("\n💔 ПОРАЖЕНИЕ")
					return "loss"
				default:
					fmt.Println("\n🤝 НИЧЬЯ")
					return "draw"
				}
			}
		}

		if strings.HasPrefix(status, "round_result:") {
			c.printRoundResult(status, p)
			continue
		}

		if strings.HasPrefix(status, "wait_turn:") {
			parts := strings.Split(status, ":")
			if len(parts) == 2 {
				if parts[1] == c.playerName {
					c.handlePlayerTurn(p)
				} else {
					c.waitForTurnInput()
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "error"
}

func (c *PvPClient) handlePlayerTurn(p *player.Player) {
	fmt.Printf("\n⚔️ %d/%d\n", p.HP, p.GetMaxHP())

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\nНажмите 1 - для открытия чата")
		fmt.Println("Нажмите 2 - для атаки")
		fmt.Print("> ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			c.openChat()
		case "2":
			attack := c.chooseHit()
			block := c.chooseBlock()

			moveData := fmt.Sprintf("%s|%s|%d|%d",
				c.matchID, c.playerName, attack, block)

			c.httpClient.Post(
				fmt.Sprintf("%s/pvp/move", c.serverURL),
				"text/plain",
				strings.NewReader(moveData),
			)

			fmt.Println("\n⏳ Ожидание противника, вы можете написать сообщение и нажать enter для отправки:")
			c.waitWithChat()
			return
		}
	}
}


func (c *PvPClient) waitForTurnInput() {
	reader := bufio.NewReader(os.Stdin)
	go func() {
		ch := make(chan string)
		go func() {
			input, _ := reader.ReadString('\n')
			ch <- strings.TrimSpace(input)
		}()
		select {
		case input := <-ch:
			if input == "2" {
				c.openChat()
			}
		case <-time.After(100 * time.Millisecond):
		}
	}()
}

func (c *PvPClient) openChat() {
	if c.inChat {
		return
	}
	c.inChat = true
	fmt.Println("\n=== ЧАТ ===")
	fmt.Println("/back - выход")

	reader := bufio.NewReader(os.Stdin)
	for c.running && c.inChat {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "/back" {
			c.inChat = false
			fmt.Println("---")
			break
		}
		if input != "" {
			chatMsg := fmt.Sprintf("chat:%s|%s|%s", c.matchID, c.playerName, input)
			c.httpClient.Post(fmt.Sprintf("%s/pvp/chat", c.serverURL), "text/plain", strings.NewReader(chatMsg))
		}
	}
}

func (c *PvPClient) receiveChatMessages() {
	for c.running {
		resp, err := c.httpClient.Get(fmt.Sprintf("%s/pvp/chat?matchId=%s&player=%s&last=%d",
			c.serverURL, c.matchID, c.playerName, c.lastMsgCount))
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		messages := strings.TrimSpace(string(body))
		if messages != "" {
			lines := strings.Split(messages, "\n")
			for _, msg := range lines {
				if msg != "" {
					if c.inChat {
						fmt.Print("\r\033[K")
						fmt.Println(msg)
						fmt.Print("> ")
					} else {
						fmt.Printf("\r\033[K[%s]\n", msg)
					}
					c.lastMsgCount++
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
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

	fmt.Printf("\n=== РАУНД %d ===\n", round)
	fmt.Printf("💥 %d урона\n", yourDamage)
	fmt.Printf("💔 -%d\n", damageToYou)
	fmt.Printf("❤️ %d→%d\n", yourHPBefore, yourHPAfter)
	fmt.Printf("Враг: %d→%d\n", opponentHPBefore, opponentHPAfter)

	if yourHPAfter != p.HP {
		p.HP = yourHPAfter
	}
}

func (c *PvPClient) chooseHit() int {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\nКуда атаковать?")
		fmt.Println("1 — Голова (x1.3 урона)")
		fmt.Println("2 — Тело (обычный урон)")
		fmt.Println("3 — Ноги (x0.8 урона)")
		fmt.Print("Выберите (1-3): ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		choice, err := strconv.Atoi(input)
		if err == nil && choice >= 1 && choice <= 3 {
			return choice - 1
		}
		fmt.Println("❌ Неверный ввод! Введите число 1, 2 или 3.")
	}
}

func (c *PvPClient) chooseBlock() int {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\nЧто защищать?")
		fmt.Println("1 — Голову")
		fmt.Println("2 — Тело")
		fmt.Println("3 — Ноги")
		fmt.Print("Выберите (1-3): ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		choice, err := strconv.Atoi(input)
		if err == nil && choice >= 1 && choice <= 3 {
			return choice - 1
		}
		fmt.Println("❌ Неверный ввод! Введите число 1, 2 или 3.")
	}
}

func (c *PvPClient) waitWithChat() {
	reader := bufio.NewReader(os.Stdin)

	for c.running {
		resp, err := c.httpClient.Get(fmt.Sprintf(
			"%s/pvp/battle?matchId=%s&player=%s",
			c.serverURL, c.matchID, c.playerName))

		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			status := string(body)

			if strings.HasPrefix(status, "round_result:") ||
				strings.HasPrefix(status, "finished:") {
				return
			}
		}

		fmt.Print("> ")
		inputCh := make(chan string)

		go func() {
			text, _ := reader.ReadString('\n')
			inputCh <- strings.TrimSpace(text)
		}()

		select {
		case text := <-inputCh:
			if text != "" {
				chatMsg := fmt.Sprintf("chat:%s|%s|%s",
					c.matchID, c.playerName, text)

				c.httpClient.Post(
					fmt.Sprintf("%s/pvp/chat", c.serverURL),
					"text/plain",
					strings.NewReader(chatMsg),
				)
			}
		case <-time.After(1 * time.Second):
		}
	}
}
