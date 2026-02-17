package pvp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"game/player"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type PvPClient struct {
	serverURL     string
	httpClient    *http.Client
	matchID       string
	playerName    string
	running       bool
	chatLastCount int
	chatRunning   bool
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
	fmt.Println("\n=== БОЙ НАЧИНАЕТСЯ ===")
	c.chatRunning = true

	go c.sendPvPChat()
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
			c.chatRunning = false
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
			time.Sleep(2 * time.Second)
			continue
		}

		if strings.HasPrefix(status, "wait_turn:") {
			parts := strings.Split(status, ":")
			if len(parts) == 2 {
				turnPlayer := parts[1]

				if turnPlayer == c.playerName {
					// ===== ВАШ ХОД =====
					fmt.Println("\n⚔️ ВАШ ХОД!")
					fmt.Println("1 — Атаковать")
					fmt.Println("2 — Написать сообщение")

					reader := bufio.NewReader(os.Stdin)
					fmt.Print("Выберите: ")
					choiceRaw, _ := reader.ReadString('\n')
					choiceRaw = strings.TrimSpace(choiceRaw)

					if choiceRaw == "2" {
						fmt.Print("Введите сообщение: ")
						msg, _ := reader.ReadString('\n')
						msg = strings.TrimSpace(msg)

						if msg != "" {
							data := fmt.Sprintf("%s|%s|%s", c.matchID, c.playerName, msg)
							c.httpClient.Post(
								fmt.Sprintf("%s/pvp/chat/send", c.serverURL),
								"text/plain",
								strings.NewReader(data),
							)
						}
						continue
					}

					// Если выбрал атаковать
					attack := c.chooseHit()
					block := c.chooseBlock()

					moveData := fmt.Sprintf("%s|%s|%d|%d", c.matchID, c.playerName, attack, block)
					c.httpClient.Post(
						fmt.Sprintf("%s/pvp/move", c.serverURL),
						"text/plain",
						strings.NewReader(moveData),
					)

					fmt.Println("\n⏳ Ход отправлен...")
					time.Sleep(1 * time.Second)

				} else {
					// ===== ОЖИДАНИЕ =====
					fmt.Println("\n⏳ Ожидание хода противника...")
					fmt.Println("Вы можете писать сообщения.")

					reader := bufio.NewReader(os.Stdin)

					for {
						fmt.Print("💬 > ")
						text, _ := reader.ReadString('\n')
						text = strings.TrimSpace(text)

						if text == "" {
							continue
						}

						// Проверяем не настал ли наш ход
						checkResp, err := c.httpClient.Get(
							fmt.Sprintf("%s/pvp/battle?matchId=%s&player=%s",
								c.serverURL, c.matchID, c.playerName),
						)
						if err == nil {
							body, _ := io.ReadAll(checkResp.Body)
							checkResp.Body.Close()
							if strings.HasPrefix(string(body), "wait_turn:"+c.playerName) {
								break
							}
						}

						data := fmt.Sprintf("%s|%s|%s", c.matchID, c.playerName, text)
						c.httpClient.Post(
							fmt.Sprintf("%s/pvp/chat/send", c.serverURL),
							"text/plain",
							strings.NewReader(data),
						)
					}
				}
			}
		}

		time.Sleep(500 * time.Millisecond)

	}
	c.chatRunning = false
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

func (c *PvPClient) sendPvPChat() {
	reader := bufio.NewReader(os.Stdin)

	for c.chatRunning {
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)

		if text == "" {
			continue
		}

		data := fmt.Sprintf("%s|%s|%s", c.matchID, c.playerName, text)

		c.httpClient.Post(
			fmt.Sprintf("%s/pvp/chat/send", c.serverURL),
			"text/plain",
			strings.NewReader(data),
		)
	}
}
