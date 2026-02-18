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
)

type PvPClient struct {
	serverURL       string
	httpClient      *http.Client
	matchID         string
	playerName      string
	running         bool
	lastTurnMessage string
}

func NewPvPClient(serverURL string) *PvPClient {
	// Убираем слеш в конце, если есть
	serverURL = strings.TrimRight(serverURL, "/")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			// Для dev-среды с самоподписанными сертификатами
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	return &PvPClient{
		serverURL:  serverURL,
		httpClient: client,
	}
}

func (c *PvPClient) Play(p *player.Player) string {
	c.playerName = p.Name
	c.running = true

	fmt.Println("\n=== ПОИСК PvP СОПЕРНИКА ===")

	// Формируем данные
	data := fmt.Sprintf("%s|%d|%d|%d", p.Name, p.HP, p.GetMaxHP(), p.GetStrength())

	// Отправляем запрос на поиск
	resp, err := c.httpClient.Post(fmt.Sprintf("%s/pvp/join", c.serverURL), "text/plain", strings.NewReader(data))
	if err != nil {
		fmt.Println("❌ Ошибка подключения к PvP-серверу:", err)
		return "error"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	response := string(body)

	if strings.HasPrefix(response, "queued") {
		fmt.Println("⏳ Ожидание противника... (нажмите Enter для отмены)")

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
	c.lastTurnMessage = ""
	fmt.Println("\n=== БОЙ НАЧИНАЕТСЯ ===")

	for c.running {
		resp, err := c.httpClient.Get(fmt.Sprintf("%s/pvp/battle?matchId=%s&player=%s", c.serverURL, c.matchID, c.playerName))
		if err != nil {
			fmt.Println("❌ Ошибка получения статуса боя")
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
					fmt.Println("\n🎉 ВЫ ПОБЕДИЛИ В PvP!")
					return "win"
				case "loss":
					fmt.Println("\n💔 ВЫ ПРОИГРАЛИ В PvP.")
					return "loss"
				default:
					fmt.Println("\n🤝 НИЧЬЯ В PvP!")
					return "draw"
				}
			}
		}

		if strings.HasPrefix(status, "round_result:") {
			c.printRoundResult(status, p)
			time.Sleep(1 * time.Second)
			continue
		}

		if strings.HasPrefix(status, "wait_turn:") {
			parts := strings.Split(status, ":")
				if len(parts) == 2 {
					turnPlayer := parts[1]

					isMyTurn := turnPlayer == c.playerName

					if isMyTurn {
						c.lastTurnMessage = ""
						fmt.Println("\n⚔️ ВАШ ХОД!")
					} else {
						msg := fmt.Sprintf("⏳ Ожидание хода %s...", turnPlayer)
						if c.lastTurnMessage != msg {
							fmt.Println("\n" + msg)
							c.lastTurnMessage = msg
						}
					}
					
					action := c.chooseAction(isMyTurn)

					switch action {

					case 1:
						c.openPvPChat()
						continue

					case 2:
						if !isMyTurn {
							fmt.Println("❌ Сейчас не ваш ход!")
							continue
						}

						attack := c.chooseHit()
						block := c.chooseBlock()

						moveData := fmt.Sprintf("%s|%s|%d|%d",
							c.matchID, c.playerName, attack, block)

						c.httpClient.Post(fmt.Sprintf("%s/pvp/move", c.serverURL),
							"text/plain",
							strings.NewReader(moveData))

					case 3:
						if !isMyTurn {
							fmt.Println("❌ Предмет можно использовать только в свой ход!")
							continue
						}
						c.useItemInBattle(p)

					case 4:
						p.ShowInventory()
					}
				} 
		}

		time.Sleep(500 * time.Millisecond)
		
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

func (c *PvPClient) sendChat(playerName, message string) {
	endpoint := fmt.Sprintf("%s/pvp/chat?matchID=%s&player=%s&msg=%s",
		c.serverURL, c.matchID, url.QueryEscape(playerName), url.QueryEscape(message))
	c.httpClient.Get(endpoint)
}

func (c *PvPClient) chooseAction(isMyTurn bool) int {
	reader := bufio.NewReader(os.Stdin)

	for {

		fmt.Println("\n1 — Открыть чат")

		if isMyTurn {
			fmt.Println("2 — Атака")
			fmt.Println("3 — Использовать предмет")
		}

		fmt.Println("4 — Инвентарь")
		fmt.Print("Выберите действие: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("❌ Неверный ввод!")
			continue
		}

		if isMyTurn {
			if choice >= 1 && choice <= 4 {
				return choice
			}
		} else {
			if choice == 1 || choice == 4 {
				return choice
			}
		}

		fmt.Println("❌ Недоступное действие!")
	}
}


func (c *PvPClient) openPvPChat() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== PvP ЧАТ ===")
	fmt.Println("Введите /back для возврата")

	for {
		// Получаем историю 1 раз
		resp, err := c.httpClient.Get(
			fmt.Sprintf("%s/pvp/chat/history?matchID=%s",
				c.serverURL, c.matchID))

		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			fmt.Println("\n--- ЧАТ ---")
			fmt.Print(string(body))
		}

		fmt.Print("> ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)

		if text == "/back" {
			return
		}

		if text != "" {
			c.sendChat(c.playerName, text)
		}
	}
}


func manageInventory(p *player.Player, reader *bufio.Reader) {
	for {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("УПРАВЛЕНИЕ ИНВЕНТАРЕМ")
		fmt.Println(strings.Repeat("=", 50))

		fmt.Println("1. Показать инвентарь")
		fmt.Println("2. Экипировать предмет")
		fmt.Println("3. Снять предмет")
		fmt.Println("4. Использовать предмет")
		fmt.Println("5. Назад")
		fmt.Print("Выберите действие: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Неверный ввод!")
			continue
		}

		switch choice {
		case 1:
			p.ShowInventory()

		case 2:
			p.ShowInventory()
			if len(p.Inventory) == 0 {
				fmt.Println("Нет предметов для экипировки.")
				continue
			}
			fmt.Print("Введите номер предмета для экипировки: ")
			idxInput, _ := reader.ReadString('\n')
			idxInput = strings.TrimSpace(idxInput)
			idx, _ := strconv.Atoi(idxInput)
			if idx >= 1 && idx <= len(p.Inventory) {
				p.EquipItem(idx - 1)
			} else {
				fmt.Println("Неверный номер!")
			}

		case 3:
			if len(p.Equipped) == 0 {
				fmt.Println("Нет надетых предметов.")
				continue
			}
			fmt.Println("\n=== НАДЕТО ===")
			for i, item := range p.Equipped {
				fmt.Printf("%d. %s\n", i+1, item.Name)
			}
			fmt.Print("Введите номер предмета для снятия: ")
			idxInput, _ := reader.ReadString('\n')
			idxInput = strings.TrimSpace(idxInput)
			idx, _ := strconv.Atoi(idxInput)
			if idx >= 1 && idx <= len(p.Equipped) {
				p.UnequipItem(idx - 1)
			} else {
				fmt.Println("Неверный номер!")
			}

		case 4:
			p.ShowInventory()
			if len(p.Inventory) == 0 {
				fmt.Println("Нет предметов для использования.")
				continue
			}
			fmt.Print("Введите номер предмета для использования: ")
			idxInput, _ := reader.ReadString('\n')
			idxInput = strings.TrimSpace(idxInput)
			idx, _ := strconv.Atoi(idxInput)
			if idx >= 1 && idx <= len(p.Inventory) {
				p.UseItem(idx - 1)
			} else {
				fmt.Println("Неверный номер!")
			}

		case 5:
			return
		}
	}
}

func (c *PvPClient) useItemInBattle(p *player.Player) {
	reader := bufio.NewReader(os.Stdin)

	if len(p.Inventory) == 0 {
		fmt.Println("Инвентарь пуст.")
		return
	}

	p.ShowInventory()
	fmt.Print("Введите номер предмета для использования: ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(p.Inventory) {
		fmt.Println("❌ Неверный номер!")
		return
	}

	p.UseItem(idx - 1)
}
