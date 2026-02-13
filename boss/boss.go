package boss

import (
	"fmt"
	"game/combat"
	"math/rand"
)

type Boss struct {
	Name         string
	HP           int
	MaxHP        int
	Strength     int
	Description  string
	Phase        int
	Stunned      bool
	StunRounds   int
	SpecialMoves []SpecialMove
}

type SpecialMove struct {
	Name        string
	Damage      int
	Description string
}

func NewGuildBoss(name string, hp, strength int) *Boss {
	return &Boss{
		Name:        name,
		HP:          hp,
		MaxHP:       hp,
		Strength:    strength,
		Description: "",
		Phase:       1,
		Stunned:     false,
		StunRounds:  0,
		SpecialMoves: []SpecialMove{
			{
				Name:        "💥 Сокрушающий удар",
				Damage:      strength + 15,
				Description: "Мощная атака, пробивающая защиту",
			},
		},
	}
}

func NewFinalBoss() *Boss {
	return &Boss{
		Name:        "👾 ДРЕВНИЙ ХАОС",
		HP:          400,
		MaxHP:       400,
		Strength:    35,
		Description: "Первобытная сила, стоящая за всеми конфликтами Воображариума",
		Phase:       1,
		Stunned:     false,
		StunRounds:  0,
		SpecialMoves: []SpecialMove{
			{
				Name:        "🌪 Вихрь реальности",
				Damage:      45,
				Description: "Искажает пространство вокруг",
			},
			{
				Name:        "📖 Стирание истории",
				Damage:      60,
				Description: "Пытается стереть ваше прошлое",
			},
			{
				Name:        "🌀 Пустота",
				Damage:      80,
				Description: "Поглощает всё воображение",
			},
		},
	}
}

func (b *Boss) TakeDamage(damage int) {
	b.HP -= damage
	if b.HP < 0 {
		b.HP = 0
	}

	// Проверяем смену фазы (только для финального босса)
	if b.Name == "👾 ДРЕВНИЙ ХАОС" {
		if b.HP <= b.MaxHP*2/3 && b.Phase == 1 {
			b.Phase = 2
			b.Strength += 10
			fmt.Printf("\n⚠️ ФАЗА 2: %s становится сильнее! ⚠️\n", b.Name)
		} else if b.HP <= b.MaxHP/3 && b.Phase == 2 {
			b.Phase = 3
			b.Strength += 15
			fmt.Printf("\n⚠️ ФАЗА 3: %s в ярости! ⚠️\n", b.Name)
		}
	}

	fmt.Printf("💥 Нанесено %d урона %s! Осталось: %d/%d\n", damage, b.Name, b.HP, b.MaxHP)
}

func (b *Boss) ChooseAttack() (combat.BodyPart, int) {
	if b.Stunned {
		b.StunRounds--
		if b.StunRounds <= 0 {
			b.Stunned = false
			fmt.Printf("🌀 %s выходит из оглушения!\n", b.Name)
		}
		return combat.Stun, 0
	}

	// Шанс на особую атаку (30%)
	if rand.Float32() < 0.3 && len(b.SpecialMoves) > 0 {
		moveIndex := rand.Intn(len(b.SpecialMoves))
		move := b.SpecialMoves[moveIndex]
		fmt.Printf("\n⚠️ %s использует: %s!\n", b.Name, move.Name)
		fmt.Printf("   %s\n", move.Description)
		return combat.Torso, move.Damage
	}

	// Обычная атака
	damage := b.Strength + rand.Intn(15)
	part := combat.BodyPart(rand.Intn(3))
	return part, damage
}

func (b *Boss) ChooseBlock() combat.BodyPart {
	if b.Stunned {
		return combat.Torso
	}
	return combat.BodyPart(rand.Intn(3))
}

func (b *Boss) ApplyStun(rounds int) {
	b.Stunned = true
	b.StunRounds = rounds
	fmt.Printf("🌀 %s оглушен на %d хода!\n", b.Name, rounds)
}

func (b *Boss) IsStunned() bool {
	return b.Stunned
}

func (b *Boss) IsAlive() bool {
	return b.HP > 0
}

func (b *Boss) GetName() string {
	return b.Name
}

func (b *Boss) GetHP() int {
	return b.HP
}

func (b *Boss) GetStrength() int {
	return b.Strength
}
