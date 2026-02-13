package pvp

import (
	"bufio"
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
	serverURL  string
	httpClient *http.Client
	matchID    string
	playerName string
	running    bool
}

func NewPvPClient(serverURL string) *PvPClient {
	return &PvPClient{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Увеличиваем таймаут
		},
	}
}

func (c *PvPClient) Play(p *player.Player) string {
	c.playerName = p.Name
	c.running = true
	
	fmt.Println("\n=== ПОИСК PvP СОПЕРНИКА ===")
	
	// Формируем данные для отправки
	data := fmt.Sprintf("%s|%d|%d|%d", p.Name, p.HP, p.GetMaxHP(), p.GetStrength())
	
	// Отправляем запрос на поиск
	resp, err := c.httpClient.Post(c.serverURL+"/pvp/join", "text/plain", strings.NewReader(data))
	if err != nil {
		fmt.Println("❌ Ошибка подключения к PvP-серверу:", err)
		return "error"
	}
	defer resp.Body.Close()
	
	// Читаем ответ
	body, _ := io.ReadAll(resp.Body)
	response := string(body)
	
	if strings.HasPrefix(response, "queued") {
		fmt.Println("⏳ Ожидание противника... (нажмите Enter для отмены)")
		
		// Канал для отмены
		cancelCh := make(chan bool)
		go c.waitForCancel(cancelCh)
		
		// Ждем противника с возможностью отмены
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
					fmt.Printf("\n✅ ПРОТИВНИК НАЙДЕН!\n")
					fmt.Printf("👤 %s\n", opponent)
					matchFound = true
				} else {
					time.Sleep(1 * time.Second)
				}
			}
		}
	} else if strings.HasPrefix(response, "match:") {
		// Формат: match:ID|Имя|HP|MaxHP|Сила
		parts := strings.Split(response, ":")
		if len(parts) == 2 {
			matchParts := strings.Split(parts[1], "|")
			if len(matchParts) >= 5 {
				c.matchID = matchParts[0]
				opponentName := matchParts[1]
				opponentHP := matchParts[2]
				opponentMaxHP := matchParts[3]
				opponentStrength := matchParts[4]
				
				fmt.Printf("\n✅ ПРОТИВНИК НАЙДЕН!\n")
				fmt.Printf("👤 Имя: %s\n", opponentName)
				fmt.Printf("❤️ Здоровье: %s/%s\n", opponentHP, opponentMaxHP)
				fmt.Printf("⚔️ Сила: %s\n", opponentStrength)
			}
		}
	}
	
	// Начинаем бой
	return c.startBattle(p)
}

func (c *PvPClient) waitForCancel(cancelCh chan<- bool) {
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	cancelCh <- true
	c.running = false
}

func (c *PvPClient) checkMatchStatus() (string, string) {
	resp, err := c.httpClient.Get(c.serverURL + "/pvp/status?player=" + c.playerName)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	status := string(body)
	
	if strings.HasPrefix(status, "match:") {
		// Формат: match:ID|Имя противника|HP|MaxHP|Сила
		parts := strings.Split(status, ":")
		if len(parts) == 2 {
			matchParts := strings.Split(parts[1], "|")
			if len(matchParts) >= 2 {
				opponentInfo := fmt.Sprintf("%s (❤️ %s/%s, ⚔️ %s)", 
					matchParts[1], matchParts[2], matchParts[3], matchParts[4])
				return matchParts[0], opponentInfo
			}
		}
	}
	
	return "", ""
}

