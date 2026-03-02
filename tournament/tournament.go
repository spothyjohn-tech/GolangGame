package tournament

import (
	"fmt"
	"game/boss"
	"game/fight"
	"game/player"
	"game/story"
)

type Guild struct {
	Name     string
	Boss     *boss.Boss
	Defeated bool
}

type Tournament struct {
	Player       *player.Player
	Guilds       []Guild
	CurrentGuild int
	FinalBoss    *boss.Boss
	FinalDefeated bool
}

func NewTournament(p *player.Player) *Tournament {
	return &Tournament{
		Player: p,
		Guilds: []Guild{
			{
				Name:     "⚔️ Стальные Легенды",
				Boss:     boss.NewGuildBoss("Командир Стальных Легенд", 150, 25),
				Defeated: false,
			},
			{
				Name:     "🌑 Теневые Мечтатели",
				Boss:     boss.NewGuildBoss("Верховный Теневой", 180, 28),
				Defeated: false,
			},
			{
				Name:     "✨ Искры Творчества",
				Boss:     boss.NewGuildBoss("Магистр Творчества", 220, 32),
				Defeated: false,
			},
		},
		CurrentGuild: 0,
		FinalBoss:    boss.NewFinalBoss(),
		FinalDefeated: false,
	}
}

func (t *Tournament) IsComplete() bool {
	// Турнир завершен, если все гильдии побеждены
	for _, guild := range t.Guilds {
		if !guild.Defeated {
			return false
		}
	}
	return true
}

func (t *Tournament) IsFinalDefeated() bool {
	return t.FinalDefeated
}

func (t *Tournament) StartNextGuild() bool {
	if t.CurrentGuild >= len(t.Guilds) {
		fmt.Println("Все гильдии уже побеждены!")
		return false
	}
	
	guild := &t.Guilds[t.CurrentGuild]
	
	fmt.Printf("\n%s\n", "========================================")
	fmt.Printf("ГИЛЬДИЯ: %s\n", guild.Name)
	fmt.Println("========================================")
	
	// Бой с гильдией
	fight := fight.NewFight(t.Player, guild.Boss)
	victory := fight.Start()
	
	if victory {
		guild.Defeated = true
		t.Player.Wins++
		
		// Награда
		reward := 50 + t.CurrentGuild*25
		t.Player.AddImagination(reward)
		fmt.Printf("\n✨ Награда: %d воображения!\n", reward)
		
		t.CurrentGuild++
		
		// Проверяем, все ли гильдии побеждены
		if t.IsComplete() {
			fmt.Println("\n🎉 ВЫ ПОБЕДИЛИ ВСЕ ГИЛЬДИИ! 🎉")
			fmt.Println("Теперь вас ждет финальный бой с Древним Хаосом!")
		}
		
		return true
	}
	
	return false
}

func (t *Tournament) StartFinalBoss() bool {
	fmt.Printf("\n%s\n", "========================================")
	fmt.Printf("ФИНАЛЬНЫЙ БОЙ: %s\n", t.FinalBoss.Name)
	fmt.Println("========================================")
	
	story.BossIntro()
	
	fight := fight.NewFight(t.Player, t.FinalBoss)
	victory := fight.Start()
	
	if victory {
		t.FinalDefeated = true
		t.Player.Wins++
		
		// Финальная награда
		t.Player.AddImagination(200)
		story.Victory(t.Player.Imagination)
		return true
	}
	
	return false
}

func (t *Tournament) ShowProgress() {
	fmt.Println("\n=== ПРОГРЕСС ТУРНИРА ===")
	for i, guild := range t.Guilds {
		status := "❌ Не побеждена"
		if guild.Defeated {
			status = "✅ Побеждена"
		}
		fmt.Printf("%d. %s - %s\n", i+1, guild.Name, status)
	}
	
	if t.IsComplete() && !t.FinalDefeated {
		fmt.Println("\n👾 ФИНАЛЬНЫЙ БОСС: Древний Хаос - ⚔️ ДОСТУПЕН")
	} else if t.FinalDefeated {
		fmt.Println("\n👾 ФИНАЛЬНЫЙ БОСС: Древний Хаос - ✅ ПОБЕЖДЕН")
	}
	fmt.Println("==========================")
}

func IntroPlay() {
	story.Introduction()
}