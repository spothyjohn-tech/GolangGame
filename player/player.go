package player

import (
	"fmt"
	"game/items"
)

type Player struct {
	Name          string
	HP            int
	MaxHP         int
	BaseStrength  int
	Imagination   int
	Inventory     []*items.Item
	Equipped      []*items.Item
	ActiveEffects map[string]int
	Wins          int
}

func NewPlayer(name string) *Player {
	return &Player{
		Name:          name,
		HP:            120,
		MaxHP:         120,
		BaseStrength:  20,
		Imagination:   150,
		Inventory:     make([]*items.Item, 0),
		Equipped:      make([]*items.Item, 0),
		ActiveEffects: make(map[string]int),
		Wins:          0,
	}
}

func (p *Player) GetStrength() int {
	total := p.BaseStrength
	for _, item := range p.Equipped {
		total += item.Effect.Strength
	}
	if bonus, ok := p.ActiveEffects["strength_bonus"]; ok {
		total += bonus
	}
	return total
}

func (p *Player) GetMaxHP() int {
	total := p.MaxHP
	for _, item := range p.Equipped {
		total += item.Effect.MaxHP
	}
	return total
}

func (p *Player) AddItem(item *items.Item) {
	p.Inventory = append(p.Inventory, item)
	color := item.GetRarityColor()
	fmt.Printf("%s🎒 Получен предмет: %s - %s\033[0m\n", color, item.Name, item.Description)
}

func (p *Player) EquipItem(index int) bool {
	if index < 0 || index >= len(p.Inventory) {
		fmt.Println("Неверный номер предмета")
		return false
	}

	item := p.Inventory[index]

	// Проверяем, можно ли экипировать (не расходный)
	if item.Effect.Heal > 0 || item.Effect.StunRounds > 0 || item.Effect.SpecialEffect != "" {
		if item.Effect.Heal > 0 {
			fmt.Println("❌ Лечебные предметы нельзя экипировать, их нужно использовать в бою")
		} else if item.Effect.StunRounds > 0 {
			fmt.Println("❌ Предметы оглушения нельзя экипировать, их нужно использовать в бою")
		} else {
			fmt.Println("❌ Этот предмет нельзя экипировать")
		}
		return false
	}

	// Удаляем из инвентаря и добавляем в экипировку
	p.Inventory = append(p.Inventory[:index], p.Inventory[index+1:]...)
	p.Equipped = append(p.Equipped, item)

	// Применяем эффекты
	if item.Effect.MaxHP > 0 {
		oldMax := p.MaxHP
		p.MaxHP += item.Effect.MaxHP
		p.HP += item.Effect.MaxHP
		fmt.Printf("❤️ Максимальное здоровье увеличено: %d -> %d\n", oldMax, p.MaxHP)
	}

	fmt.Printf("✅ Экипировано: %s\n", item.Name)
	return true
}

func (p *Player) UnequipItem(index int) bool {
	if index < 0 || index >= len(p.Equipped) {
		fmt.Println("Неверный номер предмета")
		return false
	}

	item := p.Equipped[index]
	p.Equipped = append(p.Equipped[:index], p.Equipped[index+1:]...)
	p.Inventory = append(p.Inventory, item)

	if item.Effect.MaxHP > 0 {
		p.MaxHP -= item.Effect.MaxHP
		if p.HP > p.MaxHP {
			p.HP = p.MaxHP
		}
		fmt.Printf("❤️ Максимальное здоровье уменьшено: %d\n", p.MaxHP)
	}

	fmt.Printf("✅ Снято: %s\n", item.Name)
	return true
}

func (p *Player) UseItem(index int) (*items.ItemEffect, bool) {
	if index < 0 || index >= len(p.Inventory) {
		fmt.Println("Неверный номер предмета")
		return nil, false
	}

	item := p.Inventory[index]
	color := item.GetRarityColor()
	if item.Effect.MaxHP > 0 || item.Effect.Strength > 0 {
		fmt.Println("Данную вещь можно только экипировать")
		return nil, false
	} else {
		fmt.Printf("\n%s✨ Используется: %s\033[0m\n", color, item.Name)

		// Удаляем использованный предмет
		p.Inventory = append(p.Inventory[:index], p.Inventory[index+1:]...)
		fmt.Println("Предмет использован")

		return &item.Effect, true
	}
}

func (p *Player) Heal(amount int) {
	oldHP := p.HP
	p.HP += amount
	maxHP := p.GetMaxHP()
	if p.HP > maxHP {
		p.HP = maxHP
	}
	healed := p.HP - oldHP
	if healed > 0 {
		fmt.Printf("❤️ Восстановлено %d здоровья! (%d/%d)\n", healed, p.HP, maxHP)
	}
}

func (p *Player) TakeDamage(damage int) {
	p.HP -= damage
	if p.HP < 0 {
		p.HP = 0
	}
	fmt.Printf("💔 Получено %d урона! Осталось: %d/%d\n", damage, p.HP, p.GetMaxHP())
}

func (p *Player) AddImagination(amount int) {
	p.Imagination += amount
	fmt.Printf("✨ Получено %d воображения! Теперь: %d\n", amount, p.Imagination)
}

func (p *Player) SpendImagination(amount int) bool {
	if p.Imagination >= amount {
		p.Imagination -= amount
		return true
	}
	fmt.Println("❌ Недостаточно воображения!")
	return false
}

func (p *Player) IsAlive() bool {
	return p.HP > 0
}

func (p *Player) ResetForBattle() {
	// Сбрасываем временные эффекты
	p.ActiveEffects = make(map[string]int)
	// Восстанавливаем здоровье до максимума
	p.HP = p.GetMaxHP()
}

func (p *Player) ShowInventory() {
	fmt.Println("\n=== 🎒 ИНВЕНТАРЬ ===")
	if len(p.Inventory) == 0 {
		fmt.Println("Инвентарь пуст")
	} else {
		for i, item := range p.Inventory {
			color := item.GetRarityColor()
			fmt.Printf("%s%d. %s - %s\033[0m\n", color, i+1, item.Name, item.Description)
		}
	}

	if len(p.Equipped) > 0 {
		fmt.Println("\n=== ⚔️ ЭКИПИРОВАНО ===")
		for i, item := range p.Equipped {
			color := item.GetRarityColor()
			fmt.Printf("%s%d. %s\033[0m\n", color, i+1, item.Name)
		}
	}

	p.ShowStats()
}

func (p *Player) ShowStats() {
	fmt.Printf("\n❤️ Здоровье: %d/%d\n", p.HP, p.GetMaxHP())
	fmt.Printf("⚔️ Сила: %d (базовая: %d", p.GetStrength(), p.BaseStrength)
	if len(p.Equipped) > 0 {
		fmt.Printf(" + предметы")
	}
	if bonus, ok := p.ActiveEffects["strength_bonus"]; ok {
		fmt.Printf(" +%d бонус", bonus)
	}
	fmt.Printf(")\n")
	fmt.Printf("✨ Воображение: %d\n", p.Imagination)
	fmt.Printf("🏆 Побед: %d\n", p.Wins)
}