func (c *PvPClient) startBattle(p *player.Player) string {
	fmt.Println("\n=== БОЙ НАЧИНАЕТСЯ ===")
	fmt.Println("⚔️ Вводите свои ходы, когда придет ваша очередь")
	
	for c.running {
		// Получаем статус боя
		resp, err := c.httpClient.Get(c.serverURL + "/pvp/battle?matchId=" + c.matchID + "&player=" + c.playerName)
		if err != nil {
			fmt.Println("❌ Ошибка получения статуса боя")
			time.Sleep(1 * time.Second)
			continue
		}
		
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		status := string(body)
		
		// Проверяем завершение боя
		if strings.HasPrefix(status, "finished:") {
			parts := strings.Split(status, ":")
			if len(parts) == 2 {
				c.running = false
				if parts[1] == "win" {
					fmt.Println("\n🎉 ВЫ ПОБЕДИЛИ В PvP!")
					return "win"
				} else if parts[1] == "loss" {
					fmt.Println("\n💔 ВЫ ПРОИГРАЛИ В PvP.")
					return "loss"
				} else {
					fmt.Println("\n🤝 НИЧЬЯ В PvP!")
					return "draw"
				}
			}
		}
		
		// РЕЗУЛЬТАТ РАУНДА
		if strings.HasPrefix(status, "round_result:") {
			parts := strings.Split(status, ":")
			if len(parts) == 2 {
				resultData := strings.Split(parts[1], "|")
				if len(resultData) >= 7 {
					round, _ := strconv.Atoi(resultData[0])
					yourDamage, _ := strconv.Atoi(resultData[1])
					yourHPBefore, _ := strconv.Atoi(resultData[2])
					yourHPAfter, _ := strconv.Atoi(resultData[3])
					damageToYou, _ := strconv.Atoi(resultData[4])
					opponentHPBefore, _ := strconv.Atoi(resultData[5])
					opponentHPAfter, _ := strconv.Atoi(resultData[6])
					
					fmt.Printf("\n=== РЕЗУЛЬТАТ РАУНДА %d ===\n", round)
					fmt.Println("═══════════════════════════")
					fmt.Printf("💥 ВЫ нанесли: %d урона\n", yourDamage)
					fmt.Printf("💔 ВАМ нанесли: %d урона\n", damageToYou)
					fmt.Println("───────────────────────────")
					fmt.Printf("❤️ ВАШЕ здоровье: %d → %d (изменение: %d)\n", 
						yourHPBefore, yourHPAfter, yourHPAfter - yourHPBefore)
					fmt.Printf("❤️ Здоровье ПРОТИВНИКА: %d → %d (изменение: %d)\n", 
						opponentHPBefore, opponentHPAfter, opponentHPAfter - opponentHPBefore)
					fmt.Println("═══════════════════════════")
					
					// Обновляем здоровье игрока
					if yourHPAfter != p.HP {
						p.HP = yourHPAfter
					}
				}
			}
			
			fmt.Println("\n⏳ Подготовка к следующему раунду...")
			time.Sleep(2 * time.Second)
			continue
		}
		
		// Ожидание хода
		if strings.HasPrefix(status, "wait_turn:") {
			parts := strings.Split(status, ":")
			if len(parts) == 2 {
				turnPlayer := parts[1]
				if turnPlayer == c.playerName {
					// Наш ход
					fmt.Println("\n⚔️ ВАШ ХОД!")
					fmt.Printf("❤️ Ваше здоровье: %d/%d\n", p.HP, p.GetMaxHP())
					
					attack := c.chooseHit()
					block := c.chooseBlock()
					
					// Отправляем ход
					moveData := fmt.Sprintf("%s|%s|%d|%d", c.matchID, c.playerName, attack, block)
					c.httpClient.Post(c.serverURL+"/pvp/move", "text/plain", strings.NewReader(moveData))
					
					fmt.Println("\n⏳ Ожидание хода противника...")
					time.Sleep(1 * time.Second)
				} else {
					fmt.Printf("\n⏳ Ожидание хода %s...\n", turnPlayer)
					time.Sleep(2 * time.Second)
				}
			}
		}
		
		time.Sleep(500 * time.Millisecond)
	}
	
	return "error"
}
func (c *PvPClient) chooseHit() int {
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Println("\nКуда атаковать?")
		fmt.Println("1 — Голова (x1.3 урона, легко блокируется)")
		fmt.Println("2 — Тело (обычный урон)")
		fmt.Println("3 — Ноги (x0.8 урона, сложно блокировать)")
		fmt.Print("Выберите (1-3): ")
		
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		choice, err := strconv.Atoi(input)
		if err == nil && choice >= 1 && choice <= 3 {
			switch choice {
			case 1:
				return 0 // Head
			case 2:
				return 1 // Torso
			case 3:
				return 2 // Legs
			}
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
			switch choice {
			case 1:
				return 0 // Head
			case 2:
				return 1 // Torso
			case 3:
				return 2 // Legs
			}
		}
		fmt.Println("❌ Неверный ввод! Введите число 1, 2 или 3.")
	}
}