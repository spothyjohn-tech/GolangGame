package main

import (
	"bufio"
	"fmt"
	"game/client"
	"game/player"
	"game/shop"
	"game/tournament"
	"game/pvp"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	fmt.Println("=== ДОБРО ПОЖАЛОВАТЬ В ВООБРАЖАРИУМ ===")
	fmt.Print("Введите имя вашего Хранителя: ")
	
	reader := bufio.NewReader(os.Stdin)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	
	if name == "" {
		name = "Хранитель"
	}
	
	fmt.Printf("\nПриветствую, %s!\n", name)
	fmt.Println("Ваше путешествие начинается...\n")
	
	// Создаем игрока
	p := player.NewPlayer(name)
	
	// Создаем магазин
	shopInstance := shop.NewShop()
	
	// Создаем турнир
	tournamentInstance := tournament.NewTournament(p)
	
	// Добавляем стартовые предметы (убираем вызов items.GetAllItems)
	// Вместо этого добавим базовые предметы через магазин позже
	fmt.Println("💰 Вам выдано 150 воображения для стартовых покупок!")
	
	for {
		showMainMenu(p, tournamentInstance)
		
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
			// Бой с гильдией
			if tournamentInstance.IsComplete() && !tournamentInstance.IsFinalDefeated() {
				fmt.Println("\n👾 Вы победили все гильдии! Пора бросить вызов Древнему Хаосу!")
				fmt.Print("Начать финальный бой? (да/нет): ")
				confirm, _ := reader.ReadString('\n')
				confirm = strings.TrimSpace(strings.ToLower(confirm))
				if confirm == "да" || confirm == "д" || confirm == "yes" {
					p.ResetForBattle()
					tournamentInstance.StartFinalBoss()
				}
			} else if tournamentInstance.CurrentGuild < len(tournamentInstance.Guilds) {
				p.ResetForBattle()
				tournamentInstance.StartNextGuild()
			} else if tournamentInstance.IsFinalDefeated() {
				fmt.Println("\n🏆 Вы уже победили Древнего Хаоса! Игра пройдена! 🏆")
			} else {
				fmt.Println("\n❌ Нет доступных гильдий для битвы")
			}
			
		case 2:
			// PvP
			fmt.Println("\n=== PvP РЕЖИМ ===")
			fmt.Println("Подключение к серверу localhost:8080...")
			
			pvpClient := pvp.NewPvPClient("https://fluffy-space-spork-wrjqg7qq9p64fv9p5-8080.app.github.dev/")
			result := pvpClient.Play(p)
			
			if result == "loss" {
				p.AddImagination(50)
				fmt.Println("✨ За участие в PvP вы получили 50 воображения!")
			} else if result == "win" {
				p.AddImagination(100)
				p.Wins++
				fmt.Println("✨ За победу в PvP вы получили 100 воображения!")
			}
			
		case 3:
			// Чат
			fmt.Println("\n=== ЧАТ ===")
			fmt.Println("Подключение к чат-серверу localhost:8080...")
			
			// Создаем клиент
			chatClient := client.NewChatClient("https://fluffy-space-spork-wrjqg7qq9p64fv9p5-8080.app.github.dev/")
			
			// Запускаем чат (он БЛОКИРУЕТ выполнение до выхода)
			chatClient.Start()
			
			// После выхода из чата показываем меню снова
			fmt.Println("\n🔙 Возврат в главное меню...")
			// Небольшая пауза для читаемости
			time.Sleep(1 * time.Second)
			
		case 4:
			// Магазин
			shopInstance.Visit(p)
			
		case 5:
			// Инвентарь / Экипировка
			manageInventory(p, reader)
			
		case 6:
			// Показать прогресс
			tournamentInstance.ShowProgress()
			
		case 0:
			fmt.Println("Выход из игры...")
			return
			
		default:
			fmt.Println("Неверный выбор!")
		}
	}
}

func showMainMenu(p *player.Player, t *tournament.Tournament) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("ГЛАВНОЕ МЕНЮ")
	fmt.Println(strings.Repeat("=", 50))
	
	p.ShowStats()
	
	fmt.Println("\nДОСТУПНЫЕ ДЕЙСТВИЯ:")
	fmt.Println("1. Бой с гильдией")
	fmt.Println("2. PvP (игрок против игрока)")
	fmt.Println("3. Чат")
	fmt.Println("4. Магазин")
	fmt.Println("5. Инвентарь / Экипировка")
	fmt.Println("6. Прогресс турнира")
	fmt.Println("0. Выход")
}

func manageInventory(p *player.Player, reader *bufio.Reader) {
	for {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("УПРАВЛЕНИЕ ИНВЕНТАРЕМ")
		fmt.Println(strings.Repeat("=", 50))
		
		fmt.Println("1. Показать инвентарь")
		fmt.Println("2. Экипировать предмет")
		fmt.Println("3. Снять предмет")
		fmt.Println("4. Использовать предмет (вне боя)")
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