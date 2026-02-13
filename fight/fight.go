package fight

import (
	"bufio"
	"fmt"
	"game/boss"
	"game/combat"
	"game/player"
	"math/rand"
	"os"
	"strings"
	"strconv"
)

type Fight struct {
	Player      *player.Player
	Boss        *boss.Boss
	Round       int
}

func NewFight(p *player.Player, b *boss.Boss) *Fight {
	return &Fight{
		Player: p,
		Boss:   b,
		Round:  0,
	}
}

func (f *Fight) Start() bool {
	fmt.Printf("\n⚔️ БИТВА С %s ⚔️\n", f.Boss.GetName())
	if f.Boss.Description != "" {
		fmt.Printf("%s\n", f.Boss.Description)
	}
	fmt.Printf("❤️ Здоровье врага: %d/%d\n", f.Boss.HP, f.Boss.MaxHP)
	fmt.Printf("⚔️ Сила врага: %d\n", f.Boss.Strength)
	
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nНажмите Enter, чтобы начать бой...")
	reader.ReadString('\n')
	
	for f.Boss.IsAlive() && f.Player.IsAlive() {
		f.Round++
		fmt.Printf("\n%s РАУНД %d %s\n", strings.Repeat("=", 10), f.Round, strings.Repeat("=", 10))
		
		// Ход игрока
		playerAction := f.playerTurn()
		
		// Ход босса
		bossAction, bossDamage := f.Boss.ChooseAttack()
		
		// Применяем результаты
		f.applyRound(playerAction, bossAction, bossDamage)
		
		// Показываем статус
		f.showStatus()
		
		if !f.Player.IsAlive() || !f.Boss.IsAlive() {
			break
		}
		
		fmt.Print("\nНажмите Enter для следующего раунда...")
		reader.ReadString('\n')
	}
	
	if f.Boss.IsAlive() == false {
		fmt.Printf("\n🏆 ПОБЕДА! Вы победили %s! 🏆\n", f.Boss.GetName())
		return true
	} else {
		fmt.Printf("\n💔 ПОРАЖЕНИЕ! Вы проиграли %s... 💔\n", f.Boss.GetName())
		return false
	}
}

func (f *Fight) playerTurn() combat.BodyPart {
	fmt.Printf("\n❤️ Ваше здоровье: %d/%d\n", f.Player.HP, f.Player.GetMaxHP())
	fmt.Printf("⚔️ Ваша сила: %d\n", f.Player.GetStrength())
	
	fmt.Println("\n⚔️ ВЫБЕРИТЕ ДЕЙСТВИЕ:")
	fmt.Println("1 — Атаковать")
	fmt.Println("2 — Использовать предмет")
	fmt.Println("3 — Показать инвентарь")
	
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	
	switch input {
	case "1":
		return f.chooseHit()
	case "2":
		return f.useItem()
	case "3":
		f.Player.ShowInventory()
		return f.playerTurn()
	default:
		fmt.Println("Неверный ввод, выбираю атаку")
		return f.chooseHit()
	}
}

func (f *Fight) chooseHit() combat.BodyPart {
	fmt.Println("\n⚔️ КУДА АТАКОВАТЬ:")
	fmt.Println("1 — Голова (x1.3 урона, легко блокируется)")
	fmt.Println("2 — Тело (обычный урон)")
	fmt.Println("3 — Ноги (x0.8 урона, сложно блокировать)")
	
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	
	switch input {
	case "1":
		return combat.Head
	case "2":
		return combat.Torso
	case "3":
		return combat.Legs
	default:
		return combat.Torso
	}
}

func (f *Fight) chooseBlock() combat.BodyPart {
	fmt.Println("\n🛡️ КАК ЗАЩИЩАТЬСЯ:")
	fmt.Println("1 — Защитить голову")
	fmt.Println("2 — Защитить тело")
	fmt.Println("3 — Защитить ноги")
	
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	
	switch input {
	case "1":
		return combat.Head
	case "2":
		return combat.Torso
	case "3":
		return combat.Legs
	default:
		return combat.Torso
	}
}

func (f *Fight) useItem() combat.BodyPart {
	f.Player.ShowInventory()
	if len(f.Player.Inventory) == 0 {
		fmt.Println("Инвентарь пуст!")
		return f.playerTurn()
	}
	
	fmt.Print("Выберите номер предмета (0 для отмены): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	
	choice, err := strconv.Atoi(input)
	if err != nil {
		fmt.Println("Неверный ввод!")
		return f.playerTurn()
	}
	
	if choice == 0 {
		return f.playerTurn()
	}
	
	effect, used := f.Player.UseItem(choice - 1)
	if !used {
		return f.playerTurn()
	}
	
	if effect.StunRounds > 0 {
		f.Boss.ApplyStun(effect.StunRounds)
		return combat.Stun
	}
	
	if effect.SpecialEffect == "instant_peace" {
		if rand.Float32() < 0.3 {
			fmt.Println("\n✨ Печать старого договора сработала! Бой закончен миром! ✨")
			f.Boss.HP = 0
			return combat.Negotiate
		}
		fmt.Println("\n❌ Печать не сработала...")
	}
	
	// После использования предмета можно атаковать
	return f.chooseHit()
}

func (f *Fight) applyRound(playerAction, bossAction combat.BodyPart, bossDamage int) {
	// Игрок атакует (если не использовал специальное действие)
	if playerAction != combat.Stun && playerAction != combat.Negotiate {
		// Босс пытается блокировать
		bossBlock := f.Boss.ChooseBlock()
		
		// Расчет урона игрока
		playerDamage := f.calculateDamage(f.Player.GetStrength(), playerAction, bossBlock)
		
		// Применяем урон боссу
		if playerDamage > 0 {
			f.Boss.TakeDamage(playerDamage)
		}
	}
	
	// Босс атакует (если не оглушен)
	if bossAction != combat.Stun && bossDamage > 0 {
		// Игрок выбирает блок
		playerBlock := f.chooseBlock()
		
		// Проверка блока
		if bossAction == playerBlock {
			fmt.Println("🛡 Вы успешно заблокировали атаку!")
			bossDamage = bossDamage / 2
		}
		
		// Применяем урон игроку
		f.Player.TakeDamage(bossDamage)
	}
}

func (f *Fight) calculateDamage(strength int, attack, block combat.BodyPart) int {
	baseDamage := strength + rand.Intn(15)
	
	// Модификаторы от части тела
	switch attack {
	case combat.Head:
		baseDamage = int(float64(baseDamage) * 1.3)
	case combat.Legs:
		baseDamage = int(float64(baseDamage) * 0.8)
	}
	
	// Проверка блока
	if attack == block {
		fmt.Println("🛡 Противник заблокировал атаку!")
		baseDamage = baseDamage / 2
	}
	
	return baseDamage
}

func (f *Fight) showStatus() {
	fmt.Println("\n📊 СТАТУС БОЯ:")
	fmt.Printf("❤️ Ваше здоровье: %d/%d\n", f.Player.HP, f.Player.GetMaxHP())
	fmt.Printf("❤️ Здоровье %s: %d/%d\n", f.Boss.GetName(), f.Boss.HP, f.Boss.MaxHP)
}